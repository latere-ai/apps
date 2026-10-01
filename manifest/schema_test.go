// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

// TestSchemaExamples: the JSON Schema accepts every example under
// testdata/examples and refuses every fixture under testdata/invalid, and
// Parse agrees on the examples.
func TestSchemaExamples(t *testing.T) {
	var root map[string]any
	if err := json.Unmarshal(Schema(), &root); err != nil {
		t.Fatal(err)
	}
	v := validator{root: root}
	for _, dir := range []string{"examples", "invalid"} {
		files, err := filepath.Glob(filepath.Join("testdata", dir, "*.yaml"))
		if err != nil || len(files) == 0 {
			t.Fatalf("%s: %v %v", dir, files, err)
		}
		for _, f := range files {
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			var doc any
			if err := yaml.Unmarshal(src, &doc); err != nil {
				t.Fatalf("%s: %v", f, err)
			}
			if doc == nil {
				doc = map[string]any{}
			}
			err = v.check(root, normalize(doc), "")
			switch dir {
			case "examples":
				if err != nil {
					t.Errorf("%s: the schema refuses an example: %v", f, err)
				}
				if _, perr := Parse(src); perr != nil {
					t.Errorf("%s: Parse refuses an example: %v", f, perr)
				}
			case "invalid":
				if err == nil {
					t.Errorf("%s: the schema accepts an invalid manifest", f)
				}
			}
		}
	}
}

// TestSchemaMatchesGrammar: the schema and Parse's grammar name the same
// keys at every level, so an editor's completion and the server's
// refusal of an unknown key agree.
func TestSchemaMatchesGrammar(t *testing.T) {
	var root map[string]any
	if err := json.Unmarshal(Schema(), &root); err != nil {
		t.Fatal(err)
	}
	defs := root["$defs"].(map[string]any)
	keys := func(s map[string]any) []string {
		if ref, ok := s["$ref"].(string); ok {
			s = defs[strings.TrimPrefix(ref, "#/$defs/")].(map[string]any)
		}
		props, _ := s["properties"].(map[string]any)
		return slices.Sorted(maps.Keys(props))
	}
	gkeys := func(g *grammar) []string { return slices.Sorted(maps.Keys(g.keys)) }
	for _, tc := range []struct {
		name   string
		schema map[string]any
		g      *grammar
	}{
		{"root", root, rootGrammar},
		{"static", defs["static"].(map[string]any), staticGrammar},
		{"service", defs["service"].(map[string]any), serviceGrammar},
		{"component", defs["component"].(map[string]any), componentGrammar},
		{"resource", defs["resource"].(map[string]any), rootGrammar.keys["resources"].each},
	} {
		if got, want := keys(tc.schema), gkeys(tc.g); !slices.Equal(got, want) {
			t.Errorf("%s: the schema names %v, the grammar %v", tc.name, got, want)
		}
	}
	if Schema()[0] != '{' {
		t.Error("Schema is not the JSON document")
	}
}

// normalize turns the YAML decoder's values into the JSON model: maps
// keyed by string, numbers as float64.
func normalize(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, e := range x {
			out[k] = normalize(e)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = normalize(e)
		}
		return out
	case uint64:
		return float64(x)
	case int64:
		return float64(x)
	case int:
		return float64(x)
	}
	return v
}

// validator checks a document against the subset of JSON Schema the
// manifest's schema uses: type, enum, properties, additionalProperties,
// propertyNames, required, items, maxItems, minimum, maximum, maxLength,
// pattern, $ref into $defs, dependentSchemas, not, and anyOf.
type validator struct{ root map[string]any }

func (v validator) check(s map[string]any, doc any, at string) error {
	if ref, ok := s["$ref"].(string); ok {
		s = v.root["$defs"].(map[string]any)[strings.TrimPrefix(ref, "#/$defs/")].(map[string]any)
	}
	if t, ok := s["type"]; ok && !typeMatches(t, doc) {
		return fmt.Errorf("%s: %v is not %v", at, doc, t)
	}
	if e, ok := s["enum"].([]any); ok && !slices.Contains(e, doc) {
		return fmt.Errorf("%s: %v is not one of %v", at, doc, e)
	}
	if n, ok := doc.(float64); ok {
		if lo, ok := s["minimum"].(float64); ok && n < lo {
			return fmt.Errorf("%s: %v < %v", at, n, lo)
		}
		if hi, ok := s["maximum"].(float64); ok && n > hi {
			return fmt.Errorf("%s: %v > %v", at, n, hi)
		}
	}
	if str, ok := doc.(string); ok {
		if p, ok := s["pattern"].(string); ok && !regexp.MustCompile(p).MatchString(str) {
			return fmt.Errorf("%s: %q does not match %s", at, str, p)
		}
		if n, ok := s["maxLength"].(float64); ok && float64(len(str)) > n {
			return fmt.Errorf("%s: longer than %v", at, n)
		}
	}
	if list, ok := doc.([]any); ok {
		if n, ok := s["maxItems"].(float64); ok && float64(len(list)) > n {
			return fmt.Errorf("%s: more than %v items", at, n)
		}
		if items, ok := s["items"].(map[string]any); ok {
			for i, e := range list {
				if err := v.check(items, e, fmt.Sprintf("%s[%d]", at, i)); err != nil {
					return err
				}
			}
		}
	}
	if obj, ok := doc.(map[string]any); ok {
		if err := v.object(s, obj, at); err != nil {
			return err
		}
	}
	if not, ok := s["not"].(map[string]any); ok && v.check(not, doc, at) == nil {
		return fmt.Errorf("%s: matches a schema it must not", at)
	}
	if alts, ok := s["anyOf"].([]any); ok {
		matched := false
		for _, alt := range alts {
			matched = matched || v.check(alt.(map[string]any), doc, at) == nil
		}
		if !matched {
			return fmt.Errorf("%s: matches none of anyOf", at)
		}
	}
	return nil
}

func (v validator) object(s map[string]any, obj map[string]any, at string) error {
	props, _ := s["properties"].(map[string]any)
	for _, r := range asStrings(s["required"]) {
		if _, ok := obj[r]; !ok {
			return fmt.Errorf("%s: %s is required", at, r)
		}
	}
	for _, k := range slices.Sorted(maps.Keys(obj)) {
		if names, ok := s["propertyNames"].(map[string]any); ok {
			if err := v.check(names, k, at+"."+k); err != nil {
				return err
			}
		}
		switch sub, ok := props[k].(map[string]any); {
		case ok:
			if err := v.check(sub, obj[k], at+"."+k); err != nil {
				return err
			}
		default:
			switch ap := s["additionalProperties"].(type) {
			case bool:
				if !ap {
					return fmt.Errorf("%s: %s is not allowed", at, k)
				}
			case map[string]any:
				if err := v.check(ap, obj[k], at+"."+k); err != nil {
					return err
				}
			}
		}
		if dep, ok := s["dependentSchemas"].(map[string]any); ok {
			if d, ok := dep[k].(map[string]any); ok {
				if err := v.check(d, obj, at); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func asStrings(v any) []string {
	list, _ := v.([]any)
	out := make([]string, 0, len(list))
	for _, e := range list {
		out = append(out, e.(string))
	}
	return out
}

func typeMatches(t any, doc any) bool {
	if list, ok := t.([]any); ok {
		return slices.ContainsFunc(list, func(e any) bool { return typeMatches(e, doc) })
	}
	switch t {
	case "object":
		_, ok := doc.(map[string]any)
		return ok
	case "array":
		_, ok := doc.([]any)
		return ok
	case "string":
		_, ok := doc.(string)
		return ok
	case "boolean":
		_, ok := doc.(bool)
		return ok
	case "number":
		_, ok := doc.(float64)
		return ok
	case "integer":
		n, ok := doc.(float64)
		return ok && n == math.Trunc(n)
	}
	return false
}
