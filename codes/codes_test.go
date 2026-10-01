// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package codes

import "testing"

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

func TestLookup(t *testing.T) {
	if c, ok := Lookup("not_found"); !ok || c.Status != 404 {
		t.Fatalf("not_found: %+v %v", c, ok)
	}
	if _, ok := Lookup("no_such_code"); ok {
		t.Fatal("an unknown code must not be found")
	}
}
