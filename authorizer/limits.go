// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package authorizer

import (
	"fmt"
	"reflect"
	"strings"

	"latere.ai/x/pkg/authz"
)

// WireLimits is the limits object an allow carries, exported so an
// authorizer renders its answer through the type the server decodes rather
// than through string literals. Every member is optional: a member the
// object does not name stays nil and omitempty leaves it out, and the server
// takes its own default for it. A count is an integer, a switch a boolean,
// and a size or a CPU figure a Kubernetes quantity string such as "1Gi" or
// "1000m". Each member is carried on the action whose effect it bounds,
// named in its comment.
type WireLimits struct {
	// AppsMax is the tenant's number of apps, on app.create.
	AppsMax *int64 `json:"apps_max,omitempty"`
	// ShortSlugs allows slugs under the usual minimum length, on app.create
	// and app.settings.
	ShortSlugs *bool `json:"short_slugs,omitempty"`
	// DeploysPerHour is an app's deploys in its last hour, on deploy.create.
	DeploysPerHour *int64 `json:"deploys_per_hour,omitempty"`
	// ComponentsMax, LinksMax and ResourcesMax bound what a manifest
	// declares, on deploy.create.
	ComponentsMax *int64 `json:"components_max,omitempty"`
	LinksMax      *int64 `json:"links_max,omitempty"`
	ResourcesMax  *int64 `json:"resources_max,omitempty"`
	// SourceBytesMax is an uploaded or pushed source tree's size, on
	// deploy.create.
	SourceBytesMax *int64 `json:"source_bytes_max,omitempty"`
	// StaticBytesMax and StaticFilesMax bound a static build's output, on
	// deploy.create.
	StaticBytesMax *int64 `json:"static_bytes_max,omitempty"`
	StaticFilesMax *int64 `json:"static_files_max,omitempty"`
	// MemoryMax and CPUMax bound one service component, on deploy.create.
	MemoryMax *string `json:"memory_max,omitempty"`
	CPUMax    *string `json:"cpu_max,omitempty"`
	// BuildJobsMax is the tenant's concurrent builds, on deploy.create.
	BuildJobsMax *int64 `json:"build_jobs_max,omitempty"`
	// RetainedDeploys and PreviewsMax bound what an app keeps, on
	// deploy.create.
	RetainedDeploys *int64 `json:"retained_deploys,omitempty"`
	PreviewsMax     *int64 `json:"previews_max,omitempty"`
	// NamespacePodsMax, NamespaceCPU and NamespaceMemory bound an app's
	// namespace, on deploy.create.
	NamespacePodsMax *int64  `json:"namespace_pods_max,omitempty"`
	NamespaceCPU     *string `json:"namespace_cpu,omitempty"`
	NamespaceMemory  *string `json:"namespace_memory,omitempty"`
	// EdgeRPS and EdgeBurst bound the requests an app's address takes, on
	// deploy.create.
	EdgeRPS   *int64 `json:"edge_rps,omitempty"`
	EdgeBurst *int64 `json:"edge_burst,omitempty"`
	// SleepAfterMinSeconds is the shortest idle time before an app sleeps,
	// and AlwaysOn allows an app that never sleeps, on app.settings.
	SleepAfterMinSeconds *int64 `json:"sleep_after_min_seconds,omitempty"`
	AlwaysOn             *bool  `json:"always_on,omitempty"`
	// EnvVarsMax is an app's number of variables, on env.write.
	EnvVarsMax *int64 `json:"env_vars_max,omitempty"`
	// LogStreamsMax is an app's concurrent runtime log streams, on app.read.
	LogStreamsMax *int64 `json:"log_streams_max,omitempty"`
	// TunnelsPerPrincipal and TunnelsPerApp bound open tunnels, on
	// tunnel.open.
	TunnelsPerPrincipal *int64 `json:"tunnels_per_principal,omitempty"`
	TunnelsPerApp       *int64 `json:"tunnels_per_app,omitempty"`
	// PostgresSizeMax, PostgresConnectionsMax and CacheMemoryMax bound an
	// app's data services, and PreviewCopyMax the copy of production's
	// database a preview starts from, on deploy.create.
	PostgresSizeMax        *string `json:"postgres_size_max,omitempty"`
	PostgresConnectionsMax *int64  `json:"postgres_connections_max,omitempty"`
	CacheMemoryMax         *string `json:"cache_memory_max,omitempty"`
	PreviewCopyMax         *string `json:"preview_copy_max,omitempty"`
}

// DecodeLimits reads a decision's limits object. A decision with no limits
// is the zero WireLimits, and a member the server does not know is ignored.
// An object that does not parse, a count below zero, or an empty quantity is
// an error, and the caller treats the answer as no decision: a ceiling the
// server cannot read is not one it can hold. Whether a quantity string is a
// valid quantity is the server's to check, where its Kubernetes types are.
func DecodeLimits(d authz.Decision) (WireLimits, error) {
	var w WireLimits
	if err := d.DecodeLimits(&w); err != nil {
		return WireLimits{}, err
	}
	v := reflect.ValueOf(w)
	t := v.Type()
	for i := range t.NumField() {
		f := v.Field(i)
		if f.IsNil() {
			continue
		}
		name, _, _ := strings.Cut(t.Field(i).Tag.Get("json"), ",")
		switch e := f.Elem(); e.Kind() {
		case reflect.Int64:
			if e.Int() < 0 {
				return WireLimits{}, fmt.Errorf("limits.%s is %d, below zero", name, e.Int())
			}
		case reflect.String:
			if strings.TrimSpace(e.String()) == "" {
				return WireLimits{}, fmt.Errorf("limits.%s is an empty quantity", name)
			}
		}
	}
	return w, nil
}
