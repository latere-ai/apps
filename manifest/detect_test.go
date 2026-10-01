// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"testing"

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
