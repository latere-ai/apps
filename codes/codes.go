// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

// Package codes is the registry of every error code Latere Apps answers
// with: the code, its HTTP status, the one user sentence, and the one
// hint. The API renders an error from this table, the CLI prints the same
// sentence offline from the copy it was compiled with, and the docs render
// the table from this package, so the three cannot disagree. The package
// depends on the standard library alone, so a client imports it without a
// server's dependencies.
package codes

// Code is one entry of the registry.
type Code struct {
	// Name is the stable machine identifier, the map key.
	Name string
	// Status is the HTTP status the API answers with.
	Status int
	// Message is the user sentence, fixed here.
	Message string
	// Hint is the next step, fixed here.
	Hint string
}

// Codes is the registry, keyed by Name. Every code the API answers with
// has an entry here, and a new code is added here before anything
// answers with it.
var Codes = map[string]Code{
	// Any address.
	"not_found": {Name: "not_found", Status: 404,
		Message: "There is nothing at this address.",
		Hint:    "Check the path and the app slug."},
	"internal_error": {Name: "internal_error", Status: 500,
		Message: "Something failed on the platform. The platform has been notified.",
		Hint:    "Try again in a minute."},
	// Sign-in and access.
	"unauthenticated": {Name: "unauthenticated", Status: 401,
		Message: "Sign in to continue.",
		Hint:    "Run `latere login`."},
	"audience_mismatch": {Name: "audience_mismatch", Status: 401,
		Message: "This token was issued for another service.",
		Hint:    "Run the command again; the CLI mints a token for this API."},
	"token_expired": {Name: "token_expired", Status: 401,
		Message: "This token has expired.",
		Hint:    "Run the command again to get a fresh token."},
	"forbidden": {Name: "forbidden", Status: 403,
		Message: "You do not have permission to do this.",
		Hint:    "Ask an owner or admin of the organization, or check the key's grants."},
	"authorizer_unavailable": {Name: "authorizer_unavailable", Status: 503,
		Message: "The platform could not check your access just now.",
		Hint:    "Try again in a minute."},
	// Apps and their names.
	"invalid_slug": {Name: "invalid_slug", Status: 400,
		Message: "This slug is not a valid address.",
		Hint:    "Use 1 to 40 lowercase letters, digits, and single hyphens."},
	"slug_taken": {Name: "slug_taken", Status: 409,
		Message: "This slug is already in use.",
		Hint:    "Pick another slug or leave it out to get a generated one."},
	"slug_reserved": {Name: "slug_reserved", Status: 409,
		Message: "This slug is reserved by the platform.",
		Hint:    "Pick another slug."},
	"slug_held": {Name: "slug_held", Status: 409,
		Message: "This slug was released recently and is held for its previous owner.",
		Hint:    "Pick another slug or wait seven days."},
	"slug_generation_failed": {Name: "slug_generation_failed", Status: 503,
		Message: "The platform could not find a free address.",
		Hint:    "Try again in a minute, or pass a slug."},
	"id_too_short": {Name: "id_too_short", Status: 400,
		Message: "A deploy id needs at least 8 characters.",
		Hint:    "Copy the id from `latere app deploys list`."},
	"app_not_found": {Name: "app_not_found", Status: 404,
		Message: "There is no app with this slug in this account.",
		Hint:    "Check the slug with `latere app list`, or switch organizations with `latere org`."},
	"app_paused": {Name: "app_paused", Status: 409,
		Message: "This app is paused.",
		Hint:    "Resume it in the console settings, then deploy again."},
	"app_deleting": {Name: "app_deleting", Status: 409,
		Message: "This app is being deleted.",
		Hint:    "Create a new app."},
	"setting_invalid": {Name: "setting_invalid", Status: 400,
		Message: "A setting has a value the platform does not accept.",
		Hint:    "Check the setting against the docs."},
	// Requests.
	"invalid_request": {Name: "invalid_request", Status: 400,
		Message: "The request could not be read.",
		Hint:    "Check the request body against the API reference."},
	// An app's repository.
	"repository_create_failed": {Name: "repository_create_failed", Status: 502,
		Message: "The app's repository could not be created.",
		Hint:    "Try again in a minute. If it happens twice, the platform is looking into it."},
	"repository_conflict": {Name: "repository_conflict", Status: 409,
		Message: "Someone pushed to this app while the deploy was being recorded.",
		Hint:    "Deploy again; the new commit will be recorded after theirs."},
	"repository_quota_exceeded": {Name: "repository_quota_exceeded", Status: 409,
		Message: "The app's repository has reached its size limit.",
		Hint:    "Remove large files from the tree and push again."},
	"repository_unavailable": {Name: "repository_unavailable", Status: 503,
		Message: "The app's repository did not answer.",
		Hint:    "Deploy again in a few minutes. The platform has been notified."},
	"git_token_invalid": {Name: "git_token_invalid", Status: 401,
		Message: "The repository did not accept your git credential.",
		Hint:    "Deploy again; the CLI mints a fresh credential for each deploy."},
}

// Lookup returns the entry for name and whether it exists.
func Lookup(name string) (Code, bool) {
	c, ok := Codes[name]
	return c, ok
}
