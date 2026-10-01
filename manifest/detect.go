// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"bytes"
	"encoding/json"
	"maps"
	"path"
	"slices"
	"strings"
)

// Tree is the files of an app's tree. Names are slash-separated paths
// relative to the tree's root, files only.
type Tree interface {
	// Files lists every file of the tree.
	Files() []string
	// ReadFile reads one file; detection reads at most MaxDetectRead of it.
	ReadFile(name string) ([]byte, error)
}

// MaxDetectRead is the most of one file detection reads.
const MaxDetectRead = 64 << 10

// Detection is what detection found for one component: the rule that
// matched, its evidence, and the kind, output directory and build
// command it chose. It is stored with the deploy so the build never
// detects again.
type Detection struct {
	Rule     int     `json:"rule"`
	Evidence string  `json:"evidence"`
	Kind     string  `json:"kind"`
	Dir      string  `json:"dir,omitempty"`
	Build    *string `json:"build,omitempty"`
}

type packageJSON struct {
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

func (p *packageJSON) dep(name string) bool {
	_, a := p.Dependencies[name]
	_, b := p.DevDependencies[name]
	return a || b
}

// Detect applies the detection rules, in order, to the files under root,
// the component's path. The first rule that matches wins; none matching
// is CodeKindUndetectable. Rule 1 is a Dockerfile, which Resolve refuses
// with CodeDockerfile while Dockerfile builds are not available.
func Detect(t Tree, root string) (*Detection, error) {
	dir := path.Clean(root)
	at := func(name string) string {
		if dir == "." {
			return name
		}
		return dir + "/" + name
	}
	here := map[string]bool{}
	var entries []string
	for _, f := range t.Files() {
		rel := f
		if dir != "." {
			r, ok := strings.CutPrefix(f, dir+"/")
			if !ok {
				continue
			}
			rel = r
		}
		top, _, nested := strings.Cut(rel, "/")
		if !nested {
			here[top] = true
		}
		if !slices.Contains(entries, top) {
			entries = append(entries, top)
		}
	}
	names := slices.Sorted(maps.Keys(here))
	marker := func(prefix string) string {
		for _, name := range names {
			if strings.HasPrefix(name, prefix+".") {
				return name
			}
		}
		return ""
	}
	read := func(name string) []byte {
		b, err := t.ReadFile(at(name))
		if err != nil {
			return nil
		}
		return b[:min(len(b), MaxDetectRead)]
	}

	if here["Dockerfile"] {
		return &Detection{Rule: 1, Evidence: "Dockerfile", Kind: KindService}, nil
	}
	if here["package.json"] {
		var pkg packageJSON
		if err := json.Unmarshal(read("package.json"), &pkg); err != nil {
			pkg = packageJSON{}
		}
		build := "npm run build"
		if here["bun.lock"] || here["bun.lockb"] {
			build = "bun run build"
		}
		_, hasBuild := pkg.Scripts["build"]
		static, evidence := "", ""
		if hasBuild {
			switch {
			case marker("vite.config") != "":
				static, evidence = "dist", marker("vite.config")
			case marker("astro.config") != "":
				static, evidence = "dist", marker("astro.config")
			case marker("svelte.config") != "" && bytes.Contains(read(marker("svelte.config")), []byte("adapter-static")):
				static, evidence = "build", marker("svelte.config")
			case marker("next.config") != "" && exports(read(marker("next.config"))):
				static, evidence = "out", marker("next.config")
			case pkg.dep("react-scripts"):
				static, evidence = "build", "package.json react-scripts"
			}
		}
		if _, start := pkg.Scripts["start"]; start && static == "" {
			return &Detection{Rule: 2, Evidence: "package.json scripts.start", Kind: KindService}, nil
		}
		if static != "" {
			return &Detection{Rule: 3, Evidence: evidence, Kind: KindStatic, Dir: static, Build: &build}, nil
		}
		if pkg.dep("next") {
			return &Detection{Rule: 4, Evidence: "package.json next", Kind: KindService}, nil
		}
	}
	if here["index.html"] && !here["package.json"] {
		none := ""
		return &Detection{Rule: 5, Evidence: "index.html", Kind: KindStatic, Dir: ".", Build: &none}, nil
	}
	for _, f := range []string{"go.mod", "requirements.txt", "pyproject.toml", "Gemfile", "pom.xml", "build.gradle"} {
		if here[f] {
			return &Detection{Rule: 6, Evidence: f, Kind: KindService}, nil
		}
	}
	slices.Sort(entries)
	if len(entries) > 20 {
		entries = entries[:20]
	}
	return nil, fail(CodeKindUndetectable, "component", root, "entries", entries)
}

// exports reports whether a next.config sets output: 'export'.
func exports(src []byte) bool {
	s := strings.Join(strings.Fields(string(src)), "")
	return strings.Contains(s, `output:'export'`) || strings.Contains(s, `output:"export"`)
}
