// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"encoding/json/v2"
	"fmt"
	"maps"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

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
// manifest's schema uses: type, enum, const, properties,
// additionalProperties, propertyNames, required, minProperties, items,
// uniqueItems, maxItems, minimum, maximum, minLength, maxLength (both count
// characters, not bytes), pattern, $ref into $defs beside its sibling
// keywords, dependentSchemas, allOf, anyOf, not, and if with then.
type validator struct{ root map[string]any }

func (v validator) check(s map[string]any, doc any, at string) error {
	if ref, ok := s["$ref"].(string); ok {
		target := v.root["$defs"].(map[string]any)[strings.TrimPrefix(ref, "#/$defs/")].(map[string]any)
		if err := v.check(target, doc, at); err != nil {
			return err
		}
	}
	if t, ok := s["type"]; ok && !typeMatches(t, doc) {
		return fmt.Errorf("%s: %v is not %v", at, doc, t)
	}
	if e, ok := s["enum"].([]any); ok && !slices.Contains(e, doc) {
		return fmt.Errorf("%s: %v is not one of %v", at, doc, e)
	}
	if c, ok := s["const"]; ok && !reflect.DeepEqual(c, doc) {
		return fmt.Errorf("%s: %v is not %v", at, doc, c)
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
		if n, ok := s["maxLength"].(float64); ok && float64(utf8.RuneCountInString(str)) > n {
			return fmt.Errorf("%s: longer than %v characters", at, n)
		}
		if n, ok := s["minLength"].(float64); ok && float64(utf8.RuneCountInString(str)) < n {
			return fmt.Errorf("%s: shorter than %v characters", at, n)
		}
	}
	if list, ok := doc.([]any); ok {
		if n, ok := s["maxItems"].(float64); ok && float64(len(list)) > n {
			return fmt.Errorf("%s: more than %v items", at, n)
		}
		if u, _ := s["uniqueItems"].(bool); u {
			for i := range list {
				for j := range i {
					if reflect.DeepEqual(list[i], list[j]) {
						return fmt.Errorf("%s: items %d and %d are equal", at, j, i)
					}
				}
			}
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
	if alts, ok := s["allOf"].([]any); ok {
		for _, alt := range alts {
			if err := v.check(alt.(map[string]any), doc, at); err != nil {
				return err
			}
		}
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
	if cond, ok := s["if"].(map[string]any); ok && v.check(cond, doc, at) == nil {
		if then, ok := s["then"].(map[string]any); ok {
			if err := v.check(then, doc, at); err != nil {
				return err
			}
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
	if n, ok := s["minProperties"].(float64); ok && float64(len(obj)) < n {
		return fmt.Errorf("%s: fewer than %v properties", at, n)
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

// TestSchemaIsNamedWhereTheDocsServeIt holds the schema's $id to the address
// the platform's docs publish these bytes at, beside the manifest reference.
func TestSchemaIsNamedWhereTheDocsServeIt(t *testing.T) {
	var doc struct {
		ID string `json:"$id"`
	}
	if err := json.Unmarshal(Schema(), &doc); err != nil {
		t.Fatal(err)
	}
	if want := "https://platform.latere.ai/docs/apps/latere-app.schema.json"; doc.ID != want {
		t.Errorf("$id = %q, want %q", doc.ID, want)
	}
}

// agreeTree holds every path the agreement cases name, so a refusal is
// never a missing directory.
var agreeTree = tree{
	"index.html": "", "go.mod": "module x",
	"web/index.html": "", "api/go.mod": "module api",
	"apps/web/index.html": "", "apps/api/go.mod": "module api",
	"..foo/index.html": "",
}

// verdicts is the schema's verdict and Parse plus Resolve's on one source:
// no slug, DefaultLimits and agreeTree.
func verdicts(t *testing.T, src string) (schemaErr, goErr error) {
	t.Helper()
	var root map[string]any
	if err := json.Unmarshal(Schema(), &root); err != nil {
		t.Fatal(err)
	}
	var doc any
	if err := yaml.Unmarshal([]byte(src), &doc); err != nil {
		schemaErr = err
	} else {
		if doc == nil {
			doc = map[string]any{}
		}
		schemaErr = validator{root: root}.check(root, normalize(doc), "")
	}
	m, err := Parse([]byte(src))
	if err == nil {
		_, err = Resolve(m, agreeTree, Options{Limits: DefaultLimits()})
	}
	return schemaErr, err
}

// The directions in which a case may differ. A case that differs is a
// known difference and says why; the test holds it to its direction, so a
// fix on either side shows as a failure to update the list.
const (
	// schemaOnlyAccepts: the schema says valid and Parse or Resolve refuses.
	schemaOnlyAccepts = "schema accepts, Parse+Resolve refuses"
	// schemaOnlyRefuses: the schema says invalid and Parse and Resolve accept.
	schemaOnlyRefuses = "schema refuses, Parse+Resolve accepts"
)

type agreeCase struct {
	name, src string
	// diff and why are empty when the two verdicts agree.
	diff, why string
}

// TestSchemaAgreesWithParseAndResolve runs the testdata files and hand-made
// cases through the schema and through Parse plus Resolve, and holds the
// two verdicts equal except where a case names its difference. Resolve's
// own rules that need the tree or the platform's limits are not cases here;
// the differences below are the schema's known limits, each with its reason.
func TestSchemaAgreesWithParseAndResolve(t *testing.T) {
	const (
		byDesign  = "not available yet: Resolve refuses what the schema admits on purpose"
		yamlLevel = "the YAML decoder coerces it before the schema sees it"
		fileLevel = "a rule of the file, not of the document the schema reads"
		limits    = "a figure of the platform's limits, Resolve's alone"
		crossRef  = "needs another part of the manifest, which a schema cannot compare"
		stricter  = "the schema refuses a spelling Resolve normalizes"
	)
	rep := strings.Repeat
	svc := func(body string) string { return "kind: service\nservice:\n" + body }
	hdr := func(rule string) string { return "kind: static\nstatic:\n  headers:\n    - " + rule + "\n" }
	comp := func(name, route string) string {
		return "components:\n  " + name + ": {kind: static, path: web, route: " + route + "}\n"
	}
	two := func(route string) string {
		return "components:\n  w: {kind: static, path: web, route: /}\n  a: {kind: service, path: api, route: " + route + "}\n"
	}
	envOf := func(v string) string { return "env:\n  X: " + v + "\n" }
	var manyVars strings.Builder
	manyVars.WriteString("env:\n")
	for i := range 1001 {
		fmt.Fprintf(&manyVars, "  V%d: x\n", i)
	}

	var cases []agreeCase
	for _, dir := range []string{"examples", "invalid"} {
		files, err := filepath.Glob(filepath.Join("testdata", dir, "*.yaml"))
		if err != nil || len(files) == 0 {
			t.Fatalf("%s: %v %v", dir, files, err)
		}
		for _, f := range files {
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			c := agreeCase{name: f, src: string(b)}
			if f == filepath.Join("testdata", "examples", "components.yaml") {
				c.diff, c.why = schemaOnlyAccepts, byDesign+": its cron component"
			}
			cases = append(cases, c)
		}
	}
	add := func(cs ...agreeCase) { cases = append(cases, cs...) }
	ok := func(name, src string) agreeCase { return agreeCase{name: name, src: src} }

	// names and slugs
	add(
		ok("name with a space", "name: \"Bad Name\"\n"),
		ok("name of 41 characters", "name: "+rep("a", 41)+"\n"),
		ok("name of 40 characters", "name: "+rep("a", 40)+"\n"),
		ok("name with a double hyphen", "name: my--app\n"),
		ok("name ending in a hyphen", "name: my-\n"),
		ok("name with digits and hyphens", "name: a1-b2-3\n"),
		agreeCase{"name empty", "name: \"\"\n", schemaOnlyRefuses, stricter + ": an empty name means no name"},
		ok("component a--b", comp("a--b", "/")),
		ok("component name of 20 characters", comp(rep("a", 20), "/")),
		ok("component name of 21 characters", comp(rep("a", 21), "/")),
		ok("component name starting with a digit", comp("1a", "/")),
		ok("component name ending in a hyphen", comp("a-", "/")),
		ok("component name with hyphens", comp("web-app-2", "/")),
		ok("resource a--b", "resources:\n  a--b: {type: cache}\n"),
		ok("resource name of 21 characters", "resources:\n  "+rep("a", 21)+": {type: cache}\n"),
		ok("resource name of 20 characters", "resources:\n  "+rep("a", 20)+": {type: cache}\n"),
		ok("resource without a type", "resources:\n  db: {size: 1Gi}\n"),
		ok("resource null", "resources:\n  db: ~\n"),
	)
	// links
	add(
		ok("links one", "links: [api]\n"),
		ok("link duplicated", "links: [api, api]\n"),
		ok("link with capitals", "links: [Api]\n"),
		ok("link of 41 characters", "links: ["+rep("a", 41)+"]\n"),
		ok("link a--b", "links: [a--b]\n"),
		ok("link starting with a digit", "links: [1a]\n"),
		agreeCase{"links of ten", "links: [" + strings.Join([]string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}, ", ") + "]\n", "", ""},
		agreeCase{"links of eleven", "links: [a, b, c, d, e, f, g, h, i, j, k]\n", schemaOnlyAccepts, limits + ": links_max"},
		agreeCase{"links of a number and a bool", "links: [1, true]\n", schemaOnlyRefuses, yamlLevel},
		agreeCase{"links null", "links: ~\n", schemaOnlyRefuses, yamlLevel},
	)
	// components and routes
	add(
		ok("component without a route", "components:\n  web: {kind: static, path: web}\n"),
		ok("component null", "components:\n  web: ~\n"),
		ok("component with a route", comp("web", "/")),
		ok("component route /api", two("/api")),
		ok("component route /api/", two("/api/")),
		ok("component route //api", two("//api")),
		ok("component route /api//x", two("/api//x")),
		ok("component route /a/./b", two("/a/./b")),
		ok("component route /a/../b", two("/a/../b")),
		ok("component route /..", two("/..")),
		ok("component route /.", two("/.")),
		ok("component route /..a", two("/..a")),
		ok("component route /...", two("/...")),
		ok("component route /a.", two("/a.")),
		ok("component route /v1.2/x_y~z", two("/v1.2/x_y~z")),
		ok("component route with a space", two("'/a b'")),
		ok("component route with a percent", two("/a%20b")),
		ok("component route with a star", two("/a*")),
		ok("component route without a slash", two("api")),
		ok("component route empty", two("''")),
		ok("component route accented", two("/é")),
		ok("components beside kind", "kind: static\ncomponents:\n  web: {kind: static, path: web, route: /}\n"),
		agreeCase{"components {} beside kind", "components: {}\nkind: static\n", schemaOnlyRefuses, yamlLevel + ": an empty mapping counts as none"},
		agreeCase{"components null", "components: ~\n", schemaOnlyRefuses, yamlLevel},
		agreeCase{"components null beside kind", "components: ~\nkind: static\n", schemaOnlyRefuses, yamlLevel},
		agreeCase{"component kind worker", "components:\n  web: {kind: static, path: web, route: /}\n  w: {kind: worker, path: api}\n", schemaOnlyAccepts, byDesign},
		agreeCase{"component kind cron with a route", "components:\n  web: {kind: static, path: web, route: /}\n  c: {kind: cron, path: api, route: /c}\n", schemaOnlyAccepts, byDesign},
		agreeCase{"no component at the root", "components:\n  a: {kind: service, path: api, route: /api}\n", schemaOnlyAccepts, crossRef + ": some component must own /"},
		agreeCase{"two components at the root", "components:\n  w: {kind: static, path: web, route: /}\n  a: {kind: service, path: api, route: /}\n", schemaOnlyAccepts, crossRef + ": only one component owns /"},
		agreeCase{"two components on one route", "components:\n  w: {kind: static, path: web, route: /}\n  a: {kind: service, path: api, route: /x}\n  b: {kind: service, path: apps/api, route: /x}\n", schemaOnlyAccepts, crossRef + ": routes are unique"},
		agreeCase{"four components", "components:\n  w: {kind: static, path: web, route: /}\n  a: {kind: service, path: api, route: /a}\n  b: {kind: service, path: api, route: /b}\n  c: {kind: service, path: api, route: /c}\n", schemaOnlyAccepts, limits + ": components_max"},
	)
	// paths
	add(
		ok("component path ../x", "components:\n  web: {kind: static, path: ../x, route: /}\n"),
		ok("component path /etc", "components:\n  web: {kind: static, path: /etc, route: /}\n"),
		ok("component path ..", "components:\n  web: {kind: static, path: .., route: /}\n"),
		ok("component path ..foo", "components:\n  web: {kind: static, path: ..foo, route: /}\n"),
		ok("component path web/.", "components:\n  web: {kind: static, path: web/., route: /}\n"),
		ok("component path web/", "components:\n  web: {kind: static, path: web/, route: /}\n"),
		ok("component path apps/web", "components:\n  web: {kind: static, path: apps/web, route: /}\n"),
		agreeCase{"component path web/../web", "components:\n  web: {kind: static, path: web/../web, route: /}\n", schemaOnlyRefuses, stricter + ": a .. segment that stays inside the tree"},
		agreeCase{"component path missing", "components:\n  web: {kind: static, path: nope, route: /}\n", schemaOnlyAccepts, "needs the tree: the folder does not exist"},
		ok("static.dir ../x", "kind: static\nstatic: {dir: ../x}\n"),
		ok("static.dir /abs", "kind: static\nstatic: {dir: /abs}\n"),
		ok("static.dir dist", "kind: static\nstatic: {dir: dist}\n"),
		agreeCase{"static.dir dist/../out", "kind: static\nstatic: {dir: dist/../out}\n", schemaOnlyRefuses, stricter + ": a .. segment that stays inside the tree"},
		ok("dockerfile.path ../x", svc("  dockerfile: {path: ../x}\n")),
		ok("dockerfile.path /abs", svc("  dockerfile: {path: /abs}\n")),
		agreeCase{"dockerfile", svc("  dockerfile: {}\n"), schemaOnlyAccepts, byDesign},
		agreeCase{"dockerfile with a builder", svc("  builder: jammy-base\n  dockerfile: {}\n"), schemaOnlyAccepts, byDesign},
		agreeCase{"static.build null", "kind: static\nstatic:\n  build: ~\n", schemaOnlyRefuses, yamlLevel + ": null means detect"},
		agreeCase{"static null", "kind: static\nstatic: ~\n", schemaOnlyRefuses, yamlLevel},
	)
	// headers
	add(
		ok("header rule with only a path", hdr("path: /a")),
		ok("header rule with a header", hdr("{path: /a, Cache-Control: no-store}")),
		ok("header rule without a path", hdr("Cache-Control: no-store")),
		ok("header path without a slash", hdr("{path: a, X-A: b}")),
		ok("header path with a control character", hdr("{path: \"/a\\x01\", X-A: b}")),
		ok("header Set-Cookie", hdr("{path: /a, Set-Cookie: x}")),
		ok("header set-cookie", hdr("{path: /a, set-cookie: x}")),
		ok("header SET-COOKIE", hdr("{path: /a, SET-COOKIE: x}")),
		ok("header Content-Length", hdr("{path: /a, Content-Length: '1'}")),
		ok("header content-encoding", hdr("{path: /a, content-encoding: gzip}")),
		ok("header X-Latere-Foo", hdr("{path: /a, X-Latere-Foo: x}")),
		ok("header x-latere-", hdr("{path: /a, x-latere-: x}")),
		ok("header X-Latere without the dash", hdr("{path: /a, X-Latere: x}")),
		ok("header Content-Type", hdr("{path: /a, Content-Type: text/plain}")),
		ok("header name with a space", hdr("{path: /a, 'X A': b}")),
		ok("header name with a colon", hdr("{path: /a, 'X:A': b}")),
		ok("header name with token symbols", hdr("{path: /a, 'X!#$%&*+.^_|~-A': b}")),
		ok("header Path capitalized", hdr("{Path: /a, X-A: b}")),
		ok("header value with a newline", hdr("{path: /a, X-A: \"a\\nb\"}")),
		ok("header value with a tab", hdr("{path: /a, X-A: \"a\\tb\"}")),
		ok("header value with DEL", hdr("{path: /a, X-A: \"a\\x7fb\"}")),
		ok("header value empty", hdr("{path: /a, X-A: ''}")),
		ok("header value accented", hdr("{path: /a, X-A: é}")),
		agreeCase{"header value a number", hdr("{path: /a, Max-Age: 3600}"), schemaOnlyRefuses, yamlLevel + ": a number reads as text"},
		agreeCase{"header value a bool", hdr("{path: /a, X-A: true}"), schemaOnlyRefuses, yamlLevel},
		ok("twenty header rules", "kind: static\nstatic:\n  headers:\n"+rep("    - {path: /a, X-A: b}\n", 20)),
		ok("twenty-one header rules", "kind: static\nstatic:\n  headers:\n"+rep("    - {path: /a, X-A: b}\n", 21)),
	)
	// service
	add(
		ok("public path /a*b", svc("  public_paths: [/a*b]\n")),
		ok("public path /a*", svc("  public_paths: [/a*]\n")),
		ok("public path /*", svc("  public_paths: [/*]\n")),
		ok("public path /a**", svc("  public_paths: [/a**]\n")),
		ok("public path *", svc("  public_paths: ['*']\n")),
		ok("public path a", svc("  public_paths: [a]\n")),
		ok("public path with a space", svc("  public_paths: ['/a b']\n")),
		ok("public path /a//b", svc("  public_paths: [/a//b]\n")),
		ok("public path /a/../b", svc("  public_paths: [/a/../b]\n")),
		ok("public path /webhooks/*", svc("  public_paths: [/webhooks/*]\n")),
		ok("public paths of ten", svc("  public_paths: ["+rep("/a, ", 9)+"/a]\n")),
		ok("public paths of eleven", svc("  public_paths: ["+rep("/a, ", 10)+"/a]\n")),
		ok("health /", svc("  health: /\n")),
		ok("health /healthz", svc("  health: /healthz\n")),
		ok("health //x", svc("  health: //x\n")),
		ok("health x", svc("  health: x\n")),
		ok("health with a space", svc("  health: '/a b'\n")),
		ok("health with a tab", svc("  health: \"/a\\tb\"\n")),
		ok("health with a no-break space", svc("  health: \"/a\\u00a0b\"\n")),
		ok("health with an em space", svc("  health: \"/a\\u2003b\"\n")),
		ok("health with a C1 control", svc("  health: \"/a\\u0085b\"\n")),
		ok("health with a line separator", svc("  health: \"/a\\u2028b\"\n")),
		ok("health accented", svc("  health: /é\n")),
		ok("health with a query", svc("  health: /h?x=1\n")),
		agreeCase{"health empty", svc("  health: ''\n"), schemaOnlyRefuses, stricter + ": empty means /"},
		ok("migrate of 1000 characters", svc("  migrate: "+rep("a", 1000)+"\n")),
		ok("migrate of 1001 characters", svc("  migrate: "+rep("a", 1001)+"\n")),
		ok("migrate of 1000 CJK characters", svc("  migrate: "+rep("迁", 1000)+"\n")),
		ok("migrate of 1001 CJK characters", svc("  migrate: "+rep("迁", 1001)+"\n")),
		ok("migrate of 1000 emoji", svc("  migrate: "+rep("😀", 1000)+"\n")),
		ok("migrate of 1001 emoji", svc("  migrate: "+rep("😀", 1001)+"\n")),
		ok("migrate a list", svc("  migrate: [one, two]\n")),
		ok("port 70000", svc("  port: 70000\n")),
		ok("port 0", svc("  port: 0\n")),
		ok("port 65535", svc("  port: 65535\n")),
		ok("port as a string", svc("  port: \"8080\"\n")),
		ok("port 8080.5", svc("  port: 8080.5\n")),
		agreeCase{"port 8080.0", svc("  port: 8080.0\n"), schemaOnlyAccepts, "JSON Schema calls 8080.0 an integer; Parse wants an integer written as one"},
		agreeCase{"port null", svc("  port: ~\n"), schemaOnlyRefuses, yamlLevel + ": null means the default"},
		agreeCase{"replicas as 1.0", svc("  replicas: {min: 1.0, max: 1}\n"), schemaOnlyAccepts, "JSON Schema calls 1.0 an integer; Parse wants an integer written as one"},
		ok("replicas 1 and 1", svc("  replicas: {min: 1, max: 1}\n")),
		agreeCase{"replicas 2", svc("  replicas: {min: 2, max: 2}\n"), schemaOnlyAccepts, byDesign},
		agreeCase{"replicas 0 and 1", svc("  replicas: {min: 0, max: 1}\n"), schemaOnlyAccepts, byDesign},
		ok("replicas -1", svc("  replicas: {min: -1, max: 1}\n")),
		ok("builder unknown", svc("  builder: nope\n")),
		ok("builder jammy-full", svc("  builder: jammy-full\n")),
	)
	// quantities
	for _, q := range []string{"256Mi", "512Mi", "1Gi", "1e9", "'500000000'", "'1.5e8'", "+256Mi", "0.5G", "300M", "0.3Gi"} {
		add(ok("memory "+q, svc("  resources: {memory: "+q+"}\n")))
	}
	for _, q := range []string{"lots", "-256Mi", "1x", "Gi", "'1 Gi'", "0x10", "1_0Mi", "1mi", "1GiB", "Inf", "NaN", "1e", "'256Mi '", "..5", "1.2.3"} {
		add(ok("memory "+q, svc("  resources: {memory: "+q+"}\n")))
	}
	for _, q := range []string{"250m", "0.5", "1", "1000m", "100m", ".5", "5e-1"} {
		add(ok("cpu "+q, svc("  resources: {cpu: '"+q+"'}\n")))
	}
	for _, q := range []string{"fast", "-1", "1 core", "2c"} {
		add(ok("cpu "+q, svc("  resources: {cpu: '"+q+"'}\n")))
	}
	for _, q := range []string{"1Gi", "10Gi", "5G", "2e9", "1.5Gi"} {
		add(ok("postgres size "+q, "resources:\n  db: {type: postgres, size: "+q+"}\n"))
	}
	for _, q := range []string{"big", "-1Gi", "1 Gi", "1GB"} {
		add(ok("postgres size "+q, "resources:\n  db: {type: postgres, size: '"+q+"'}\n"))
	}
	add(
		agreeCase{"memory empty", svc("  resources: {memory: ''}\n"), schemaOnlyRefuses, stricter + ": empty means the default"},
		agreeCase{"memory above the ceiling", svc("  resources: {memory: 2Gi}\n"), schemaOnlyAccepts, limits + ": memory_max"},
		agreeCase{"memory below the floor", svc("  resources: {memory: 64Mi}\n"), schemaOnlyAccepts, limits + ": a range, not a spelling"},
		agreeCase{"memory too large to read", svc("  resources: {memory: 1e999}\n"), schemaOnlyAccepts, limits + ": a range, not a spelling"},
		agreeCase{"cpu above the ceiling", svc("  resources: {cpu: '2'}\n"), schemaOnlyAccepts, limits + ": cpu_max"},
		agreeCase{"postgres size above the ceiling", "resources:\n  db: {type: postgres, size: 100Gi}\n", schemaOnlyAccepts, limits + ": postgres_size_max"},
		agreeCase{"memory as a number", svc("  resources: {memory: 268435456}\n"), schemaOnlyRefuses, yamlLevel + ": a number reads as text"},
	)
	// resources
	add(
		ok("cache with a size", "resources:\n  c: {type: cache, size: 1Gi}\n"),
		ok("cache with preview", "resources:\n  c: {type: cache, preview: copy}\n"),
		ok("cache with both", "resources:\n  c: {type: cache, size: 1Gi, preview: empty}\n"),
		ok("cache alone", "resources:\n  c: {type: cache}\n"),
		ok("postgres alone", "resources:\n  db: {type: postgres}\n"),
		ok("postgres with preview", "resources:\n  db: {type: postgres, size: 2Gi, preview: empty}\n"),
		ok("postgres with preview nope", "resources:\n  db: {type: postgres, preview: nope}\n"),
		ok("resource type storage", "resources:\n  files: {type: storage}\n"),
		agreeCase{"cache with an empty size", "resources:\n  c: {type: cache, size: ''}\n", schemaOnlyRefuses, stricter + ": an empty size means none"},
		agreeCase{"four resources", "resources:\n  a: {type: cache}\n  b: {type: cache}\n  c: {type: cache}\n  d: {type: cache}\n", schemaOnlyAccepts, limits + ": resources_max"},
	)
	// kind and its blocks
	add(
		ok("kind static with a service block", "kind: static\nservice: {port: 8080}\n"),
		ok("kind service with a static block", "kind: service\nstatic: {dir: dist}\n"),
		ok("component static with a service block", "components:\n  w: {kind: static, path: web, route: /, service: {port: 8080}}\n"),
		ok("component service with a static block", "components:\n  a: {kind: service, path: api, route: /, static: {dir: dist}}\n"),
		ok("kind lambda", "kind: lambda\n"),
		ok("kind worker at the root", "kind: worker\n"),
		ok("unknown key", "kind: static\nstatic:\n  dri: dist\n"),
		ok("empty file", ""),
	)
	// environment variables and references
	for _, v := range []string{
		"plain", "'cost: $5'", "'${ x }'", "'$ {{ x }}'", "'${{app.url}}'", "'${{ app.slug }}'", "'a ${{ app.url }} b ${{ app.slug }}'",
		"'$${{ app.url }}'", "'${${{ app.url }}'", "'${{\tapp.url\t}}'", "3", "true",
		"'${{ secrets.token }}'", "'${{ app.foo }}'", "'${{ app }}'", "'${{ app.url.x }}'", "'${{ components.a.foo }}'",
		"'${{ links.a.slug }}'", "'${{ resources.db.nope }}'", "'${{ resources.db }}'", "'${{ }}'", "'${{ ${{ app.url }}'",
		"'${{ Components.a.url }}'", "'${{ links.A.url }}'", "'${{ links.a--b.url }}'", "'${{ components.a--b.url }}'",
	} {
		add(ok("env "+v, envOf(v)))
	}
	add(
		ok("env reference to a service component", "components:\n  w: {kind: static, path: web, route: /, env: {U: '${{ components.a.url }}'}}\n  a: {kind: service, path: api, route: /api}\n"),
		ok("env reference to a link", "links: [other]\nenv:\n  X: ${{ links.other.url }}\n"),
		ok("env reference to a postgres field", "resources:\n  db: {type: postgres}\nenv:\n  X: ${{ resources.db.password }}\n"),
		ok("env reference to a cache field", "resources:\n  c: {type: cache}\nenv:\n  X: ${{ resources.c.port }}\n"),
		ok("component env reference, unknown", "components:\n  w: {kind: static, path: web, route: /, env: {U: '${{ nope.x }}'}}\n"),
		ok("build.env plain", "build:\n  env:\n    NODE_VERSION: \"22\"\n"),
		ok("build.env a number", "build:\n  env:\n    N: 22\n"),
		ok("build.env with a dollar", "build:\n  env:\n    X: 'cost: $5 ${ x }'\n"),
		ok("build.env reference", "build:\n  env:\n    X: ${{ app.url }}\n"),
		ok("build.env reference with a prefix", "build:\n  env:\n    X: 'a$${{ app.slug }}'\n"),
		ok("build.env unknown reference", "build:\n  env:\n    X: ${{ secrets.x }}\n"),
		ok("build.env unknown key", "build:\n  nope: 1\n"),
		agreeCase{"env value null", "env:\n  A: ~\n", schemaOnlyRefuses, yamlLevel + ": null reads as empty text"},
		agreeCase{"env reference with a stray brace", envOf("'${{ app.url } }'"), schemaOnlyRefuses, stricter + ": text that opens a reference and never closes it is plain text to Resolve"},
		agreeCase{"env unterminated reference", envOf("'${{ app.url'"), schemaOnlyRefuses, stricter + ": text that opens a reference and never closes it is plain text to Resolve"},
		agreeCase{"env reference to a static component", "components:\n  w: {kind: static, path: web, route: /, env: {U: '${{ components.w.url }}'}}\n", schemaOnlyAccepts, crossRef + ": only a service component has a url"},
		agreeCase{"env reference to a link not listed, none", envOf("'${{ links.other.url }}'"), schemaOnlyAccepts, crossRef + ": the link must be listed in links"},
		agreeCase{"env reference to a cache name", "resources:\n  c: {type: cache}\nenv:\n  X: ${{ resources.c.name }}\n", schemaOnlyAccepts, crossRef + ": the fields depend on the resource's type"},
		agreeCase{"env reference to a resource not declared", envOf("'${{ resources.db.url }}'"), schemaOnlyAccepts, crossRef + ": the resource must be declared"},
		agreeCase{"env reference to an unknown component", envOf("'${{ components.zzz.url }}'"), schemaOnlyAccepts, crossRef + ": the component must exist"},
		agreeCase{"env reference to the shorthand app on a static site", "kind: static\nenv:\n  X: ${{ components.app.url }}\n", schemaOnlyAccepts, crossRef + ": only a service component has a url"},
	)
	// the file, below the schema
	add(
		agreeCase{"anchor", "env: &e {A: b}\n", schemaOnlyAccepts, fileLevel + ": anchors are refused"},
		agreeCase{"alias", "env: {A: &a b, B: *a}\n", schemaOnlyAccepts, fileLevel + ": aliases are refused"},
		agreeCase{"tag", "kind: !!str static\n", schemaOnlyAccepts, fileLevel + ": tags are refused"},
		agreeCase{"two documents", "kind: static\n---\nkind: service\n", schemaOnlyAccepts, fileLevel + ": one document only"},
		agreeCase{"a file over 64 KiB", "# " + rep("x", MaxFileBytes) + "\nkind: static\n", schemaOnlyAccepts, fileLevel + ": " + "MaxFileBytes"},
		agreeCase{"more than 1000 keys", manyVars.String(), schemaOnlyAccepts, fileLevel + ": at most 1000 lines with a key"},
		ok("quoted key", "\"kind\": static\n"),
		ok("quoted unknown key", "\"bogus\": 1\n"),
		ok("manifest written as JSON", `{"kind": "static"}`),
		ok("JSON with an unknown key", `{"bogus": 1}`),
		ok("anchored unknown key", "components: &x\n  web: {kind: static, path: web, route: /, bogus: 1}\n"),
		ok("everything at once", "name: shop\nlinks: [api]\nbuild: {env: {A: b}}\nenv: {B: c}\ncomponents:\n  web: {kind: static, path: apps/web, route: /, static: {dir: dist, spa: true, headers: [{path: /a, Cache-Control: x}]}}\n  api: {kind: service, path: apps/api, route: /api, strip_prefix: true, service: {port: 9000, health: /h, resources: {memory: 512Mi, cpu: 500m}, public_paths: [/p/*], migrate: m}}\nresources:\n  db: {type: postgres, size: 2Gi, preview: copy}\n  cache: {type: cache}\n"),
	)

	differ := 0
	for _, c := range cases {
		s, g := verdicts(t, c.src)
		var got string
		switch {
		case (s == nil) == (g == nil):
		case s == nil:
			got = schemaOnlyAccepts
		default:
			got = schemaOnlyRefuses
		}
		if got != "" {
			differ++
		}
		switch {
		case got == c.diff:
		case c.diff == "":
			t.Errorf("%s: %s\n  schema: %v\n  Parse+Resolve: %v", c.name, got, s, g)
		case got == "":
			t.Errorf("%s: the two now agree; it was a known difference (%s)\n  schema: %v\n  Parse+Resolve: %v", c.name, c.why, s, g)
		default:
			t.Errorf("%s: %s, want the known difference %q\n  schema: %v\n  Parse+Resolve: %v", c.name, got, c.diff, s, g)
		}
		if c.diff != "" && c.why == "" {
			t.Errorf("%s: a known difference without its reason", c.name)
		}
	}
	t.Logf("%d cases, %d known differences", len(cases), differ)
}
