// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package authorizer

import (
	"encoding/json"
	"testing"

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
