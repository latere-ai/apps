// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package authorizer

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"latere.ai/x/pkg/authz"
)

func decision(t *testing.T, limits string) authz.Decision {
	t.Helper()
	d, err := authz.ParseDecision([]byte(`{"allow":true,"limits":` + limits + `}`))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// TestTheMembersAreFrozen holds the limits object's wire names to the
// released list: a member keeps its name, and a new one is appended here.
func TestTheMembersAreFrozen(t *testing.T) {
	want := []string{
		"apps_max", "short_slugs", "deploys_per_hour", "components_max", "links_max", "resources_max",
		"source_bytes_max", "static_bytes_max", "static_files_max", "memory_max", "cpu_max",
		"build_jobs_max", "retained_deploys", "previews_max", "namespace_pods_max", "namespace_cpu",
		"namespace_memory", "edge_rps", "edge_burst", "sleep_after_min_seconds", "always_on",
		"env_vars_max", "log_streams_max", "tunnels_per_principal", "tunnels_per_app",
		"postgres_size_max", "postgres_connections_max", "cache_memory_max", "preview_copy_max",
	}
	typ := reflect.TypeFor[WireLimits]()
	var got []string
	for field := range typ.Fields() {
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		got = append(got, name)
	}
	if !slices.Equal(got, want) {
		t.Errorf("members drifted:\n got %v\nwant %v", got, want)
	}
}

// TestDecodeLimits: a member sent is read, a member absent stays nil, a
// member the server does not know is ignored, and zero is a figure.
func TestDecodeLimits(t *testing.T) {
	w, err := DecodeLimits(decision(t, `{"apps_max":0,"always_on":true,"memory_max":"2Gi","future_member":7}`))
	if err != nil {
		t.Fatal(err)
	}
	if w.AppsMax == nil || *w.AppsMax != 0 {
		t.Errorf("apps_max: %v", w.AppsMax)
	}
	if w.AlwaysOn == nil || !*w.AlwaysOn {
		t.Errorf("always_on: %v", w.AlwaysOn)
	}
	if w.MemoryMax == nil || *w.MemoryMax != "2Gi" {
		t.Errorf("memory_max: %v", w.MemoryMax)
	}
	if w.DeploysPerHour != nil {
		t.Errorf("an absent member decoded: %v", *w.DeploysPerHour)
	}
}

// TestNoLimits: a decision with no limits object is the zero value.
func TestNoLimits(t *testing.T) {
	d, err := authz.ParseDecision([]byte(`{"allow":true}`))
	if err != nil {
		t.Fatal(err)
	}
	w, err := DecodeLimits(d)
	if err != nil || !reflect.DeepEqual(w, WireLimits{}) {
		t.Errorf("got %+v, %v", w, err)
	}
}

// TestUnreadableLimits: a count below zero, an empty quantity, and a member
// of the wrong type are errors, so the caller holds no ceiling it misread.
func TestUnreadableLimits(t *testing.T) {
	for _, limits := range []string{
		`{"edge_rps":-1}`, `{"cpu_max":""}`, `{"namespace_memory":"  "}`, `{"apps_max":"five"}`,
	} {
		if _, err := DecodeLimits(decision(t, limits)); err == nil {
			t.Errorf("%s decoded", limits)
		}
	}
}

// TestRendersWhatItDecodes: an authorizer that renders through WireLimits
// sends exactly the members it set.
func TestRendersWhatItDecodes(t *testing.T) {
	n := int64(3)
	b, err := json.Marshal(WireLimits{PreviewsMax: &n})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"previews_max":3}` {
		t.Errorf("rendered %s", b)
	}
}
