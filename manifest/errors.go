// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// The codes a manifest is refused with; each is an entry of package
// codes, which holds its status, sentence and hint.
const (
	CodeUnknownKey          = "manifest_unknown_key"
	CodeShapeConflict       = "manifest_shape_conflict"
	CodeInvalid             = "manifest_invalid"
	CodeUnknownReference    = "manifest_unknown_reference"
	CodeKindUnsupported     = "component_kind_unsupported"
	CodePathMissing         = "component_path_missing"
	CodeRouteConflict       = "component_route_conflict"
	CodeRootMissing         = "component_root_missing"
	CodeRootConflict        = "component_root_conflict"
	CodeLinkSelf            = "link_self"
	CodeKindUndetectable    = "kind_undetectable"
	CodeDockerfile          = "dockerfile_unsupported"
	CodeReplicasUnsupported = "replicas_unsupported"
	CodeComponentQuota      = "component_quota_exceeded"
	CodeLinkQuota           = "link_quota_exceeded"
	CodeResourceQuota       = "resource_quota_exceeded"
)

// The reasons of CodeInvalid. The first group is the reference's; the
// last four name refusals the reference states as rules without a
// reason of their own.
const (
	ReasonNameMismatch         = "name_mismatch"
	ReasonDirEscapes           = "dir_escapes"
	ReasonHeaderReserved       = "header_reserved"
	ReasonBuilderUnknown       = "builder_unknown"
	ReasonPortRange            = "port_range"
	ReasonMemoryRange          = "memory_range"
	ReasonCPURange             = "cpu_range"
	ReasonPublicPathsTooMany   = "public_paths_too_many"
	ReasonDockerfileMissing    = "dockerfile_missing"
	ReasonBuilderAndDockerfile = "builder_and_dockerfile"
	ReasonResourceType         = "resource_type"
	ReasonResourceSize         = "resource_size"
	ReasonMigrateLength        = "migrate_length"

	// ReasonSyntax: the file is not YAML.
	ReasonSyntax = "syntax"
	// ReasonType: a value has the wrong type, such as a word for a port.
	ReasonType = "type"
	// ReasonValue: a value outside a key's closed set or form: a kind, a
	// component or resource name, a path, a route, a header rule.
	ReasonValue = "value"
	// ReasonKindKeys: a component carries the keys of the other kind, such
	// as a static component with a service block.
	ReasonKindKeys = "kind_keys"
)

// Error is a manifest refused with one code. Details are the developer
// detail of the code's entry: the key, its line, the reason, and what
// each code names.
type Error struct {
	Code    string
	Details map[string]any
}

func (e *Error) Error() string {
	var b strings.Builder
	b.WriteString(e.Code)
	for _, k := range slices.Sorted(maps.Keys(e.Details)) {
		fmt.Fprintf(&b, " %s=%v", k, e.Details[k])
	}
	return b.String()
}

// fail is an Error with its details as key and value pairs.
func fail(code string, kv ...any) *Error {
	d := map[string]any{}
	for i := 0; i+1 < len(kv); i += 2 {
		d[fmt.Sprint(kv[i])] = kv[i+1]
	}
	return &Error{Code: code, Details: d}
}

// invalid is CodeInvalid for key with reason, at the key's line when the
// manifest knows it.
func (m *Manifest) invalid(key, reason string) *Error {
	e := fail(CodeInvalid, "key", key, "reason", reason)
	if line := m.line(key); line > 0 {
		e.Details["line"] = line
	}
	return e
}

func (m *Manifest) line(key string) int {
	if m == nil || m.lines == nil {
		return 0
	}
	return m.lines[key]
}
