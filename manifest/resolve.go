// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"errors"
	"fmt"
	"maps"
	"path"
	"regexp"
	"slices"
	"strings"
)

// Limits are the figures a manifest is checked against, which the
// platform's answer to deploy.create carries; DefaultLimits are a
// self-hosted installation's.
type Limits struct {
	ComponentsMax   int
	LinksMax        int
	ResourcesMax    int
	MemoryMax       string
	CPUMax          string
	PostgresSizeMax string
}

// DefaultLimits are the figures a manifest is checked against when the
// platform names none.
func DefaultLimits() Limits {
	return Limits{ComponentsMax: 3, LinksMax: 10, ResourcesMax: 3, MemoryMax: "1Gi", CPUMax: "1000m", PostgresSizeMax: "10Gi"}
}

// Options steer Resolve.
type Options struct {
	// Slug is the app's slug, which name must equal when set.
	Slug string
	// Limits are the figures the manifest is checked against.
	Limits Limits
}

// Resolved is a manifest applied to one tree: every component with its
// kind decided and its defaults filled, and the app-level keys.
type Resolved struct {
	Components []ResolvedComponent  `json:"components"`
	Links      []string             `json:"links,omitempty"`
	BuildEnv   map[string]string    `json:"build_env,omitempty"`
	Env        map[string]string    `json:"env,omitempty"`
	Resources  map[string]*Resource `json:"resources,omitempty"`
}

// ResolvedComponent is one component as it is built and served.
type ResolvedComponent struct {
	Name        string            `json:"name"`
	Kind        string            `json:"kind"`
	Path        string            `json:"path"`
	Route       string            `json:"route"`
	StripPrefix bool              `json:"strip_prefix,omitempty"`
	Static      *Static           `json:"static,omitempty"`
	Service     *Service          `json:"service,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	Detected    *Detection        `json:"detected,omitempty"`
}

var (
	namePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,19}$`)
	reference   = regexp.MustCompile(`\$\{\{\s*([^}]*?)\s*\}\}`)
)

// reservedHeaders are the response headers a static header rule may not
// set.
var reservedHeaders = []string{"content-length", "content-encoding", "set-cookie"}

const (
	maxHeaderRules = 20
	maxPublicPaths = 10
	maxMigrate     = 1000
	minMemory      = "128Mi"
	minCPU         = "100m"
	minPostgres    = "1Gi"
	shorthandName  = "app"
)

// Resolve applies m to the tree t: it decides each component's kind,
// detecting it when the manifest names none, fills the defaults, and
// checks every rule of the grammar. The first rule broken is the error,
// an *Error with the code of package codes. A nil m is the empty
// manifest.
func Resolve(m *Manifest, t Tree, opts Options) (*Resolved, error) {
	if m == nil {
		m = &Manifest{}
	}
	if m.Name != "" && opts.Slug != "" && m.Name != opts.Slug {
		return nil, m.invalid("name", ReasonNameMismatch)
	}
	comps, err := m.components()
	if err != nil {
		return nil, err
	}
	if len(comps) > opts.Limits.ComponentsMax {
		return nil, fail(CodeComponentQuota, "limit", "components_max", "value", len(comps), "max", opts.Limits.ComponentsMax)
	}
	out := &Resolved{Links: m.Links, Env: m.Env, Resources: m.Resources}
	if m.Build != nil {
		out.BuildEnv = m.Build.Env
	}
	files := t.Files()
	for _, c := range comps {
		rc, err := m.resolveComponent(c, t, files, opts.Limits)
		if err != nil {
			return nil, err
		}
		out.Components = append(out.Components, *rc)
	}
	if err := checkRoutes(out.Components); err != nil {
		return nil, err
	}
	if err := m.checkLinks(opts); err != nil {
		return nil, err
	}
	if err := m.checkResources(opts.Limits); err != nil {
		return nil, err
	}
	if err := m.checkReferences(out); err != nil {
		return nil, err
	}
	return out, nil
}

// named is a component with its name and the key prefix its errors name.
type named struct {
	name string
	key  string
	c    *Component
}

// components is the components form, or the shorthand as one component
// named app at "/", in name order.
func (m *Manifest) components() ([]named, error) {
	if len(m.Components) == 0 {
		return []named{{name: shorthandName, c: &Component{Kind: m.Kind, Route: "/", Static: m.Static, Service: m.Service}}}, nil
	}
	var out []named
	for _, name := range slices.Sorted(maps.Keys(m.Components)) {
		key := "components." + name
		if !namePattern.MatchString(name) || strings.Contains(name, "--") {
			return nil, m.invalid(key, ReasonValue)
		}
		c := m.Components[name]
		if c == nil {
			c = &Component{}
		}
		out = append(out, named{name: name, key: key + ".", c: c})
	}
	return out, nil
}

func (m *Manifest) resolveComponent(n named, t Tree, files []string, l Limits) (*ResolvedComponent, error) {
	c := n.c
	rc := &ResolvedComponent{Name: n.name, Kind: c.Kind, Route: c.Route, StripPrefix: c.StripPrefix, Env: c.Env}
	p := c.Path
	if p == "" {
		p = "."
	}
	if path.IsAbs(p) || strings.HasPrefix(path.Clean(p), "..") {
		return nil, m.invalid(n.key+"path", ReasonValue)
	}
	rc.Path = path.Clean(p)
	switch c.Kind {
	case "", KindStatic, KindService:
	case KindWorker, KindCron:
		return nil, fail(CodeKindUnsupported, "component", n.name, "kind", c.Kind)
	default:
		return nil, m.invalid(n.key+"kind", ReasonValue)
	}
	if rc.Path != "." && !slices.ContainsFunc(files, func(f string) bool { return strings.HasPrefix(f, rc.Path+"/") }) {
		return nil, fail(CodePathMissing, "component", n.name, "path", rc.Path)
	}
	if rc.Kind == "" {
		d, err := Detect(t, rc.Path)
		if err != nil {
			var e *Error
			if errors.As(err, &e) {
				e.Details["component"] = n.name
			}
			return nil, err
		}
		if d.Rule == 1 {
			return nil, fail(CodeDockerfile, "component", n.name, "path", rc.Path)
		}
		rc.Kind, rc.Detected = d.Kind, d
	}
	switch rc.Kind {
	case KindStatic:
		if c.Service != nil {
			return nil, m.invalid(n.key+"service", ReasonKindKeys)
		}
		s, err := m.resolveStatic(n.key+"static.", c.Static, rc.Detected)
		if err != nil {
			return nil, err
		}
		rc.Static = s
	case KindService:
		if c.Static != nil {
			return nil, m.invalid(n.key+"static", ReasonKindKeys)
		}
		s, err := m.resolveService(n, c.Service, files, rc.Path, l)
		if err != nil {
			return nil, err
		}
		rc.Service = s
	}
	return rc, nil
}

func (m *Manifest) resolveStatic(key string, s *Static, d *Detection) (*Static, error) {
	out := &Static{}
	if s != nil {
		*out = *s
	}
	if out.Build == nil && d != nil {
		out.Build = d.Build
	}
	if out.Dir == "" && d != nil {
		out.Dir = d.Dir
	}
	if out.Dir == "" {
		out.Dir = "."
	}
	if path.IsAbs(out.Dir) || strings.HasPrefix(path.Clean(out.Dir), "..") {
		return nil, m.invalid(key+"dir", ReasonDirEscapes)
	}
	out.Dir = path.Clean(out.Dir)
	if len(out.Headers) > maxHeaderRules {
		return nil, m.invalid(key+"headers", ReasonValue)
	}
	for i, rule := range out.Headers {
		k := fmt.Sprintf("%sheaders[%d]", key, i)
		if !strings.HasPrefix(rule["path"], "/") || len(rule) < 2 {
			return nil, m.invalid(k, ReasonValue)
		}
		for h := range rule {
			lh := strings.ToLower(h)
			if h != "path" && (slices.Contains(reservedHeaders, lh) || strings.HasPrefix(lh, "x-latere-")) {
				return nil, m.invalid(k, ReasonHeaderReserved)
			}
		}
	}
	return out, nil
}

func (m *Manifest) resolveService(n named, s *Service, files []string, root string, l Limits) (*Service, error) {
	key := n.key + "service."
	out := &Service{}
	if s != nil {
		*out = *s
	}
	if out.Dockerfile != nil && out.Builder != "" {
		return nil, m.invalid(key+"builder", ReasonBuilderAndDockerfile)
	}
	if out.Dockerfile != nil {
		df := out.Dockerfile.Path
		if df == "" {
			df = "Dockerfile"
		}
		if !slices.Contains(files, path.Join(root, df)) {
			return nil, m.invalid(key+"dockerfile.path", ReasonDockerfileMissing)
		}
		return nil, fail(CodeDockerfile, "component", n.name, "path", path.Join(root, df))
	}
	switch out.Builder {
	case "":
		out.Builder = "jammy-base"
	case "jammy-base", "jammy-full":
	default:
		return nil, m.invalid(key+"builder", ReasonBuilderUnknown)
	}
	if out.Port == nil {
		port := 8080
		out.Port = &port
	} else if *out.Port < 1 || *out.Port > 65535 {
		return nil, m.invalid(key+"port", ReasonPortRange)
	}
	if out.Health == "" {
		out.Health = "/"
	} else if !strings.HasPrefix(out.Health, "/") {
		return nil, m.invalid(key+"health", ReasonValue)
	}
	r := Resources{Memory: "256Mi", CPU: "250m"}
	if out.Resources != nil {
		if out.Resources.Memory != "" {
			r.Memory = out.Resources.Memory
		}
		if out.Resources.CPU != "" {
			r.CPU = out.Resources.CPU
		}
	}
	if !within(memoryBytes, r.Memory, minMemory, l.MemoryMax) {
		return nil, m.invalid(key+"resources.memory", ReasonMemoryRange)
	}
	if !within(cpuMillis, r.CPU, minCPU, l.CPUMax) {
		return nil, m.invalid(key+"resources.cpu", ReasonCPURange)
	}
	out.Resources = &r
	if out.Replicas == nil {
		out.Replicas = &Replicas{Min: 1, Max: 1}
	} else if out.Replicas.Min != 1 || out.Replicas.Max != 1 {
		return nil, fail(CodeReplicasUnsupported, "component", n.name, "min", out.Replicas.Min, "max", out.Replicas.Max)
	}
	if len(out.PublicPaths) > maxPublicPaths {
		return nil, m.invalid(key+"public_paths", ReasonPublicPathsTooMany)
	}
	for _, p := range out.PublicPaths {
		if !strings.HasPrefix(p, "/") || strings.Contains(strings.TrimSuffix(p, "*"), "*") {
			return nil, m.invalid(key+"public_paths", ReasonValue)
		}
	}
	if len(out.Migrate) > maxMigrate {
		return nil, m.invalid(key+"migrate", ReasonMigrateLength)
	}
	return out, nil
}

// within reports whether q reads as a quantity between lo and hi.
func within(read func(string) (float64, bool), q, lo, hi string) bool {
	v, ok := read(q)
	floor, _ := read(lo)
	ceil, okCeil := read(hi)
	return ok && v >= floor && (!okCeil || v <= ceil)
}

func checkRoutes(comps []ResolvedComponent) error {
	owners := map[string][]string{}
	var roots []string
	for _, c := range comps {
		r := c.Route
		if r == "" || r != "/" && (path.Clean(r) != r || !strings.HasPrefix(r, "/")) {
			return fail(CodeInvalid, "key", "components."+c.Name+".route", "reason", ReasonValue)
		}
		owners[r] = append(owners[r], c.Name)
		if r == "/" {
			roots = append(roots, c.Name)
		}
	}
	for _, r := range slices.Sorted(maps.Keys(owners)) {
		if r != "/" && len(owners[r]) > 1 {
			return fail(CodeRouteConflict, "route", r, "components", owners[r])
		}
	}
	names := make([]string, len(comps))
	for i, c := range comps {
		names[i] = c.Name
	}
	switch {
	case len(roots) == 0:
		return fail(CodeRootMissing, "components", names)
	case len(roots) > 1:
		return fail(CodeRootConflict, "components", roots)
	}
	return nil
}

func (m *Manifest) checkLinks(opts Options) error {
	if len(m.Links) > opts.Limits.LinksMax {
		return fail(CodeLinkQuota, "limit", "links_max", "value", len(m.Links), "max", opts.Limits.LinksMax)
	}
	for _, s := range m.Links {
		if opts.Slug != "" && s == opts.Slug {
			return fail(CodeLinkSelf, "slug", s)
		}
	}
	return nil
}

func (m *Manifest) checkResources(l Limits) error {
	if len(m.Resources) > l.ResourcesMax {
		return fail(CodeResourceQuota, "limit", "resources_max", "value", len(m.Resources), "max", l.ResourcesMax)
	}
	for _, name := range slices.Sorted(maps.Keys(m.Resources)) {
		key := "resources." + name
		r := m.Resources[name]
		if !namePattern.MatchString(name) {
			return m.invalid(key, ReasonValue)
		}
		if r == nil || (r.Type != "postgres" && r.Type != "cache") {
			return m.invalid(key+".type", ReasonResourceType)
		}
		if r.Type != "postgres" && (r.Size != "" || r.Preview != "") {
			return m.invalid(key+".type", ReasonResourceType)
		}
		if r.Size != "" && !within(memoryBytes, r.Size, minPostgres, l.PostgresSizeMax) {
			return m.invalid(key+".size", ReasonResourceSize)
		}
		if r.Preview != "" && r.Preview != "copy" && r.Preview != "empty" {
			return m.invalid(key+".preview", ReasonValue)
		}
	}
	return nil
}

// fields are the values a reference to a resource may name, by type.
var fields = map[string][]string{
	"postgres": {"url", "host", "port", "name", "user", "password"},
	"cache":    {"url", "host", "port"},
}

// checkReferences checks every ${{ }} expression of an env value against
// what the platform provides (app, a service component, a link, a
// resource field), and refuses any in build.env.
func (m *Manifest) checkReferences(r *Resolved) error {
	services := map[string]bool{}
	for _, c := range r.Components {
		services[c.Name] = c.Kind == KindService
	}
	check := func(key string, env map[string]string, allowed bool) error {
		for _, k := range slices.Sorted(maps.Keys(env)) {
			for _, match := range reference.FindAllStringSubmatch(env[k], -1) {
				if !allowed || !m.known(match[1], services) {
					return fail(CodeUnknownReference, "key", key+"."+k, "expression", match[0])
				}
			}
		}
		return nil
	}
	if err := check("build.env", r.BuildEnv, false); err != nil {
		return err
	}
	if err := check("env", r.Env, true); err != nil {
		return err
	}
	for _, c := range r.Components {
		if err := check("components."+c.Name+".env", c.Env, true); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manifest) known(expr string, services map[string]bool) bool {
	parts := strings.Split(expr, ".")
	switch {
	case len(parts) == 2 && parts[0] == "app":
		return parts[1] == "url" || parts[1] == "slug"
	case len(parts) == 3 && parts[0] == "components" && parts[2] == "url":
		return services[parts[1]]
	case len(parts) == 3 && parts[0] == "links" && parts[2] == "url":
		return slices.Contains(m.Links, parts[1])
	case len(parts) == 3 && parts[0] == "resources":
		r, ok := m.Resources[parts[1]]
		return ok && r != nil && slices.Contains(fields[r.Type], parts[2])
	}
	return false
}

// Expand replaces each reference of s whose expression values names,
// such as "app.url", with its value; a reference values does not name is
// left as written.
func Expand(s string, values map[string]string) string {
	return reference.ReplaceAllStringFunc(s, func(match string) string {
		expr := reference.FindStringSubmatch(match)[1]
		if v, ok := values[expr]; ok {
			return v
		}
		return match
	})
}
