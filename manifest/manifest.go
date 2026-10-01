// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
)

// FileName is the manifest's name at the root of an app's tree.
const FileName = "latere-app.yaml"

// The component kinds. Worker and cron parse and are refused until they
// are built.
const (
	KindStatic  = "static"
	KindService = "service"
	KindWorker  = "worker"
	KindCron    = "cron"
)

// Manifest is a parsed latere-app.yaml. A field the file leaves out is
// its zero value or nil, and Resolve applies the defaults.
type Manifest struct {
	Name       string                `yaml:"name" json:"name,omitempty"`
	Kind       string                `yaml:"kind" json:"kind,omitempty"`
	Static     *Static               `yaml:"static" json:"static,omitempty"`
	Service    *Service              `yaml:"service" json:"service,omitempty"`
	Components map[string]*Component `yaml:"components" json:"components,omitempty"`
	Links      []string              `yaml:"links" json:"links,omitempty"`
	Build      *Build                `yaml:"build" json:"build,omitempty"`
	Env        map[string]string     `yaml:"env" json:"env,omitempty"`
	Resources  map[string]*Resource  `yaml:"resources" json:"resources,omitempty"`

	// lines is the line of each key the file sets, by its dotted path.
	lines map[string]int
}

// Static is the static block: how a static component is built and served.
type Static struct {
	// Build is the build command; nil means detected, "" no build.
	Build *string `yaml:"build" json:"build,omitempty"`
	// Dir is the directory published, relative to the component's root.
	Dir string `yaml:"dir" json:"dir,omitempty"`
	// SPA serves index.html for unknown paths; nil means true when the
	// output has an index.html.
	SPA *bool `yaml:"spa" json:"spa,omitempty"`
	// Headers are extra response headers by path glob, each rule a path
	// and the headers set on it.
	Headers []map[string]string `yaml:"headers" json:"headers,omitempty"`
	// Sitemap generates /sitemap.xml from the component's HTML paths.
	Sitemap bool `yaml:"sitemap" json:"sitemap,omitempty"`
}

// Service is the service block: how a service component is built and run.
type Service struct {
	Builder              string      `yaml:"builder" json:"builder,omitempty"`
	Start                string      `yaml:"start" json:"start,omitempty"`
	Port                 *int        `yaml:"port" json:"port,omitempty"`
	Health               string      `yaml:"health" json:"health,omitempty"`
	Resources            *Resources  `yaml:"resources" json:"resources,omitempty"`
	Replicas             *Replicas   `yaml:"replicas" json:"replicas,omitempty"`
	PublicPaths          []string    `yaml:"public_paths" json:"public_paths,omitempty"`
	ForwardAuthorization bool        `yaml:"forward_authorization" json:"forward_authorization,omitempty"`
	Dockerfile           *Dockerfile `yaml:"dockerfile" json:"dockerfile,omitempty"`
	Migrate              string      `yaml:"migrate" json:"migrate,omitempty"`
}

// Resources are a service component's memory and CPU, as Kubernetes
// quantities.
type Resources struct {
	Memory string `yaml:"memory" json:"memory,omitempty"`
	CPU    string `yaml:"cpu" json:"cpu,omitempty"`
}

// Replicas are a service component's replica bounds.
type Replicas struct {
	Min int `yaml:"min" json:"min"`
	Max int `yaml:"max" json:"max"`
}

// Dockerfile builds a service from a Dockerfile.
type Dockerfile struct {
	Path   string `yaml:"path" json:"path,omitempty"`
	Target string `yaml:"target" json:"target,omitempty"`
}

// Component is one component of the components form.
type Component struct {
	Kind        string            `yaml:"kind" json:"kind,omitempty"`
	Path        string            `yaml:"path" json:"path,omitempty"`
	Route       string            `yaml:"route" json:"route,omitempty"`
	StripPrefix bool              `yaml:"strip_prefix" json:"strip_prefix,omitempty"`
	Static      *Static           `yaml:"static" json:"static,omitempty"`
	Service     *Service          `yaml:"service" json:"service,omitempty"`
	Env         map[string]string `yaml:"env" json:"env,omitempty"`
	Schedule    string            `yaml:"schedule" json:"schedule,omitempty"`
	Start       string            `yaml:"start" json:"start,omitempty"`
}

// Build is the build block.
type Build struct {
	Env map[string]string `yaml:"env" json:"env,omitempty"`
}

// Resource is one data service.
type Resource struct {
	Type    string `yaml:"type" json:"type,omitempty"`
	Size    string `yaml:"size" json:"size,omitempty"`
	Preview string `yaml:"preview" json:"preview,omitempty"`
}

// grammar is the shape of one value: a mapping of named keys, a mapping
// of any names to one grammar, an open mapping of any keys to scalars, a
// list of one grammar, or a scalar.
type grammar struct {
	keys map[string]*grammar
	each *grammar
	open bool
	list *grammar
}

var (
	scalar  = &grammar{}
	openMap = &grammar{open: true}

	staticGrammar = &grammar{keys: map[string]*grammar{
		"build": scalar, "dir": scalar, "spa": scalar, "sitemap": scalar,
		"headers": {list: openMap},
	}}
	serviceGrammar = &grammar{keys: map[string]*grammar{
		"builder": scalar, "start": scalar, "port": scalar, "health": scalar,
		"resources":             {keys: map[string]*grammar{"memory": scalar, "cpu": scalar}},
		"replicas":              {keys: map[string]*grammar{"min": scalar, "max": scalar}},
		"public_paths":          {list: scalar},
		"forward_authorization": scalar,
		"dockerfile":            {keys: map[string]*grammar{"path": scalar, "target": scalar}},
		"migrate":               scalar,
	}}
	componentGrammar = &grammar{keys: map[string]*grammar{
		"kind": scalar, "path": scalar, "route": scalar, "strip_prefix": scalar,
		"static": staticGrammar, "service": serviceGrammar, "env": openMap,
		"schedule": scalar, "start": scalar,
	}}
	rootGrammar = &grammar{keys: map[string]*grammar{
		"name": scalar, "kind": scalar,
		"static": staticGrammar, "service": serviceGrammar,
		"components": {each: componentGrammar},
		"links":      {list: scalar},
		"build":      {keys: map[string]*grammar{"env": openMap}},
		"env":        openMap,
		"resources": {each: &grammar{keys: map[string]*grammar{
			"type": scalar, "size": scalar, "preview": scalar,
		}}},
	}}
)

// Parse reads a latere-app.yaml. An empty file is the empty manifest. A
// key the grammar does not have is CodeUnknownKey with its dotted path
// and line; components beside the single-component keys is
// CodeShapeConflict; a file that is not YAML or a value of the wrong type
// is CodeInvalid.
func Parse(data []byte) (*Manifest, error) {
	m := &Manifest{lines: map[string]int{}}
	file, err := parser.ParseBytes(data, 0)
	if err != nil {
		return nil, syntaxError(err)
	}
	for _, doc := range file.Docs {
		if doc == nil || doc.Body == nil {
			continue
		}
		if err := m.walk(doc.Body, rootGrammar, ""); err != nil {
			return nil, err
		}
	}
	if err := yaml.Unmarshal(data, m); err != nil {
		return nil, typeError(err)
	}
	if len(m.Components) > 0 {
		var mixed []string
		for _, k := range []string{"kind", "static", "service"} {
			if _, ok := m.lines[k]; ok {
				mixed = append(mixed, k)
			}
		}
		if len(mixed) > 0 {
			return nil, fail(CodeShapeConflict, "keys", append([]string{"components"}, mixed...))
		}
	}
	return m, nil
}

// walk checks node against g, recording the line of every key it meets.
func (m *Manifest) walk(node ast.Node, g *grammar, path string) error {
	switch {
	case node == nil, g == scalar, g.open:
		return nil
	case g.list != nil:
		seq, ok := node.(*ast.SequenceNode)
		if !ok {
			return nil
		}
		for i, v := range seq.Values {
			if err := m.walk(v, g.list, path+"["+strconv.Itoa(i)+"]"); err != nil {
				return err
			}
		}
		return nil
	}
	for _, mv := range mappingValues(node) {
		key := mv.Key.String()
		full := key
		if path != "" {
			full = path + "." + key
		}
		line := mv.Key.GetToken().Position.Line
		m.lines[full] = line
		next := g.each
		if next == nil {
			var ok bool
			if next, ok = g.keys[key]; !ok {
				return fail(CodeUnknownKey, "key", full, "line", line)
			}
		}
		if err := m.walk(mv.Value, next, full); err != nil {
			return err
		}
	}
	return nil
}

func mappingValues(node ast.Node) []*ast.MappingValueNode {
	switch n := node.(type) {
	case *ast.MappingNode:
		return n.Values
	case *ast.MappingValueNode:
		return []*ast.MappingValueNode{n}
	}
	return nil
}

// position is the "[line:column]" prefix the YAML package writes.
var position = regexp.MustCompile(`^\[(\d+):\d+\]\s*`)

// cleanError is the YAML package's first line, without its position
// prefix, and the line it names.
func cleanError(err error) (string, int) {
	msg, _, _ := strings.Cut(err.Error(), "\n")
	line := 0
	if p := position.FindStringSubmatch(msg); p != nil {
		line, _ = strconv.Atoi(p[1])
		msg = msg[len(p[0]):]
	}
	return msg, line
}

func syntaxError(err error) *Error {
	msg, line := cleanError(err)
	e := fail(CodeInvalid, "reason", ReasonSyntax, "error", msg)
	if line > 0 {
		e.Details["line"] = line
	}
	return e
}

func typeError(err error) *Error {
	msg, line := cleanError(err)
	e := fail(CodeInvalid, "reason", ReasonType, "error", msg)
	if line > 0 {
		e.Details["line"] = line
	}
	return e
}
