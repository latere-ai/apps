// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package codes

import (
	"strings"
	"testing"
)

// TestEveryCodeHasASentence: an entry with no sentence or no hint, a key
// that differs from its name, or a status outside the error range is a bug
// in the registry.
func TestEveryCodeHasASentence(t *testing.T) {
	if len(Codes) == 0 {
		t.Fatal("the registry is empty")
	}
	for key, c := range Codes {
		if c.Name != key {
			t.Errorf("%s: Name is %q", key, c.Name)
		}
		if c.Message == "" {
			t.Errorf("%s: empty Message", key)
		}
		if c.Hint == "" {
			t.Errorf("%s: empty Hint", key)
		}
		if c.Status < 400 || c.Status > 599 {
			t.Errorf("%s: status %d is not an error status", key, c.Status)
		}
	}
}

// TestBuildTimeoutNamesNoFigure: a build is stopped by more than one
// limit, each step's budget and the build's whole life, and the platform
// that runs the build holds them, so the sentence states none and the
// details name the one that was reached.
func TestBuildTimeoutNamesNoFigure(t *testing.T) {
	c, ok := Lookup("build_timeout")
	if !ok {
		t.Fatal("build_timeout is not in the registry")
	}
	if strings.ContainsAny(c.Message, "0123456789") {
		t.Errorf("build_timeout states a figure: %q", c.Message)
	}
}

func TestLookup(t *testing.T) {
	if c, ok := Lookup("not_found"); !ok || c.Status != 404 {
		t.Fatalf("not_found: %+v %v", c, ok)
	}
	if _, ok := Lookup("no_such_code"); ok {
		t.Fatal("an unknown code must not be found")
	}
}
