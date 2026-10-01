// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

// Package manifest is the grammar of latere-app.yaml, the file at the root
// of an app's tree that says how Latere Apps builds and serves it: Parse
// reads the file and refuses a key the platform does not know, Detect
// tells a static site from a service when the file names no kind, Resolve
// applies the defaults and every rule to one tree, and Schema is the same
// grammar as a JSON Schema for editors and for the CLI's offline check.
//
// The server, the CLI and an agent's skill import this one package, so a
// manifest the CLI accepts offline is one the server accepts, and an
// error names the same code, key and line on both sides. Every Code here
// is a code of package codes.
package manifest
