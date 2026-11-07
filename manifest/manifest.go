// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
)

// FileName is the manifest's name at the root of an app's tree.
const FileName = "latere-app.yaml"

// MaxFileBytes is the most of latere-app.yaml that Parse reads; a longer
// file is refused as CodeTooLarge before anything parses it. The YAML
// parser's cost grows faster than its input, so the bound is what keeps a
// hostile file cheap.
const MaxFileBytes = 64 << 10

// maxDepth is the deepest nesting Parse accepts, counted as open flow
// brackets plus a run of block indicators. The grammar needs about seven
// levels. The parser's memory is quadratic in flow nesting, so the depth
// is bounded by a scan of the bytes before parsing.
const maxDepth = 32

// maxKeyLines is the most lines holding a key that Parse accepts; more is
// CodeTooLarge. The parser's cost grows with the square of the keys in one
// mapping, so the count is bounded before parsing. Real manifests hold a
// few dozen, and one at the default limits with a hundred variables in
// each env block holds about 500.
const maxKeyLines = 1000

// The component kinds. Worker and cron parse and are refused until they
// are built.
const (
	KindStatic  = "static"
	KindService = "service"
	KindWorker  = "worker"
	KindCron    = "cron"
)

// Manifest is a parsed latere-app.yaml. A field the file leaves out is
// its zero value or nil, and Resolve applies the defaults.
type Manifest struct {
	Name       string                `yaml:"name" json:"name,omitempty"`
	Kind       string                `yaml:"kind" json:"kind,omitempty"`
	Static     *Static               `yaml:"static" json:"static,omitzero"`
	Service    *Service              `yaml:"service" json:"service,omitzero"`
	Components map[string]*Component `yaml:"components" json:"components,omitempty"`
	Links      []string              `yaml:"links" json:"links,omitempty"`
	Build      *Build                `yaml:"build" json:"build,omitzero"`
	Env        map[string]string     `yaml:"env" json:"env,omitempty"`
	Resources  map[string]*Resource  `yaml:"resources" json:"resources,omitempty"`

	// lines is the line of each key the file sets, by its dotted path.
	lines map[string]int
}

// Static is the static block: how a static component is built and served.
type Static struct {
	// Build is the build command; nil means detected, "" no build.
	Build *string `yaml:"build" json:"build,omitzero"`
	// Dir is the directory published, relative to the component's root.
	Dir string `yaml:"dir" json:"dir,omitempty"`
	// SPA serves index.html for unknown paths; nil means true when the
	// output has an index.html.
	SPA *bool `yaml:"spa" json:"spa,omitzero"`
	// Headers are extra response headers by path glob, each rule a path
	// and the headers set on it.
	Headers []map[string]string `yaml:"headers" json:"headers,omitempty"`
	// Sitemap generates /sitemap.xml from the component's HTML paths.
	Sitemap bool `yaml:"sitemap" json:"sitemap,omitzero"`
}

// Service is the service block: how a service component is built and run.
type Service struct {
	Builder              string      `yaml:"builder" json:"builder,omitempty"`
	Start                string      `yaml:"start" json:"start,omitempty"`
	Port                 *int        `yaml:"port" json:"port,omitzero"`
	Health               string      `yaml:"health" json:"health,omitempty"`
	Resources            *Resources  `yaml:"resources" json:"resources,omitzero"`
	Replicas             *Replicas   `yaml:"replicas" json:"replicas,omitzero"`
	PublicPaths          []string    `yaml:"public_paths" json:"public_paths,omitempty"`
	ForwardAuthorization bool        `yaml:"forward_authorization" json:"forward_authorization,omitzero"`
	Dockerfile           *Dockerfile `yaml:"dockerfile" json:"dockerfile,omitzero"`
	Migrate              string      `yaml:"migrate" json:"migrate,omitempty"`
}

// Resources are a service component's memory and CPU, as Kubernetes
// quantities.
type Resources struct {
	Memory string `yaml:"memory" json:"memory,omitempty"`
	CPU    string `yaml:"cpu" json:"cpu,omitempty"`
}

// Replicas are a service component's replica bounds.
type Replicas struct {
	Min int `yaml:"min" json:"min"`
	Max int `yaml:"max" json:"max"`
}

// Dockerfile builds a service from a Dockerfile.
type Dockerfile struct {
	Path   string `yaml:"path" json:"path,omitempty"`
	Target string `yaml:"target" json:"target,omitempty"`
}

// Component is one component of the components form.
type Component struct {
	Kind        string            `yaml:"kind" json:"kind,omitempty"`
	Path        string            `yaml:"path" json:"path,omitempty"`
	Route       string            `yaml:"route" json:"route,omitempty"`
	StripPrefix bool              `yaml:"strip_prefix" json:"strip_prefix,omitzero"`
	Static      *Static           `yaml:"static" json:"static,omitzero"`
	Service     *Service          `yaml:"service" json:"service,omitzero"`
	Env         map[string]string `yaml:"env" json:"env,omitempty"`
	Schedule    string            `yaml:"schedule" json:"schedule,omitempty"`
	Start       string            `yaml:"start" json:"start,omitempty"`
}

// Build is the build block.
type Build struct {
	Env map[string]string `yaml:"env" json:"env,omitempty"`
}

// Resource is one data service.
type Resource struct {
	Type    string `yaml:"type" json:"type,omitempty"`
	Size    string `yaml:"size" json:"size,omitempty"`
	Preview string `yaml:"preview" json:"preview,omitempty"`
}

// grammar is the shape of one value: a mapping of named keys, a mapping
// of any names to one grammar, an open mapping of any keys to scalars, a
// list of one grammar, or a scalar.
type grammar struct {
	keys map[string]*grammar
	each *grammar
	open bool
	list *grammar
	// integer marks a scalar that must be a YAML integer.
	integer bool
}

var (
	scalar  = &grammar{}
	integer = &grammar{integer: true}
	openMap = &grammar{open: true}

	staticGrammar = &grammar{keys: map[string]*grammar{
		"build": scalar, "dir": scalar, "spa": scalar, "sitemap": scalar,
		"headers": {list: openMap},
	}}
	serviceGrammar = &grammar{keys: map[string]*grammar{
		"builder": scalar, "start": scalar, "port": integer, "health": scalar,
		"resources":             {keys: map[string]*grammar{"memory": scalar, "cpu": scalar}},
		"replicas":              {keys: map[string]*grammar{"min": integer, "max": integer}},
		"public_paths":          {list: scalar},
		"forward_authorization": scalar,
		"dockerfile":            {keys: map[string]*grammar{"path": scalar, "target": scalar}},
		"migrate":               scalar,
	}}
	componentGrammar = &grammar{keys: map[string]*grammar{
		"kind": scalar, "path": scalar, "route": scalar, "strip_prefix": scalar,
		"static": staticGrammar, "service": serviceGrammar, "env": openMap,
		"schedule": scalar, "start": scalar,
	}}
	rootGrammar = &grammar{keys: map[string]*grammar{
		"name": scalar, "kind": scalar,
		"static": staticGrammar, "service": serviceGrammar,
		"components": {each: componentGrammar},
		"links":      {list: scalar},
		"build":      {keys: map[string]*grammar{"env": openMap}},
		"env":        openMap,
		"resources": {each: &grammar{keys: map[string]*grammar{
			"type": scalar, "size": scalar, "preview": scalar,
		}}},
	}}
)

// Parse reads a latere-app.yaml. An empty file is the empty manifest. A
// file longer than MaxFileBytes, or with more than maxKeyLines lines
// holding a key, is CodeTooLarge. A leading UTF-8 byte
// order mark is ignored. A key the grammar does not have is
// CodeUnknownKey with its dotted path and line; components beside the
// single-component keys is CodeShapeConflict. These are CodeInvalid, each
// with its reason and line where it has one: a file that is not YAML
// (ReasonSyntax); a value of the wrong type, including a port or replica
// bound that is not an integer (ReasonType); more than one document
// (ReasonDocuments); an anchor or an alias (ReasonAnchor), which could
// hide a key from the check or multiply the file's size; a tag
// (ReasonTag), which changes how a value reads; nesting deeper than
// maxDepth (ReasonDepth). Depth and key lines are counted over the raw
// bytes before parsing, quotes and
// comments ignored, so a file with more than maxDepth open brackets is
// refused even when they sit inside a quoted value.
func Parse(data []byte) (*Manifest, error) {
	if len(data) > MaxFileBytes {
		return nil, fail(CodeTooLarge, "limit", "bytes", "value", len(data), "max", MaxFileBytes)
	}
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	if err := checkShape(data); err != nil {
		return nil, err
	}
	m := &Manifest{lines: map[string]int{}}
	file, err := parser.ParseBytes(data, 0)
	if err != nil {
		return nil, syntaxError(err)
	}
	var body ast.Node
	for _, doc := range file.Docs {
		if doc == nil || doc.Body == nil {
			continue
		}
		if body != nil {
			return nil, fail(CodeInvalid, "reason", ReasonDocuments, "line", lineOf(doc.Body))
		}
		body = doc.Body
	}
	if body == nil {
		return m, nil
	}
	// Before any decoding: the decoder expands aliases and applies tags.
	if err := refuseIndirection(body); err != nil {
		return nil, err
	}
	if err := m.walk(body, rootGrammar, ""); err != nil {
		return nil, err
	}
	if err := yaml.NodeToValue(body, m); err != nil {
		return nil, typeError(err)
	}
	if len(m.Components) > 0 {
		var mixed []string
		for _, k := range []string{"kind", "static", "service"} {
			if _, ok := m.lines[k]; ok {
				mixed = append(mixed, k)
			}
		}
		if len(mixed) > 0 {
			return nil, fail(CodeShapeConflict, "keys", append([]string{"components"}, mixed...))
		}
	}
	return m, nil
}

// checkShape refuses data nested deeper than maxDepth or holding more
// than maxKeyLines lines with a key. It over-counts on purpose. Depth is
// every open bracket minus every close (never below zero), plus the run of
// "- " and "? " indicators that starts at the token being read; block
// nesting by indentation is not counted, MaxFileBytes bounds it. A key
// line is one with a colon followed by a space or the end of the line.
func checkShape(data []byte) error {
	depth, line, keyLines, keyed := 0, 1, 0, false
	tooMany := func() error {
		return fail(CodeTooLarge, "limit", "keys", "max", maxKeyLines, "line", line)
	}
	for i := 0; i < len(data); i++ {
		switch c := data[i]; c {
		case ':':
			keyed = keyed || i+1 == len(data) || strings.ContainsRune(" \t\r\n", rune(data[i+1]))
		case '\n':
			if keyed {
				keyLines++
				keyed = false
				if keyLines > maxKeyLines {
					return tooMany()
				}
			}
			line++
		case '[', '{':
			depth++
			if depth > maxDepth {
				return fail(CodeInvalid, "reason", ReasonDepth, "line", line, "max", maxDepth)
			}
		case ']', '}':
			if depth > 0 {
				depth--
			}
		case '-', '?':
			if i > 0 && !strings.ContainsRune(" \t\r\n[{,", rune(data[i-1])) {
				continue
			}
			run, j := 0, i
			for j < len(data) && (data[j] == '-' || data[j] == '?') && (j+1 == len(data) || strings.ContainsRune(" \t\r\n", rune(data[j+1]))) {
				run++
				j++
				for j < len(data) && (data[j] == ' ' || data[j] == '\t') {
					j++
				}
			}
			if depth+run > maxDepth {
				return fail(CodeInvalid, "reason", ReasonDepth, "line", line, "max", maxDepth)
			}
			if run > 0 {
				i = j - 1
			}
		}
	}
	if keyed && keyLines+1 > maxKeyLines {
		return tooMany()
	}
	return nil
}

// indirection finds the first anchor, alias or tag of a document.
type indirection struct{ err *Error }

func (v *indirection) Visit(n ast.Node) ast.Visitor {
	if v.err != nil || n == nil {
		return nil
	}
	switch n.(type) {
	case *ast.AnchorNode, *ast.AliasNode:
		v.err = fail(CodeInvalid, "reason", ReasonAnchor, "line", lineOf(n))
		return nil
	case *ast.TagNode:
		v.err = fail(CodeInvalid, "reason", ReasonTag, "line", lineOf(n))
		return nil
	}
	return v
}

// refuseIndirection refuses an anchor, an alias or a tag anywhere in node.
func refuseIndirection(node ast.Node) error {
	v := &indirection{}
	ast.Walk(v, node)
	if v.err != nil {
		return v.err
	}
	return nil
}

// lineOf is the line a node starts on, or 0 when it has no token.
func lineOf(n ast.Node) int {
	if tk := n.GetToken(); tk != nil && tk.Position != nil {
		return tk.Position.Line
	}
	return 0
}

// keyName is the key as written without its quotes.
func keyName(k ast.Node) string {
	switch n := k.(type) {
	case *ast.StringNode:
		return n.Value
	case *ast.MappingKeyNode:
		return keyName(n.Value)
	}
	return k.String()
}

// walk checks node against g, recording the line of every key it meets.
func (m *Manifest) walk(node ast.Node, g *grammar, path string) error {
	switch {
	case node == nil, g.integer, g == scalar, g.open:
		return nil
	case g.list != nil:
		seq, ok := node.(*ast.SequenceNode)
		if !ok {
			return nil
		}
		for i, v := range seq.Values {
			if err := m.walk(v, g.list, path+"["+strconv.Itoa(i)+"]"); err != nil {
				return err
			}
		}
		return nil
	}
	for _, mv := range mappingValues(node) {
		key := keyName(mv.Key)
		full := key
		if path != "" {
			full = path + "." + key
		}
		line := lineOf(mv.Key)
		m.lines[full] = line
		next := g.each
		if next == nil {
			var ok bool
			if next, ok = g.keys[key]; !ok {
				return fail(CodeUnknownKey, "key", full, "line", line)
			}
		}
		if next.integer {
			switch mv.Value.(type) {
			case *ast.IntegerNode, *ast.NullNode:
			default:
				return fail(CodeInvalid, "key", full, "reason", ReasonType, "line", line)
			}
		}
		if err := m.walk(mv.Value, next, full); err != nil {
			return err
		}
	}
	return nil
}

func mappingValues(node ast.Node) []*ast.MappingValueNode {
	switch n := node.(type) {
	case *ast.MappingNode:
		return n.Values
	case *ast.MappingValueNode:
		return []*ast.MappingValueNode{n}
	}
	return nil
}

// position is the "[line:column]" prefix the YAML package writes.
var position = regexp.MustCompile(`^\[(\d+):\d+\]\s*`)

// cleanError is the YAML package's first line, without its position
// prefix, and the line it names.
func cleanError(err error) (string, int) {
	msg, _, _ := strings.Cut(err.Error(), "\n")
	line := 0
	if p := position.FindStringSubmatch(msg); p != nil {
		line, _ = strconv.Atoi(p[1])
		msg = msg[len(p[0]):]
	}
	return msg, line
}

func syntaxError(err error) *Error {
	msg, line := cleanError(err)
	e := fail(CodeInvalid, "reason", ReasonSyntax, "error", msg)
	if line > 0 {
		e.Details["line"] = line
	}
	return e
}

func typeError(err error) *Error {
	msg, line := cleanError(err)
	e := fail(CodeInvalid, "reason", ReasonType, "error", msg)
	if line > 0 {
		e.Details["line"] = line
	}
	return e
}
