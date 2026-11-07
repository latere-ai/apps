// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"latere.ai/x/apps/codes"
)

// TestDetectionRules: every rule has a positive and a negative tree, the
// first rule that matches wins, and rule 7 is kind_undetectable with its
// documented sentence.
func TestDetectionRules(t *testing.T) {
	pkg := func(scripts, deps string) string {
		return `{"scripts": {` + scripts + `}, "dependencies": {` + deps + `}}`
	}
	for _, tc := range []struct {
		name  string
		tr    tree
		rule  int
		kind  string
		dir   string
		build string
	}{
		{"rule 1 dockerfile", tree{"Dockerfile": "FROM x", "package.json": pkg(`"start": "x"`, "")}, 1, KindService, "", ""},
		{"rule 2 start script", tree{"package.json": pkg(`"start": "node s.js"`, "")}, 2, KindService, "", ""},
		{"rule 2 start beside a build without a marker", tree{"package.json": pkg(`"start": "x", "build": "y"`, "")}, 2, KindService, "", ""},
		{"rule 3 vite", tree{"package.json": pkg(`"build": "vite build"`, ""), "vite.config.ts": ""}, 3, KindStatic, "dist", "npm run build"},
		{"rule 3 vite with bun", tree{"package.json": pkg(`"build": "vite build"`, ""), "vite.config.js": "", "bun.lock": ""}, 3, KindStatic, "dist", "bun run build"},
		{"rule 3 astro beats start", tree{"package.json": pkg(`"build": "astro build", "start": "x"`, ""), "astro.config.mjs": ""}, 3, KindStatic, "dist", "npm run build"},
		{"rule 3 svelte static", tree{"package.json": pkg(`"build": "x"`, ""), "svelte.config.js": "import adapter from '@sveltejs/adapter-static'"}, 3, KindStatic, "build", "npm run build"},
		{"rule 3 next export", tree{"package.json": pkg(`"build": "next build"`, `"next": "15"`), "next.config.js": "module.exports = { output: 'export' }"}, 3, KindStatic, "out", "npm run build"},
		{"rule 3 react scripts", tree{"package.json": pkg(`"build": "react-scripts build"`, `"react-scripts": "5"`)}, 3, KindStatic, "build", "npm run build"},
		{"rule 4 next server", tree{"package.json": pkg(`"build": "next build"`, `"next": "15"`), "next.config.js": "module.exports = {}"}, 4, KindService, "", ""},
		{"rule 5 plain html", tree{"index.html": "", "style.css": ""}, 5, KindStatic, ".", ""},
		{"rule 6 go", tree{"go.mod": "module x", "main.go": ""}, 6, KindService, "", ""},
		{"rule 6 python", tree{"pyproject.toml": ""}, 6, KindService, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, err := Detect(tc.tr, ".")
			if err != nil {
				t.Fatal(err)
			}
			if d.Rule != tc.rule || d.Kind != tc.kind || d.Dir != tc.dir || (d.Build != nil) != (tc.rule == 3 || tc.rule == 5) ||
				(d.Build != nil && *d.Build != tc.build) {
				t.Errorf("Detect = %+v", d)
			}
		})
	}

	// The negatives: a marker without a build script, a svelte config
	// without the static adapter, index.html beside a package.json, and
	// unreadable JSON all fall through to the rule after them.
	for _, tc := range []struct {
		name string
		tr   tree
		rule int
	}{
		{"vite without a build script", tree{"package.json": pkg(`"start": "x"`, ""), "vite.config.ts": ""}, 2},
		{"svelte without the static adapter", tree{"package.json": pkg(`"build": "x", "start": "y"`, ""), "svelte.config.js": "adapter-node"}, 2},
		{"next without export", tree{"package.json": pkg(`"build": "next build"`, `"next": "15"`)}, 4},
		{"index.html beside a package.json", tree{"index.html": "", "package.json": pkg("", ""), "go.mod": ""}, 6},
		{"broken package.json", tree{"package.json": "{", "requirements.txt": ""}, 6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, err := Detect(tc.tr, ".")
			if err != nil || d.Rule != tc.rule {
				t.Errorf("Detect = %+v, %v; want rule %d", d, err, tc.rule)
			}
		})
	}

	// A component under a path is detected from its own files only.
	d, err := Detect(tree{"web/index.html": "", "api/go.mod": ""}, "web")
	if err != nil || d.Rule != 5 {
		t.Errorf("Detect(web) = %+v, %v", d, err)
	}

	// Rule 7.
	_, err = Detect(tree{"README.md": "", "src/a.txt": ""}, ".")
	e := refused(t, err, CodeKindUndetectable, "")
	if got := e.Details["entries"]; len(got.([]string)) != 2 {
		t.Errorf("entries = %v", got)
	}
	c, ok := codes.Lookup(CodeKindUndetectable)
	if !ok || c.Message != "The platform could not tell whether this is a static site or a service." {
		t.Errorf("rule 7's sentence = %q", c.Message)
	}
}

// cutTree is a Tree like the platform's: ReadFile returns only the first
// MaxDetectRead bytes of a file.
type cutTree struct{ tree }

func (c cutTree) ReadFile(name string) ([]byte, error) {
	b, err := c.tree.ReadFile(name)
	if err != nil {
		return nil, err
	}
	return b[:min(len(b), MaxDetectRead)], nil
}

// padDeps is a dependencies object past MaxDetectRead bytes.
func padDeps() string {
	return `{` + strings.Repeat(`"p":"1",`, MaxDetectRead/7+1) + `"q":"1"}`
}

// TestDetectLargePackageJSON: a package.json longer than MaxDetectRead
// keeps the members read in full before the cut, so the rules still see
// scripts that come first.
func TestDetectLargePackageJSON(t *testing.T) {
	deps := padDeps()
	for _, tc := range []struct {
		name string
		tr   tree
		rule int
	}{
		{"rule 2 start near the top", tree{"package.json": `{"scripts":{"start":"node s.js"},"dependencies":` + deps + `}`}, 2},
		{"rule 3 build near the top", tree{"package.json": `{"scripts":{"build":"vite build"},"dependencies":` + deps + `}`, "vite.config.ts": ""}, 3},
		{"rule 4 next in dependencies before the cut", tree{"package.json": `{"dependencies":{"next":"15"},"scripts":{"x":"` + strings.Repeat("a", MaxDetectRead) + `"}}`}, 4},
		{"member names in another case, duplicates", tree{"package.json": `{"Scripts":{"a":"b"},"scripts":{"start":"x"},"description":"` + strings.Repeat("d", MaxDetectRead) + `"}`}, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.tr["package.json"]) <= MaxDetectRead {
				t.Fatalf("package.json is %d bytes, not over MaxDetectRead", len(tc.tr["package.json"]))
			}
			for name, tr := range map[string]Tree{"whole file": tc.tr, "cut by the tree": cutTree{tc.tr}} {
				d, err := Detect(tr, ".")
				if err != nil || d.Rule != tc.rule {
					t.Errorf("%s: Detect = %+v, %v; want rule %d", name, d, err, tc.rule)
				}
			}
		})
	}
}

// TestDetectCutInsideScripts: a scripts object cut in the middle is dropped
// whole, not kept in part. A half-read scripts object could hold start but
// miss the build script that makes the app static, so detection fails
// safe: it says undetectable instead of guessing.
func TestDetectCutInsideScripts(t *testing.T) {
	pkgJSON := `{"scripts":{"start":"node s.js","pad":"` + strings.Repeat("a", MaxDetectRead) + `","build":"vite build"}}`
	_, err := Detect(tree{"package.json": pkgJSON, "vite.config.ts": ""}, ".")
	refused(t, err, CodeKindUndetectable, "")
}

// TestDetectPackageJSONNotJSON: a package.json that is not a JSON object
// reads as empty, as before.
func TestDetectPackageJSONNotJSON(t *testing.T) {
	for _, src := range []string{"", "not json", "[1,2]", `"scripts"`} {
		d, err := Detect(tree{"package.json": src, "go.mod": ""}, ".")
		if err != nil || d.Rule != 6 {
			t.Errorf("package.json %q: Detect = %+v, %v; want rule 6", src, d, err)
		}
	}
}

// TestDetectIsLinearInEntries: Detect over many top-level files returns the
// first 20 entries sorted, and finishes within a bound a quadratic scan of
// the entries (seconds at this size) would miss by far.
func TestDetectIsLinearInEntries(t *testing.T) {
	const n = 200000
	tr := make(tree, n)
	for i := range n {
		tr[fmt.Sprintf("f%06d.txt", i)] = ""
	}
	start := time.Now()
	_, err := Detect(tr, ".")
	took := time.Since(start)
	e := refused(t, err, CodeKindUndetectable, "")
	got, ok := e.Details["entries"].([]string)
	if !ok || len(got) != 20 || !slices.IsSorted(got) || got[0] != "f000000.txt" {
		t.Errorf("entries = %v", e.Details["entries"])
	}
	if took > 5*time.Second {
		t.Errorf("Detect over %d top-level files took %v", n, took)
	}
}
