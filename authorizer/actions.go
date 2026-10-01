// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package authorizer

import (
	"slices"

	"latere.ai/x/pkg/authz"
)

// Core is the name the shared contract carries this vocabulary under, which
// an endpoint's refusal of an unknown action names and which the grants on a
// key qualify their actions with: "insula:app.read".
const Core = "insula"

// The resource kinds. Every question about one app acts on KindApp; the
// other three name no app.
const (
	KindApp   = "app"
	KindAdmin = "admin"
	KindUsage = "usage"
	KindEvent = "event"
)

// The actions: every question the Apps server asks an authorizer. A constant
// never changes its string and never disappears.
const (
	// ActionAppList lists the apps of the request's tenant.
	ActionAppList = "app.list"
	// ActionAppCreate creates an app, or reclaims one within the hold that
	// follows a delete.
	ActionAppCreate = "app.create"
	// ActionAppRead reads an app and everything below it: its deploys and
	// releases, build and runtime logs, the event stream, the variable
	// listing with every secret value null, its data services and tunnels.
	ActionAppRead = "app.read"
	// ActionAppSettings changes an app's name, slug, visibility, state or
	// settings.
	ActionAppSettings = "app.settings"
	// ActionAppDelete deletes an app.
	ActionAppDelete = "app.delete"
	// ActionAppRelease moves an app's production to a release: a release tag
	// and a rollback.
	ActionAppRelease = "app.release"
	// ActionGateAccess is the access check of a private app, asked for a
	// signed-in visitor.
	ActionGateAccess = "gate.access"
	// ActionDeployCreate creates a preview from an upload, a push, or an
	// earlier deploy, redeploys the current release, or cancels a deploy.
	ActionDeployCreate = "deploy.create"
	// ActionEnvWrite sets, unsets, requests and imports variables.
	ActionEnvWrite = "env.write"
	// ActionTunnelOpen opens a tunnel to one of the app's targets.
	ActionTunnelOpen = "tunnel.open"
	// ActionResourceManage deletes, rotates and restores a data service.
	ActionResourceManage = "resource.manage"
	// ActionSigninManage enables, changes and disables the app's sign-in
	// client.
	ActionSigninManage = "signin.manage"
	// ActionAdminAccess is every route of the server's administration.
	ActionAdminAccess = "admin.access"
	// ActionUsageRead reads the usage meter.
	ActionUsageRead = "usage.read"
	// ActionEventDeliver delivers a repository event to the server.
	ActionEventDeliver = "event.deliver"
)

// table is the action table in its order, one row per action, and the one
// place the pairing of an action with its kind is written.
var table = []authz.Action{
	{Name: ActionAppList, Kind: KindApp},
	{Name: ActionAppCreate, Kind: KindApp},
	{Name: ActionAppRead, Kind: KindApp},
	{Name: ActionAppSettings, Kind: KindApp},
	{Name: ActionAppDelete, Kind: KindApp},
	{Name: ActionAppRelease, Kind: KindApp},
	{Name: ActionGateAccess, Kind: KindApp},
	{Name: ActionDeployCreate, Kind: KindApp},
	{Name: ActionEnvWrite, Kind: KindApp},
	{Name: ActionTunnelOpen, Kind: KindApp},
	{Name: ActionResourceManage, Kind: KindApp},
	{Name: ActionSigninManage, Kind: KindApp},
	{Name: ActionAdminAccess, Kind: KindAdmin},
	{Name: ActionUsageRead, Kind: KindUsage},
	{Name: ActionEventDeliver, Kind: KindEvent},
}

// labels is the name a person reads for each kind, where a picker groups a
// key's grants under a heading.
var labels = map[string]string{
	KindApp:   "Apps",
	KindAdmin: "Administration",
	KindUsage: "Usage",
	KindEvent: "Repository events",
}

// Vocabulary is the action table as the shared contract carries it: the
// client refuses an action outside it before the wire, an endpoint's
// scaffold answers 400 for one, the conformance suite drives a case per
// row, and a picker reads the heading of each kind off Label. The value is a
// fresh copy each call, so a caller that sorts or appends to it changes
// nothing here.
func Vocabulary() authz.Vocabulary {
	return authz.Vocabulary{Core: Core, Actions: slices.Clone(table)}.WithLabels(labels)
}

// Actions lists every action of the vocabulary, in the table's order.
func Actions() []string {
	out := make([]string, len(table))
	for i, a := range table {
		out[i] = a.Name
	}
	return out
}

// Kind is the resource kind an action acts on, and "" for a string outside
// the vocabulary.
func Kind(action string) string {
	for _, a := range table {
		if a.Name == action {
			return a.Kind
		}
	}
	return ""
}

// Known reports whether action is one of the vocabulary.
func Known(action string) bool { return Kind(action) != "" }
