// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package authorizer

import "latere.ai/x/pkg/authz"

// The resource shapes, one type per kind. They render what the server sends
// and what an endpoint reads, so neither side writes a field name by hand.
// On the wire a resource is one flat object, kind and id beside the fields:
// resource.owner and not resource.fields.owner. A field the server does not
// know for a question is left out rather than sent empty.

// The values of App.Via: how a deploy or a release was asked for.
const (
	// ViaUpload is a preview from an uploaded working tree.
	ViaUpload = "upload"
	// ViaPush is a preview from a push, or a release from a pushed tag.
	ViaPush = "push"
	// ViaFromDeploy is a preview made again from an earlier deploy.
	ViaFromDeploy = "from_deploy"
	// ViaRedeploy is a redeploy of the current release.
	ViaRedeploy = "redeploy"
	// ViaCancel is the cancel of a deploy, named in App.Deploy.
	ViaCancel = "cancel"
	// ViaAPI is a release or a rollback asked through the API.
	ViaAPI = "api"
)

// App is the resource of every action of kind app. ID is the app's id on
// every action but app.list and app.create, which name no app yet; the other
// fields are sent on the actions that know them.
type App struct {
	// ID is the app's id.
	ID string
	// Owner is the rendered subject of the tenant the app belongs to.
	Owner string
	// Slug is the app's slug.
	Slug string
	// Visibility is public, unlisted or private, on the actions that read
	// or change it.
	Visibility string
	// Keys names what a change touches: the top-level fields and settings
	// keys of an app.settings body, the variable names of an env.write.
	Keys []string
	// Tag and Commit name the release of an app.release.
	Tag    string
	Commit string
	// Via is how a deploy.create or an app.release was asked for.
	Via string
	// Deploy is the id of the deploy a cancel names.
	Deploy string
	// Host is the host a gate.access check is for.
	Host string
	// Target is the target of a tunnel.open.
	Target string
	// Name and Type name the data service of a resource.manage.
	Name string
	Type string
}

// Resource renders the app as the envelope carries it.
func (a App) Resource() authz.Resource {
	return authz.NewResource(KindApp, a.ID, fields(
		field{"owner", a.Owner}, field{"slug", a.Slug}, field{"visibility", a.Visibility},
		strs("keys", a.Keys), field{"tag", a.Tag}, field{"commit", a.Commit},
		field{"via", a.Via}, field{"deploy", a.Deploy}, field{"host", a.Host},
		field{"target", a.Target}, field{"name", a.Name}, field{"type", a.Type},
	))
}

// Admin is the resource of admin.access: the route pattern asked for, and
// the app or tenant it names when it names one.
type Admin struct {
	Route string
	ID    string
	Owner string
}

// Resource renders the administration question as the envelope carries it.
func (a Admin) Resource() authz.Resource {
	return authz.NewResource(KindAdmin, a.ID, fields(field{"route", a.Route}, field{"owner", a.Owner}))
}

// Usage is the resource of usage.read: one tenant's meter when Owner is
// set, every tenant's when it is not.
type Usage struct {
	Owner string
}

// Resource renders the usage question as the envelope carries it.
func (u Usage) Resource() authz.Resource {
	return authz.NewResource(KindUsage, "", fields(field{"owner", u.Owner}))
}

// Event is the resource of event.deliver: one delivery of a repository
// event. Event is the event's type, under its own name because the envelope
// reserves kind and id for the resource itself.
type Event struct {
	// Event is the repository event's type, such as push.
	Event string
	// Delivery is the delivery's id, the same on every retry of it.
	Delivery string
	// Repo is the repository's id, which is the app's id.
	Repo string
	// Owner is the app's owner, when the repository is an app's.
	Owner string
}

// Resource renders the delivery as the envelope carries it.
func (e Event) Resource() authz.Resource {
	return authz.NewResource(KindEvent, "", fields(
		field{"event", e.Event}, field{"delivery", e.Delivery}, field{"repo", e.Repo}, field{"owner", e.Owner},
	))
}

type field struct {
	name  string
	value any
}

// strs is a list member, left out when the list is empty.
func strs(name string, v []string) field {
	if len(v) == 0 {
		return field{}
	}
	return field{name, v}
}

// fields gathers the members a resource carries, dropping the empty ones.
func fields(list ...field) map[string]any {
	out := map[string]any{}
	for _, f := range list {
		switch v := f.value.(type) {
		case nil:
			continue
		case string:
			if v == "" {
				continue
			}
		}
		out[f.name] = f.value
	}
	return out
}
