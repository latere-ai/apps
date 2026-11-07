// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

// Package manifest is the grammar of latere-app.yaml, the file at the root
// of an app's tree that says how Latere Apps builds and serves it: Parse
// reads the file and refuses a key the platform does not know, Detect
// tells a static site from a service when the file names no kind, Resolve
// applies the defaults and every rule to one tree, and Schema is the same
// grammar as a JSON Schema for editors and for a client's offline check.
//
// The Latere Apps server parses and resolves every manifest with this
// package, so a client that imports it refuses a manifest offline with the
// same code, key and line the server answers with. The server adds checks
// of its own, such as the names build.env may not use, so a manifest
// accepted offline can still be refused. Every Code here is a code of
// package codes.
package manifest
