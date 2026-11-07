// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

var svcTree = tree{"go.mod": "module x"}

// TestQuantityReadsTheKubernetesGrammar: forms Kubernetes reads are read
// and the five it refuses, with Inf and NaN, are not.
func TestQuantityReadsTheKubernetesGrammar(t *testing.T) {
	for q, want := range map[string]float64{
		"1e3": 1000, "1E3": 1000, "1E": 1e18, "1Pi": 1 << 50, "1Ei": 1 << 60, "1P": 1e15, "+1Gi": 1 << 30,
		".5Gi": 1 << 29, "1.": 1, "1E+3": 1000, "1e-1": 0.1, "1n": 1e-9, "1u": 1e-6, "128974848": 128974848,
	} {
		if got, ok := memoryBytes(q); !ok || got != want {
			t.Errorf("memoryBytes(%q) = %v, %v; want %v", q, got, ok, want)
		}
	}
	for _, q := range []string{"0x1p30", "1_024Mi", "0x1p-2Gi", "0x1p-2", "Inf", "NaN", "+Inf", "-1Gi", "-1", "1e", "1E+", "1ei",
		"1 Gi", "1Gi ", "", ".", "1..5", "1Mi3", "1e400", "1ki", "1mi"} {
		if _, ok := memoryBytes(q); ok {
			t.Errorf("memoryBytes(%q) read", q)
		}
		if _, ok := cpuMillis(q); ok {
			t.Errorf("cpuMillis(%q) read", q)
		}
	}
	for q, want := range map[string]float64{"250m": 250, "1": 1000, "0.5": 500, "1e3": 1e6, "2k": 2e6} {
		if got, ok := cpuMillis(q); !ok || got != want {
			t.Errorf("cpuMillis(%q) = %v, %v; want %v", q, got, ok, want)
		}
	}
}

// TestResolveRefusesQuantitiesKubernetesRefuses: a hex float or a digit
// underscore in memory, cpu or a postgres size is refused.
func TestResolveRefusesQuantitiesKubernetesRefuses(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		tr        Tree
		reason    string
	}{
		{"memory hex float", "kind: service\nservice:\n  resources: {memory: 0x1p30}\n", svcTree, ReasonMemoryRange},
		{"memory underscore", "kind: service\nservice:\n  resources: {memory: 1_024Mi}\n", svcTree, ReasonMemoryRange},
		{"memory hex float with suffix", "kind: service\nservice:\n  resources: {memory: 0x1p-2Gi}\n", svcTree, ReasonMemoryRange},
		{"cpu hex float", "kind: service\nservice:\n  resources: {cpu: 0x1p-2}\n", svcTree, ReasonCPURange},
		{"postgres size hex float", "resources:\n  db: {type: postgres, size: 0x1p30}\n", site, ReasonResourceSize},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := resolve(tc.src, tc.tr)
			refused(t, err, CodeInvalid, tc.reason)
		})
	}
	for _, src := range []string{
		"kind: service\nservice:\n  resources: {memory: 2e8}\n",
		"kind: service\nservice:\n  resources: {memory: +512Mi}\n",
	} {
		if _, err := resolve(src, svcTree); err != nil {
			t.Errorf("%q: %v", src, err)
		}
	}
}

// TestResolveRefusesLimitsThatDoNotRead: an unreadable ceiling is a plain
// error naming the field, never an *Error and never no ceiling.
func TestResolveRefusesLimitsThatDoNotRead(t *testing.T) {
	for _, tc := range []struct {
		name  string
		mut   func(*Limits)
		field string
	}{
		{"empty memory", func(l *Limits) { l.MemoryMax = "" }, "MemoryMax"},
		{"word memory", func(l *Limits) { l.MemoryMax = "unlimited" }, "MemoryMax"},
		{"hex cpu", func(l *Limits) { l.CPUMax = "0x1p0" }, "CPUMax"},
		{"empty cpu", func(l *Limits) { l.CPUMax = "" }, "CPUMax"},
		{"negative postgres", func(l *Limits) { l.PostgresSizeMax = "-1Gi" }, "PostgresSizeMax"},
		{"empty postgres", func(l *Limits) { l.PostgresSizeMax = "" }, "PostgresSizeMax"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l := DefaultLimits()
			tc.mut(&l)
			_, err := Resolve(nil, site, Options{Limits: l})
			var e *Error
			if err == nil || errors.As(err, &e) || !strings.Contains(err.Error(), "Limits."+tc.field) {
				t.Fatalf("err = %v, want a plain error naming %s", err, tc.field)
			}
		})
	}
	// A ceiling Kubernetes reads but the old reader did not is a ceiling.
	l := DefaultLimits()
	l.MemoryMax = "1Pi"
	m, err := Parse([]byte("kind: service\nservice:\n  resources: {memory: 2Pi}\n"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = Resolve(m, svcTree, Options{Limits: l})
	refused(t, err, CodeInvalid, ReasonMemoryRange)
}

// TestResolveTreatsZeroLimitsAsDefaultLimits: zero Options resolve a
// manifest the way DefaultLimits do, and refuse what they refuse.
func TestResolveTreatsZeroLimitsAsDefaultLimits(t *testing.T) {
	for _, src := range []string{
		"kind: static\n",
		"kind: service\nservice:\n  resources: {memory: 512Mi}\n",
		"kind: service\nservice:\n  resources: {memory: 4Gi}\n",
		"resources:\n  db: {type: postgres, size: 20Gi}\n",
	} {
		m, err := Parse([]byte(src))
		if err != nil {
			t.Fatal(err)
		}
		got, gerr := Resolve(m, svcTree, Options{})
		want, werr := Resolve(m, svcTree, Options{Limits: DefaultLimits()})
		if !reflect.DeepEqual(got, want) || (gerr == nil) != (werr == nil) || (gerr != nil && gerr.Error() != werr.Error()) {
			t.Errorf("%q: zero options = %v, %v; DefaultLimits = %v, %v", src, got, gerr, want, werr)
		}
	}
	if _, err := Resolve(nil, site, Options{}); err != nil {
		t.Errorf("empty manifest with zero options: %v", err)
	}
}

// TestResolveChecksNamesAndLinks: the app name and each link are slugs,
// component and resource names share one rule, and a link is listed once.
func TestResolveChecksNamesAndLinks(t *testing.T) {
	long := strings.Repeat("a", 41)
	for _, tc := range []struct {
		name, src string
		tr        Tree
		key       string
	}{
		{"name with a space", "name: Bad Name\n", site, "name"},
		{"name with punctuation", "name: bad!\n", site, "name"},
		{"name too long", "name: " + long + "\n", site, "name"},
		{"name ends in a hyphen", "name: a-\n", site, "name"},
		{"name has a double hyphen", "name: a--b\n", site, "name"},
		{"link too long", "links: [" + long + "]\n", site, "links"},
		{"link with a path", "links: [../x]\n", site, "links"},
		{"link in capitals", "links: [UP]\n", site, "links"},
		{"link with a dot", "links: [a.b]\n", site, "links"},
		{"link ends in a hyphen", "links: [a-]\n", site, "links"},
		{"link has a double hyphen", "links: [a--b]\n", site, "links"},
		{"link listed twice", "links: [other, other]\n", site, "links"},
		{"component ends in a hyphen", "components:\n  a-: {kind: static, route: /}\n", site, "components.a-"},
		{"component double hyphen", "components:\n  a--b: {kind: static, route: /}\n", site, "components.a--b"},
		{"component over 20", "components:\n  " + strings.Repeat("a", 21) + ": {kind: static, route: /}\n", site, ""},
		{"resource ends in a hyphen", "resources:\n  a-: {type: cache}\n", site, "resources.a-"},
		{"resource double hyphen", "resources:\n  a--b: {type: cache}\n", site, "resources.a--b"},
		{"resource over 20", "resources:\n  " + strings.Repeat("a", 21) + ": {type: cache}\n", site, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, err := Parse([]byte(tc.src))
			if err != nil {
				t.Fatal(err)
			}
			// No slug: the format is checked on its own.
			_, err = Resolve(m, tc.tr, Options{Limits: DefaultLimits()})
			e := refused(t, err, CodeInvalid, ReasonValue)
			if tc.key != "" && e.Details["key"] != tc.key {
				t.Errorf("key = %v, want %s", e.Details["key"], tc.key)
			}
			if e.Details["line"] == nil && tc.key != "" {
				t.Errorf("no line in %v", e.Details)
			}
		})
	}
	for _, src := range []string{
		"name: a\n", "name: " + strings.Repeat("a", 40) + "\n", "name: a-b-1\n", "links: [a, b-c, 9]\n",
		"resources:\n  a-b: {type: cache}\n  " + strings.Repeat("c", 20) + ": {type: cache}\n",
	} {
		m, err := Parse([]byte(src))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Resolve(m, site, Options{Limits: DefaultLimits()}); err != nil {
			t.Errorf("%q: %v", src, err)
		}
	}
	// The slug check precedes the mismatch check.
	m, err := Parse([]byte("name: Bad Name\n"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = Resolve(m, site, Options{Slug: "other", Limits: DefaultLimits()})
	refused(t, err, CodeInvalid, ReasonValue)
	// A link to the app itself is still link_self.
	m, err = Parse([]byte("links: [my-app]\n"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = Resolve(m, site, Options{Slug: "my-app", Limits: DefaultLimits()})
	refused(t, err, CodeLinkSelf, "")
}

// TestResolveRefusesHeaderNamesThatAreNotTokens: a header name must be an
// RFC 9110 token and a value may hold no control character but tab, and
// the reserved check sees the validated name.
func TestResolveRefusesHeaderNamesThatAreNotTokens(t *testing.T) {
	for _, tc := range []struct {
		name, header, reason string
	}{
		{"trailing space", `"Set-Cookie ": x`, ReasonValue},
		{"trailing tab", `"Content-Length\t": "1"`, ReasonValue},
		{"leading space", `" set-cookie": x`, ReasonValue},
		{"empty name", `"": x`, ReasonValue},
		{"space inside", `"Bad Name": x`, ReasonValue},
		{"colon", `"A:B": x`, ReasonValue},
		{"non-ASCII", `"X-Ü": x`, ReasonValue},
		{"CRLF in value", `X-A: "v\r\nSet-Cookie: y"`, ReasonValue},
		{"LF in value", `X-A: "v\n"`, ReasonValue},
		{"NUL in value", `X-A: "v\0"`, ReasonValue},
		{"DEL in value", `X-A: "v\x7f"`, ReasonValue},
		{"reserved", `Set-Cookie: x`, ReasonHeaderReserved},
		{"reserved in capitals", `CONTENT-ENCODING: gzip`, ReasonHeaderReserved},
		{"reserved prefix", `X-Latere-Id: x`, ReasonHeaderReserved},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := resolve("static:\n  headers:\n    - {path: /a, "+tc.header+"}\n", site)
			refused(t, err, CodeInvalid, tc.reason)
		})
	}
	for _, h := range []string{`X-Frame-Options: DENY`, `"X-Tab": "a\tb"`, "Cache-Control: \"max-age=60, public\"", "X_Under.dot~: v"} {
		if _, err := resolve("static:\n  headers:\n    - {path: /a, "+h+"}\n", site); err != nil {
			t.Errorf("%s: %v", h, err)
		}
	}
}

// TestResolveTakesADirectoryNamedDotDotFoo: only ".." and "../x" leave
// the tree.
func TestResolveTakesADirectoryNamedDotDotFoo(t *testing.T) {
	tr := tree{"..foo/index.html": "", "index.html": ""}
	for _, src := range []string{
		"components:\n  c: {kind: static, path: ..foo, route: /}\n",
		"kind: static\nstatic: {dir: ..foo}\n",
		"kind: static\nstatic: {dir: a/../..foo}\n",
	} {
		if _, err := resolve(src, tr); err != nil {
			t.Errorf("%q: %v", src, err)
		}
	}
	for _, tc := range []struct{ src, reason string }{
		{"components:\n  c: {kind: static, path: .., route: /}\n", ReasonValue},
		{"components:\n  c: {kind: static, path: a/../../x, route: /}\n", ReasonValue},
		{"kind: static\nstatic: {dir: ..}\n", ReasonDirEscapes},
		{"kind: static\nstatic: {dir: ../x}\n", ReasonDirEscapes},
		{"kind: static\nstatic: {dir: /abs}\n", ReasonDirEscapes},
	} {
		_, err := resolve(tc.src, tr)
		refused(t, err, CodeInvalid, tc.reason)
	}
}

// TestResolveChecksTheDockerfilePathEscape: service.dockerfile.path that
// leaves the component is a value error, not a missing file.
func TestResolveChecksTheDockerfilePathEscape(t *testing.T) {
	for _, p := range []string{"../x", "/abs/Dockerfile", ".."} {
		_, err := resolve("kind: service\nservice:\n  dockerfile: {path: "+p+"}\n", svcTree)
		e := refused(t, err, CodeInvalid, ReasonValue)
		if e.Details["key"] != "service.dockerfile.path" {
			t.Errorf("%s: key = %v", p, e.Details["key"])
		}
	}
	_, err := resolve("kind: service\nservice:\n  dockerfile: {path: nope/Dockerfile}\n", svcTree)
	refused(t, err, CodeInvalid, ReasonDockerfileMissing)
}

// TestResolveRefusesRoutesOutsideTheCharacterSet: a route is clean, holds
// only path characters, and its refusal names its line.
func TestResolveRefusesRoutesOutsideTheCharacterSet(t *testing.T) {
	for _, r := range []string{"/a/", "/a?x=1", "/a#b", "/%2e%2e", "/a%20b", "/a*", "/*", "/a b", "/über", "/a//b", "/a/../b", "a", "/\u0430pi"} {
		src := "components:\n  web: {kind: static, route: /}\n  a:\n    kind: static\n    route: " + strconv.Quote(r) + "\n"
		_, err := resolve(src, site)
		e := refused(t, err, CodeInvalid, ReasonValue)
		if e.Details["key"] != "components.a.route" || e.Details["line"] != 5 {
			t.Errorf("route %q: details = %v", r, e.Details)
		}
	}
	for _, r := range []string{"/a", "/a/b-c_d.e~f", "/A9"} {
		src := "components:\n  web: {kind: static, route: /}\n  a: {kind: static, route: " + r + "}\n"
		if _, err := resolve(src, site); err != nil {
			t.Errorf("route %q: %v", r, err)
		}
	}
}

// TestResolveChecksPublicPathsAndHealth: public paths use the route
// characters with a wildcard only at the end, and health is a path on
// the service without whitespace.
func TestResolveChecksPublicPathsAndHealth(t *testing.T) {
	for _, tc := range []struct{ name, svc string }{
		{"public path with a query", "public_paths: [\"/a?x\"]"},
		{"public path with a percent", "public_paths: [\"/%2e%2e\"]"},
		{"public path with a space", "public_paths: [\"/a b\"]"},
		{"public path non-ASCII", "public_paths: [\"/über\"]"},
		{"public path with a middle wildcard", "public_paths: [\"/a*/b\"]"},
		{"public path without a slash", "public_paths: [a]"},
		{"health with a host", `health: "//evil.example/x"`},
		{"health without a slash", "health: healthz"},
		{"health with a space", `health: "/a b"`},
		{"health with a newline", `health: "/a\nb"`},
		{"health with a tab", `health: "/a\tb"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := resolve("kind: service\nservice:\n  "+tc.svc+"\n", svcTree)
			refused(t, err, CodeInvalid, ReasonValue)
		})
	}
	for _, svc := range []string{"public_paths: [\"/\", \"/a/*\", \"/a-b.c\"]", "health: /healthz?x=1"} {
		_, err := resolve("kind: service\nservice:\n  "+svc+"\n", svcTree)
		if err != nil {
			t.Errorf("%s: %v", svc, err)
		}
	}
}

// TestResolveCountsMigrateCharacters: migrate is limited in characters,
// as the schema's maxLength is, not in bytes.
func TestResolveCountsMigrateCharacters(t *testing.T) {
	for n, ok := range map[int]bool{600: true, 1000: true, 1001: false} {
		src := "kind: service\nservice:\n  migrate: " + strings.Repeat("\u8fc1", n) + "\n"
		_, err := resolve(src, svcTree)
		if ok && err != nil {
			t.Errorf("%d characters: %v", n, err)
		}
		if !ok {
			refused(t, err, CodeInvalid, ReasonMigrateLength)
		}
	}
}

// TestResolveReadsTheShippedManifests: the examples and three manifests
// in use resolve with DefaultLimits against a tree holding their paths.
func TestResolveReadsTheShippedManifests(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		tr        Tree
	}{
		{"minllm", "name: minllm\nkind: static\nstatic:\n  build: cd site && npm ci && npm run build\n  dir: site/dist\n  spa: false\n",
			tree{"site/dist/index.html": "", "site/package.json": "{}"}},
		{"root dir", "kind: static\nstatic:\n  build: \"\"\n  dir: .\n  spa: false\n", site},
		{"web and docs", "components:\n  web:\n    path: web\n    route: /\n    kind: static\n  docs:\n    path: docs\n    route: /docs\n    kind: static\n    strip_prefix: true\n    static:\n      spa: false\n      headers:\n        - {path: /page.html, X-Smoke-Component: docs}\n",
			tree{"web/index.html": "", "docs/index.html": ""}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, err := Parse([]byte(tc.src))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Resolve(m, tc.tr, Options{Limits: DefaultLimits()}); err != nil {
				t.Fatal(err)
			}
		})
	}
}
