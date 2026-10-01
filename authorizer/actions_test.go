// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package authorizer

import (
	"slices"
	"testing"

	"latere.ai/x/pkg/authz"
)

// TestTheTableIsFrozen holds the table to its released rows, in order and by
// kind: an action string never changes and never disappears, so a row
// removed or renamed here breaks every endpoint deciding by the constant.
// A new action is appended here and in the table together.
func TestTheTableIsFrozen(t *testing.T) {
	want := []authz.Action{
		{Name: "app.list", Kind: "app"},
		{Name: "app.create", Kind: "app"},
		{Name: "app.read", Kind: "app"},
		{Name: "app.settings", Kind: "app"},
		{Name: "app.delete", Kind: "app"},
		{Name: "app.release", Kind: "app"},
		{Name: "gate.access", Kind: "app"},
		{Name: "deploy.create", Kind: "app"},
		{Name: "env.write", Kind: "app"},
		{Name: "tunnel.open", Kind: "app"},
		{Name: "resource.manage", Kind: "app"},
		{Name: "signin.manage", Kind: "app"},
		{Name: "admin.access", Kind: "admin"},
		{Name: "usage.read", Kind: "usage"},
		{Name: "event.deliver", Kind: "event"},
	}
	v := Vocabulary()
	if v.Core != "insula" {
		t.Errorf("core is %q", v.Core)
	}
	if !slices.Equal(v.Actions, want) {
		t.Errorf("the table drifted:\n got %v\nwant %v", v.Actions, want)
	}
	if got := Actions(); len(got) != len(want) {
		t.Errorf("Actions lists %d, want %d", len(got), len(want))
	}
	for _, a := range want {
		if !Known(a.Name) || Kind(a.Name) != a.Kind {
			t.Errorf("%s: Known %v, Kind %q", a.Name, Known(a.Name), Kind(a.Name))
		}
	}
}

// TestKindsAndLabels: the four kinds in the table's order, each with the
// heading a picker shows.
func TestKindsAndLabels(t *testing.T) {
	v := Vocabulary()
	if got, want := v.Kinds(), []string{KindApp, KindAdmin, KindUsage, KindEvent}; !slices.Equal(got, want) {
		t.Errorf("kinds %v, want %v", got, want)
	}
	for kind, label := range map[string]string{
		KindApp: "Apps", KindAdmin: "Administration", KindUsage: "Usage", KindEvent: "Repository events",
	} {
		if got := v.Label(kind); got != label {
			t.Errorf("%s is labeled %q, want %q", kind, got, label)
		}
	}
}

// TestUnknownActionIsUnknown: a string outside the table has no kind and is
// not known, which the client refuses before the wire.
func TestUnknownActionIsUnknown(t *testing.T) {
	for _, action := range []string{"", "app", "app.explode", "App.read", "app.read ", "deploy.promote", "app.write"} {
		if Known(action) || Kind(action) != "" {
			t.Errorf("%q reads as one of the vocabulary", action)
		}
	}
}

// TestVocabularyIsACopy: a caller that changes the value it was handed
// changes nothing for the next caller.
func TestVocabularyIsACopy(t *testing.T) {
	v := Vocabulary()
	v.Actions[0].Name = "changed"
	if Vocabulary().Actions[0].Name != ActionAppList {
		t.Error("a caller's change reached the table")
	}
}
