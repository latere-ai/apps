// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package authorizer

import (
	"encoding/json"
	"testing"

	"latere.ai/x/pkg/authkit"
	"latere.ai/x/pkg/authz"
)

func render(t *testing.T, r authz.Resource) map[string]any {
	t.Helper()
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// TestAppRendersFlat: the app is one flat object, kind and id beside the
// fields, and an unset field is absent rather than empty.
func TestAppRendersFlat(t *testing.T) {
	got := render(t, App{
		ID: "a1", Owner: "https://issuer.example|u1", Slug: "shop",
		Keys: []string{"visibility"}, Via: ViaPush, Tag: "v1.2.0", Commit: "abc",
	}.Resource())
	want := map[string]any{
		"kind": "app", "id": "a1", "owner": "https://issuer.example|u1", "slug": "shop",
		"keys": []any{"visibility"}, "via": "push", "tag": "v1.2.0", "commit": "abc",
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for k, v := range want {
		gb, _ := json.Marshal(got[k])
		wb, _ := json.Marshal(v)
		if string(gb) != string(wb) {
			t.Errorf("%s: got %s, want %s", k, gb, wb)
		}
	}
	for _, absent := range []string{"visibility", "deploy", "host", "target", "name", "type"} {
		if _, ok := got[absent]; ok {
			t.Errorf("%s is sent empty", absent)
		}
	}
}

// TestCreateCarriesNoID: app.create names no app yet, so the resource has no
// id and a grant over one app does not cover it.
func TestCreateCarriesNoID(t *testing.T) {
	got := render(t, App{Owner: "o", Slug: "s", Visibility: "public"}.Resource())
	if _, ok := got["id"]; ok {
		t.Errorf("a create carries an id: %v", got)
	}
	if got["visibility"] != "public" {
		t.Errorf("visibility: %v", got)
	}
}

// TestOtherKinds: the three kinds that name no app render their own fields.
func TestOtherKinds(t *testing.T) {
	for name, tc := range map[string]struct {
		r    authz.Resource
		want map[string]any
	}{
		"admin": {Admin{Route: "/v1/apps/admin/tenants/{id}/pause", ID: "t1", Owner: "o"}.Resource(),
			map[string]any{"kind": "admin", "id": "t1", "route": "/v1/apps/admin/tenants/{id}/pause", "owner": "o"}},
		"usage, one tenant":   {Usage{Owner: "o"}.Resource(), map[string]any{"kind": "usage", "owner": "o"}},
		"usage, every tenant": {Usage{}.Resource(), map[string]any{"kind": "usage"}},
		"event": {Event{Event: "push", Delivery: "d1", Repo: "a1", Owner: "o"}.Resource(),
			map[string]any{"kind": "event", "event": "push", "delivery": "d1", "repo": "a1", "owner": "o"}},
	} {
		got := render(t, tc.r)
		if len(got) != len(tc.want) {
			t.Errorf("%s: got %v, want %v", name, got, tc.want)
			continue
		}
		for k, v := range tc.want {
			if got[k] != v {
				t.Errorf("%s: %s is %v, want %v", name, k, got[k], v)
			}
		}
	}
}

// TestAppActionsCarryTheAppID: every action within one app is of kind app
// and carries the app's id, so a key whose one grant names the app covers
// each of them for that app and for no other, and the two actions that
// name no app yet, app.list and app.create, are covered by a grant over the
// whole kind alone.
func TestAppActionsCarryTheAppID(t *testing.T) {
	claims := func(identifier string, actions ...string) map[string]any {
		qualified := make([]any, len(actions))
		for i, a := range actions {
			qualified[i] = Core + ":" + a
		}
		return map[string]any{
			"token_use":             authkit.TokenUsePAT,
			"authorization_details": []any{map[string]any{"type": authkit.GrantType, "actions": qualified, "identifier": identifier}},
		}
	}
	decide := func(c map[string]any, action string, res authz.Resource) bool {
		grants, err := authz.ParseGrants(c)
		if err != nil {
			t.Fatal(err)
		}
		req := authz.Request{Subject: "https://issuer.example|u1", Claims: c, Action: action, Resource: res}
		return authz.Restrict(Core, authz.Decision{Allow: true}, req, grants).Allow
	}
	for _, a := range Vocabulary().Actions {
		if a.Kind != KindApp {
			continue
		}
		mine := App{ID: "a1", Owner: "o"}.Resource()
		other := App{ID: "a2", Owner: "o"}.Resource()
		if mine.Kind != KindApp || mine.ID != "a1" {
			t.Fatalf("%s: the resource is %+v", a.Name, mine)
		}
		oneApp := claims("a1", a.Name)
		switch a.Name {
		case ActionAppList, ActionAppCreate:
			if decide(oneApp, a.Name, App{Owner: "o"}.Resource()) {
				t.Errorf("%s names no app and a one-app grant covers it", a.Name)
			}
			if !decide(claims("", a.Name), a.Name, App{Owner: "o"}.Resource()) {
				t.Errorf("%s: a grant over the kind does not cover it", a.Name)
			}
		default:
			if !decide(oneApp, a.Name, mine) || decide(oneApp, a.Name, other) {
				t.Errorf("%s: a grant naming a1 must cover a1 and not a2", a.Name)
			}
		}
	}
}
