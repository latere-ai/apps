// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	_ "embed"
	"slices"
)

//go:embed schema.json
var schema []byte

// Schema is the grammar as a JSON Schema (draft 2020-12), for editors and
// for the CLI's offline check; the platform's docs publish the same bytes.
// Rules that need the tree or the platform's limits, such as a missing
// component path or a memory above the allowance, are Resolve's alone.
func Schema() []byte { return slices.Clone(schema) }
