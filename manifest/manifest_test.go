// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"errors"
	"io/fs"
	"maps"
	"slices"
	"strings"
	"testing"
)

// tree is a Tree held in memory.
type tree map[string]string

func (t tree) Files() []string { return slices.Sorted(maps.Keys(t)) }

func (t tree) ReadFile(name string) ([]byte, error) {
	s, ok := t[name]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return []byte(s), nil
}

// site is a tree any static manifest resolves against.
var site = tree{"index.html": "<html></html>"}

// refused asserts err is an *Error with code and, when given, reason.
func refused(t *testing.T, err error, code, reason string) *Error {
	t.Helper()
	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("want %s, got %v", code, err)
	}
	if e.Code != code {
		t.Fatalf("want %s, got %v", code, e)
	}
	if reason != "" && e.Details["reason"] != reason {
		t.Fatalf("want reason %s, got %v", reason, e)
	}
	return e
}

func resolve(src string, tr Tree) (*Resolved, error) {
	m, err := Parse([]byte(src))
	if err != nil {
		return nil, err
	}
	return Resolve(m, tr, Options{Slug: "my-app", Limits: DefaultLimits()})
}

// TestManifestValidation: an unknown key, a port of 0 or 70000, memory of
// 64Mi or 4Gi at the default limits, replicas {1, 2}, and a dir of ../x
// are each refused with the documented code and reason, and so is every
// other rule of the grammar.
func TestManifestValidation(t *testing.T) {
	svc := tree{"go.mod": "module x"}
	for _, tc := range []struct {
		name, src    string
		tr           Tree
		code, reason string
		key          string
	}{
		{"unknown key", "kind: static\nstatic:\n  dri: dist\n", site, CodeUnknownKey, "", "static.dri"},
		{"unknown top key", "nmae: x\n", site, CodeUnknownKey, "", "nmae"},
		{"port 0", "kind: service\nservice:\n  port: 0\n", svc, CodeInvalid, ReasonPortRange, "service.port"},
		{"port 70000", "kind: service\nservice:\n  port: 70000\n", svc, CodeInvalid, ReasonPortRange, "service.port"},
		{"memory 64Mi", "kind: service\nservice:\n  resources: {memory: 64Mi}\n", svc, CodeInvalid, ReasonMemoryRange, "service.resources.memory"},
		{"memory 4Gi", "kind: service\nservice:\n  resources: {memory: 4Gi}\n", svc, CodeInvalid, ReasonMemoryRange, "service.resources.memory"},
		{"memory word", "kind: service\nservice:\n  resources: {memory: lots}\n", svc, CodeInvalid, ReasonMemoryRange, ""},
		{"cpu 50m", "kind: service\nservice:\n  resources: {cpu: 50m}\n", svc, CodeInvalid, ReasonCPURange, ""},
		{"cpu 2 cores", "kind: service\nservice:\n  resources: {cpu: \"2\"}\n", svc, CodeInvalid, ReasonCPURange, ""},
		{"replicas", "kind: service\nservice:\n  replicas: {min: 1, max: 2}\n", svc, CodeReplicasUnsupported, "", ""},
		{"dir escapes", "kind: static\nstatic:\n  dir: ../x\n", site, CodeInvalid, ReasonDirEscapes, "static.dir"},
		{"dir absolute", "kind: static\nstatic:\n  dir: /etc\n", site, CodeInvalid, ReasonDirEscapes, ""},
		{"name mismatch", "name: other\n", site, CodeInvalid, ReasonNameMismatch, "name"},
		{"reserved header", "static:\n  headers:\n    - path: /a\n      Set-Cookie: x\n", site, CodeInvalid, ReasonHeaderReserved, ""},
		{"latere header", "static:\n  headers:\n    - path: /a\n      X-Latere-Thing: x\n", site, CodeInvalid, ReasonHeaderReserved, ""},
		{"header without path", "static:\n  headers:\n    - Cache-Control: x\n", site, CodeInvalid, ReasonValue, ""},
		{"too many headers", "static:\n  headers:\n" + strings.Repeat("    - path: /a\n      Cache-Control: x\n", 21), site, CodeInvalid, ReasonValue, ""},
		{"builder", "kind: service\nservice:\n  builder: alpine\n", svc, CodeInvalid, ReasonBuilderUnknown, ""},
		{"health relative", "kind: service\nservice:\n  health: healthz\n", svc, CodeInvalid, ReasonValue, ""},
		{"public paths", "kind: service\nservice:\n  public_paths: [" + strings.Repeat("/a, ", 10) + "/b]\n", svc, CodeInvalid, ReasonPublicPathsTooMany, ""},
		{"public path glob", "kind: service\nservice:\n  public_paths: [/a*b]\n", svc, CodeInvalid, ReasonValue, ""},
		{"migrate length", "kind: service\nservice:\n  migrate: " + strings.Repeat("x", 1001) + "\n", svc, CodeInvalid, ReasonMigrateLength, ""},
		{"migrate on static", "kind: static\nservice:\n  migrate: ./migrate\n", site, CodeInvalid, ReasonKindKeys, ""},
		{"static block on service", "kind: service\nstatic:\n  dir: x\n", svc, CodeInvalid, ReasonKindKeys, ""},
		{"dockerfile and builder", "kind: service\nservice:\n  builder: jammy-base\n  dockerfile: {path: Dockerfile}\n", svc, CodeInvalid, ReasonBuilderAndDockerfile, ""},
		{"dockerfile missing", "kind: service\nservice:\n  dockerfile: {path: Dockerfile}\n", svc, CodeInvalid, ReasonDockerfileMissing, ""},
		{"dockerfile unsupported", "kind: service\nservice:\n  dockerfile: {}\n", tree{"Dockerfile": "FROM x"}, CodeDockerfile, "", ""},
		{"kind", "kind: lambda\n", site, CodeInvalid, ReasonValue, ""},
		{"resource storage", "resources:\n  files: {type: storage}\n", site, CodeInvalid, ReasonResourceType, ""},
		{"resource size", "resources:\n  db: {type: postgres, size: 100Gi}\n", site, CodeInvalid, ReasonResourceSize, ""},
		{"cache with a size", "resources:\n  c: {type: cache, size: 1Gi}\n", site, CodeInvalid, ReasonResourceType, ""},
		{"resource preview", "resources:\n  db: {type: postgres, preview: half}\n", site, CodeInvalid, ReasonValue, ""},
		{"resource name", "resources:\n  Db: {type: postgres}\n", site, CodeInvalid, ReasonValue, ""},
		{"resource quota", "resources:\n  a: {type: cache}\n  b: {type: cache}\n  c: {type: cache}\n  d: {type: cache}\n", site, CodeResourceQuota, "", ""},
		{"links quota", "links: [" + strings.Repeat("a, ", 10) + "b]\n", site, CodeLinkQuota, "", ""},
		{"link to itself", "links: [my-app]\n", site, CodeLinkSelf, "", ""},
		{"syntax", "kind: [\n", site, CodeInvalid, ReasonSyntax, ""},
		{"type", "kind: service\nservice:\n  port: eighty\n", svc, CodeInvalid, ReasonType, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := resolve(tc.src, tc.tr)
			e := refused(t, err, tc.code, tc.reason)
			if tc.key != "" && e.Details["key"] != tc.key {
				t.Errorf("key = %v, want %s", e.Details["key"], tc.key)
			}
		})
	}
}

// TestUnknownKeyNamesItsLine: the refusal of an unknown key names the line
// it is on, which is what the CLI points at.
func TestUnknownKeyNamesItsLine(t *testing.T) {
	_, err := Parse([]byte("kind: service\nservice:\n  port: 8080\n  portt: 8081\n"))
	e := refused(t, err, CodeUnknownKey, "")
	if e.Details["line"] != 4 || e.Details["key"] != "service.portt" {
		t.Errorf("details = %v", e.Details)
	}
	if !strings.Contains(e.Error(), "manifest_unknown_key") || !strings.Contains(e.Error(), "line=4") {
		t.Errorf("Error() = %q", e.Error())
	}
}

// TestDefaults: an empty manifest over a plain site resolves to one static
// component at / with the detected build, and a bare service gets every
// default of the reference.
func TestDefaults(t *testing.T) {
	r, err := resolve("", site)
	if err != nil {
		t.Fatal(err)
	}
	c := r.Components[0]
	if len(r.Components) != 1 || c.Name != "app" || c.Kind != KindStatic || c.Route != "/" || c.Path != "." ||
		c.Static.Dir != "." || c.Static.Build == nil || *c.Static.Build != "" || c.Detected.Rule != 5 {
		t.Fatalf("empty manifest resolved to %+v %+v", c, c.Static)
	}
	r, err = resolve("kind: service\n", tree{"go.mod": "module x"})
	if err != nil {
		t.Fatal(err)
	}
	s := r.Components[0].Service
	if s.Builder != "jammy-base" || *s.Port != 8080 || s.Health != "/" || s.Resources.Memory != "256Mi" ||
		s.Resources.CPU != "250m" || s.Replicas.Min != 1 || s.Replicas.Max != 1 {
		t.Fatalf("service defaults = %+v", s)
	}
	if _, err := Resolve(nil, site, Options{Limits: DefaultLimits()}); err != nil {
		t.Fatalf("a nil manifest: %v", err)
	}
	r, err = resolve("kind: static\nstatic:\n  build: npm run build\n  dir: dist\n  spa: false\n  sitemap: true\n", site)
	if err != nil {
		t.Fatal(err)
	}
	st := r.Components[0].Static
	if *st.Build != "npm run build" || st.Dir != "dist" || *st.SPA || !st.Sitemap {
		t.Fatalf("static = %+v", st)
	}
}

func TestQuantities(t *testing.T) {
	for q, want := range map[string]float64{"128Mi": 128 << 20, "1Gi": 1 << 30, "512Ki": 512 << 10, "1k": 1e3,
		"2M": 2e6, "1G": 1e9, "1T": 1e12, "1Ti": 1 << 40, "1000": 1000, "0.5Gi": 1 << 29} {
		if got, ok := memoryBytes(q); !ok || got != want {
			t.Errorf("memoryBytes(%q) = %v, %v", q, got, ok)
		}
	}
	for q, want := range map[string]float64{"250m": 250, "1": 1000, "0.5": 500} {
		if got, ok := cpuMillis(q); !ok || got != want {
			t.Errorf("cpuMillis(%q) = %v, %v", q, got, ok)
		}
	}
	for _, q := range []string{"", "Mi", "-1Gi", "1e3", "lots"} {
		if _, ok := memoryBytes(q); ok {
			t.Errorf("memoryBytes(%q) read", q)
		}
	}
}
