// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
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
	// ReadFile reads one file. It may return only the first MaxDetectRead
	// bytes: detection uses no more than that, so a Tree need not return
	// the rest.
	ReadFile(name string) ([]byte, error)
}

// MaxDetectRead is the most of one file detection uses. A file longer than
// this is cut to it.
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
	Build    *string `json:"build,omitzero"`
}

// foreignJSON reads files the repository's author wrote: duplicate keys, invalid
// UTF-8 and any letter case are accepted, as the encoding/json decoder did.
var foreignJSON = json.JoinOptions(
	jsontext.AllowDuplicateNames(true),
	jsontext.AllowInvalidUTF8(true),
	json.MatchCaseInsensitiveNames(true),
)

type packageJSON struct {
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

// readPackageJSON reads the members of a package.json that detection uses
// from src, the first MaxDetectRead bytes of the file. It reads the top-level
// object member by member and keeps every one of scripts, dependencies and
// devDependencies read in full, so a file cut inside a large member still
// yields the members before it. A member cut in the middle, or of the wrong
// type, ends the read and is dropped whole: a half-read scripts object could
// hold a start script but miss the build script that makes the app static.
// Input that is not a JSON object yields the zero value.
func readPackageJSON(src []byte) packageJSON {
	var pkg packageJSON
	dec := jsontext.NewDecoder(bytes.NewReader(src), foreignJSON)
	if tok, err := dec.ReadToken(); err != nil || tok.Kind() != '{' {
		return pkg
	}
	for dec.PeekKind() == '"' {
		tok, err := dec.ReadToken()
		if err != nil {
			break
		}
		var into *map[string]string
		switch name := tok.String(); {
		case strings.EqualFold(name, "scripts"):
			into = &pkg.Scripts
		case strings.EqualFold(name, "dependencies"):
			into = &pkg.Dependencies
		case strings.EqualFold(name, "devDependencies"):
			into = &pkg.DevDependencies
		default:
			if err := dec.SkipValue(); err != nil {
				return pkg
			}
			continue
		}
		var m map[string]string
		if err := json.UnmarshalDecode(dec, &m, foreignJSON); err != nil {
			// A cut member fails to decode, and nothing after it was read.
			return pkg
		}
		if *into == nil {
			*into = m
		} else {
			maps.Copy(*into, m)
		}
	}
	return pkg
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
	seen := map[string]bool{}
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
		if !seen[top] {
			seen[top] = true
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
		pkg := readPackageJSON(read("package.json"))
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
