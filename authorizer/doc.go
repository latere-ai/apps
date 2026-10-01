// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

// Package authorizer is the vocabulary an authorization endpoint for Latere
// Apps is written against: the actions the Apps server asks, the resource
// kind each one acts on, the shape of the resource each question carries,
// and the limits an allow may grant. Import it to write the endpoint the
// server's authorizer URL points at, or to validate a key's grants, instead
// of keeping a copy of the strings.
//
// The envelope on the wire is latere.ai/x/pkg/authz's, and this package
// declares none of it: the server POSTs a request with the caller's subject,
// its claims verbatim, an action and a resource, and reads back an allow or
// a deny. Vocabulary is the whole action table as authz.Vocabulary, which
// the client refuses an unknown action against, latere.ai/x/pkg/authz/server
// validates against, and latere.ai/x/pkg/authz/conformance drives its cases
// from:
//
//	conformance.Run(t, url, token, conformance.WithVocabulary(authorizer.Vocabulary()))
//
// Every action within one app acts on the kind "app" and carries the app's
// id as the resource id, whatever the action's name says it touches, because
// a grant names a resource by its id: a key granted one app covers deploying
// it, its variables, its tunnels, its data services, its sign-in client and
// its releases, and no other app. The kinds "admin", "usage" and "event"
// name no app and are for the platform's operators and its own services.
//
// An owner is a rendered subject, the issuer and the sub joined,
// "https://issuer.example|9ab3" for a personal account and the issuer and
// the organization's id joined for an organization. There is no other
// spelling of a principal.
//
// The promise: additive within a module major. An action string never
// changes and never disappears, a kind stays the kind it is, a resource
// field and a limits member keep their wire names and their meanings.
package authorizer
