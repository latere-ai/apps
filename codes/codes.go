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
	// Builds.
	"source_fetch_failed": {Name: "source_fetch_failed", Status: 502,
		Message: "The source for this deploy could not be fetched.",
		Hint:    "Deploy again. If it happens twice, the platform is looking into it."},
	"source_too_many_files": {Name: "source_too_many_files", Status: 400,
		Message: "The app has more files than a deploy allows.",
		Hint:    "Add the largest directories to `.latereignore` and deploy again. `latere app deploy --check` shows them."},
	"source_path_traversal": {Name: "source_path_traversal", Status: 400,
		Message: "The upload contains a path the platform refuses.",
		Hint:    "Remove paths with `..` or absolute paths and deploy again."},
	"install_failed": {Name: "install_failed", Status: 422,
		Message: "Installing dependencies failed.",
		Hint:    "Read the build log; the failing line is near the end."},
	"build_failed": {Name: "build_failed", Status: 422,
		Message: "The build command failed.",
		Hint:    "Read the build log; the failing line is near the end."},
	"static_dir_missing": {Name: "static_dir_missing", Status: 422,
		Message: "The build finished but produced no site in the expected directory.",
		Hint:    "Set `static.dir` in `latere-app.yaml` to where the build writes its output."},
	"static_output_too_large": {Name: "static_output_too_large", Status: 422,
		Message: "The built site is larger than a deploy allows.",
		Hint:    "Reduce the output or move large files to object storage."},
	"detect_failed": {Name: "detect_failed", Status: 422,
		Message: "The platform could not tell how to run this app.",
		Hint:    "Set `kind` and `service.start` in `latere-app.yaml`, or add a Dockerfile."},
	"build_timeout": {Name: "build_timeout", Status: 422,
		Message: "The build ran longer than 15 minutes and was stopped.",
		Hint:    "Make the build faster, or split the app into components."},
	"build_unschedulable": {Name: "build_unschedulable", Status: 503,
		Message: "The platform had no room to run this build.",
		Hint:    "Deploy again in a few minutes."},
	"registry_push_failed": {Name: "registry_push_failed", Status: 502,
		Message: "The built image could not be stored.",
		Hint:    "Deploy again. If it happens twice, the platform is looking into it."},
	"build_internal": {Name: "build_internal", Status: 500,
		Message: "Something went wrong on the platform during this build.",
		Hint:    "Deploy again. The platform has been notified."},
	"image_vulnerable": {Name: "image_vulnerable", Status: 422,
		Message: "The built image has a vulnerability above the platform's threshold.",
		Hint:    "Update the dependency the build log names and deploy again."},
	"capacity_exhausted": {Name: "capacity_exhausted", Status: 503,
		Message: "The platform is at capacity for new applications.",
		Hint:    "Deploy again later. The platform has been notified."},
	// An app's address.
	"unknown_application": {Name: "unknown_application", Status: 404,
		Message: "No application is deployed at this address.",
		Hint:    "Check the address, or deploy with `latere app deploy`."},
	"unknown_path": {Name: "unknown_path", Status: 404,
		Message: "This page does not exist.",
		Hint:    "Check the address."},
	"no_deploy": {Name: "no_deploy", Status: 503,
		Message: "This application has no release yet.",
		Hint:    "Release a preview with `latere app release <version>`."},
	"no_preview": {Name: "no_preview", Status: 404,
		Message: "This app has no preview ready.",
		Hint:    "Push a commit or run `latere app deploy`, then open this address again."},
	"starting": {Name: "starting", Status: 503,
		Message: "The application is starting. This page reloads in a few seconds.",
		Hint:    "Wait a moment."},
	"paused": {Name: "paused", Status: 402,
		Message: "This application is paused.",
		Hint:    "Resume it from the console."},
	"upstream_error": {Name: "upstream_error", Status: 502,
		Message: "The application did not answer.",
		Hint:    "Check its logs with `latere app logs`."},
	"upstream_timeout": {Name: "upstream_timeout", Status: 504,
		Message: "The application took too long to answer.",
		Hint:    "Check its logs with `latere app logs`."},
	"misdirected": {Name: "misdirected", Status: 421,
		Message: "This address is not served here.",
		Hint:    "Use a `latere.site` address."},
	"body_too_large": {Name: "body_too_large", Status: 413,
		Message: "The request is larger than 32 MiB.",
		Hint:    "Send less data per request."},
	"method_not_allowed": {Name: "method_not_allowed", Status: 405,
		Message: "This method is not supported for a static site.",
		Hint:    "Use GET, HEAD, or OPTIONS."},
	"rate_limited": {Name: "rate_limited", Status: 429,
		Message: "Too many requests.",
		Hint:    "Slow down and try again."},
	"private": {Name: "private", Status: 403,
		Message: "Your account does not have access to this application.",
		Hint:    "Ask the app's owner for access, or sign out and use another account."},
}

// Lookup returns the entry for name and whether it exists.
func Lookup(name string) (Code, bool) {
	c, ok := Codes[name]
	return c, ok
}
