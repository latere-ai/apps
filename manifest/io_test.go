// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// TestNoPackageImportsIO: the module's own code imports no package that
// opens a file, a socket or a process, so a manifest, a tree or a limits
// object can only reach what the caller hands in.
func TestNoPackageImportsIO(t *testing.T) {
	banned := []string{"os", "os/exec", "net", "net/http", "syscall", "io/ioutil", "plugin"}
	for _, dir := range []string{".", "../codes", "../authorizer"} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		checked := 0
		for _, e := range entries {
			name := e.Name()
			if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, name), nil, parser.ImportsOnly)
			if err != nil {
				t.Fatal(err)
			}
			for _, imp := range f.Imports {
				p, err := strconv.Unquote(imp.Path.Value)
				if err != nil {
					t.Fatal(err)
				}
				if slices.Contains(banned, p) {
					t.Errorf("%s imports %s", filepath.Join(dir, name), p)
				}
			}
			checked++
		}
		if checked == 0 {
			t.Errorf("%s has no Go files to check", dir)
		}
	}
}
