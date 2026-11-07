// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"latere.ai/x/apps/codes"
)

// refusedAt checks err is code with reason, and returns the error.
func refusedAt(t *testing.T, name string, err error, code, reason string) *Error {
	t.Helper()
	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("%s: want %s, got %v", name, code, err)
	}
	if e.Code != code || (reason != "" && e.Details["reason"] != reason) {
		t.Fatalf("%s: want %s/%s, got %v", name, code, reason, e)
	}
	return e
}

// padded is a well-formed manifest of exactly n bytes: a name followed by
// comment padding.
func padded(n int) []byte {
	head := "name: x\n#"
	return []byte(head + strings.Repeat("a", n-len(head)-1) + "\n")
}

func TestParseRefusesFilesOverTheCap(t *testing.T) {
	if _, err := Parse(padded(MaxFileBytes)); err != nil {
		t.Errorf("a well-formed file of exactly %d bytes: %v", MaxFileBytes, err)
	}
	over := padded(MaxFileBytes + 1)
	if len(over) != MaxFileBytes+1 {
		t.Fatalf("input is %d bytes", len(over))
	}
	e := refusedAt(t, "over the cap", func() error { _, err := Parse(over); return err }(), CodeTooLarge, "")
	if e.Details["limit"] != "bytes" || e.Details["value"] != MaxFileBytes+1 || e.Details["max"] != MaxFileBytes {
		t.Errorf("details = %v", e.Details)
	}
}

// The registry's hint names the figure; the constant is the one place it
// is set.
func TestTooLargeHintNamesTheCap(t *testing.T) {
	c, ok := codes.Lookup(CodeTooLarge)
	if !ok {
		t.Fatalf("%s is not in the registry", CodeTooLarge)
	}
	for _, want := range []string{fmt.Sprintf("%d KiB", MaxFileBytes>>10), fmt.Sprintf("%d keys", maxKeyLines)} {
		if !strings.Contains(c.Hint, want) {
			t.Errorf("hint %q does not contain %q", c.Hint, want)
		}
	}
}

func TestParseIgnoresOneLeadingByteOrderMark(t *testing.T) {
	m, err := Parse([]byte("\xef\xbb\xbfkind: static\n"))
	if err != nil {
		t.Fatal(err)
	}
	if m.Kind != KindStatic || m.lines["kind"] != 1 {
		t.Errorf("kind = %q, line %d", m.Kind, m.lines["kind"])
	}
	if _, err := Parse([]byte("\xef\xbb\xbf\xef\xbb\xbfkind: static\n")); err == nil {
		t.Error("a second byte order mark was accepted")
	}
}

func TestParseRefusesDeepNesting(t *testing.T) {
	deep := strings.Repeat("[", 3000) + strings.Repeat("]", 3000)
	tests := []struct{ name, src string }{
		{"bracket bomb of 6 levels", "env:\n  a: [[[[[[x]]]]]]\n"},
		{"flow lists", "env:\n  a: " + strings.Repeat("[", maxDepth+1) + "\n"},
		{"flow maps", "env:\n  a: " + strings.Repeat("{a: ", maxDepth+1) + "\n"},
		{"thousands of lists", "env:\n  a: " + deep + "\n"},
		{"brackets in a quoted value", "env:\n  a: \"" + strings.Repeat("[", maxDepth+1) + "\"\n"},
		{"sequence indicators", "links:\n" + strings.Repeat("- ", maxDepth+1) + "x\n"},
		{"key indicators", strings.Repeat("? ", maxDepth+1) + "x\n"},
		{"indicators beside brackets", "links: " + strings.Repeat("[", maxDepth/2) + "\n" + strings.Repeat("- ", maxDepth/2+1) + "x\n"},
		{"indicators after a bracket", "env: {" + strings.Repeat("? ", maxDepth+1) + "x}\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.src) > MaxFileBytes+1 {
				t.Fatalf("input is %d bytes", len(tc.src))
			}
			_, err := Parse([]byte(tc.src))
			if tc.name == "bracket bomb of 6 levels" {
				// Six levels is within the bound; the grammar refuses the shape.
				refusedAt(t, tc.name, err, CodeInvalid, ReasonType)
				return
			}
			e := refusedAt(t, tc.name, err, CodeInvalid, ReasonDepth)
			if e.Details["line"] == nil {
				t.Errorf("no line: %v", e)
			}
		})
	}
}

func TestParseAcceptsNestingWithinTheBound(t *testing.T) {
	tests := []string{
		"static:\n  headers:\n    - {path: /a, X-A: b}\n",
		"env:\n  a: " + strings.Repeat("[", maxDepth) + strings.Repeat("]", maxDepth) + "\n",
		"links:\n" + strings.Repeat("- ", maxDepth-1) + "x\n",
		"env:\n  a: \"a - b - c - d\"\n  b: x ? y\n",
	}
	for _, src := range tests {
		// Anything but the depth refusal: the grammar may still refuse the shape.
		var e *Error
		if _, err := Parse([]byte(src)); errors.As(err, &e) && e.Details["reason"] == ReasonDepth {
			t.Errorf("%q: %v", src, err)
		}
	}
}

func TestParseRefusesAnchorsAndAliases(t *testing.T) {
	tests := []struct {
		name, src string
		line      int
	}{
		{"anchored mapping hides an unknown key", "components: &x\n  web: {kind: static, path: ., route: /, bogus: 1}\n", 1},
		{"alias of an open map", "env: &e\n  bogus: x\n  dir: d\nstatic: *e\n", 1},
		{"anchor under an open map", "env:\n  a: &v x\n", 2},
		{"alias under a list", "links: [a, &b c, *b]\n", 1},
		{"billion laughs, 6 levels", laughs(6), 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.src) > MaxFileBytes+1 {
				t.Fatalf("input is %d bytes", len(tc.src))
			}
			_, err := Parse([]byte(tc.src))
			e := refusedAt(t, tc.name, err, CodeInvalid, ReasonAnchor)
			if e.Details["line"] != tc.line {
				t.Errorf("line = %v, want %d", e.Details["line"], tc.line)
			}
		})
	}
}

// laughs is the billion laughs: levels of lists, each of ten aliases of
// the level below, with the top aliased under links. Tests use six levels
// at most; the refusal comes before any expansion.
func laughs(levels int) string {
	if levels > 6 {
		panic("laughs: more than 6 levels")
	}
	var b strings.Builder
	b.WriteString("env:\n  x0: &a0 [lol]\n")
	for i := 1; i <= levels; i++ {
		fmt.Fprintf(&b, "  x%d: &a%d [", i, i)
		for j := range 10 {
			if j > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "*a%d", i-1)
		}
		b.WriteString("]\n")
	}
	fmt.Fprintf(&b, "links: *a%d\n", levels)
	return b.String()
}

func TestParseRefusesTags(t *testing.T) {
	tests := []struct {
		name, src string
		line      int
	}{
		{"binary", "env:\n  a: !!binary aGk=\n", 2},
		{"custom tag", "name: !custom x\n", 1},
		{"tag on a mapping", "service: !!map\n  port: 80\n", 1},
		{"tag on a key", "!!str kind: static\n", 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.src))
			e := refusedAt(t, tc.name, err, CodeInvalid, ReasonTag)
			if e.Details["line"] != tc.line {
				t.Errorf("line = %v, want %d", e.Details["line"], tc.line)
			}
		})
	}
}

func TestParseReadsQuotedKeysAndJSON(t *testing.T) {
	tests := []struct{ name, src string }{
		{"double quoted", "\"kind\": static\n"},
		{"single quoted", "'kind': static\n"},
		{"json", `{"kind": "static"}`},
		{"explicit key", "? kind\n: static\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m, err := Parse([]byte(tc.src))
			if err != nil {
				t.Fatal(err)
			}
			if m.Kind != KindStatic || m.lines["kind"] == 0 {
				t.Errorf("kind = %q, lines %v", m.Kind, m.lines)
			}
		})
	}
	m, err := Parse([]byte(`{"service": {"port": 80}, "kind": "service"}`))
	if err != nil {
		t.Fatal(err)
	}
	if m.lines["service.port"] != 1 {
		t.Errorf("lines = %v", m.lines)
	}
	_, err = Parse([]byte("\"service\":\n  \"portt\": 80\n"))
	e := refusedAt(t, "quoted unknown key", err, CodeUnknownKey, "")
	if e.Details["key"] != "service.portt" || e.Details["line"] != 2 {
		t.Errorf("details = %v", e.Details)
	}
}

func TestParseRefusesMoreThanOneDocument(t *testing.T) {
	tests := []struct {
		src  string
		line int
	}{
		{"---\nkind: static\n---\nkind: service\n", 4},
		{"kind: static\n---\nkind: service\n", 3},
		{"components:\n  web: {kind: static, path: ., route: /}\n---\nkind: static\n", 4},
	}
	for _, tc := range tests {
		_, err := Parse([]byte(tc.src))
		e := refusedAt(t, tc.src, err, CodeInvalid, ReasonDocuments)
		if e.Details["line"] != tc.line {
			t.Errorf("%q: line = %v, want %d", tc.src, e.Details["line"], tc.line)
		}
	}
	// An empty second document is not a second manifest.
	if _, err := Parse([]byte("kind: static\n---\n")); err != nil {
		t.Errorf("trailing separator: %v", err)
	}
}

func TestParseReadsIntegersOnly(t *testing.T) {
	bad := []struct {
		src  string
		key  string
		line int
	}{
		{"service:\n  port: 8080.5\n", "service.port", 2},
		{"service:\n  port: \"8080\"\n", "service.port", 2},
		{"service:\n  port: 1e3\n", "service.port", 2},
		{"service:\n  port: true\n", "service.port", 2},
		{"service:\n  port: [1]\n", "service.port", 2},
		{"service:\n  replicas: {min: 1.9, max: 2}\n", "service.replicas.min", 2},
		{"service:\n  replicas:\n    min: 1\n    max: '3'\n", "service.replicas.max", 4},
		{"components:\n  api:\n    service:\n      port: 80.0\n", "components.api.service.port", 4},
	}
	for _, tc := range bad {
		_, err := Parse([]byte(tc.src))
		e := refusedAt(t, tc.src, err, CodeInvalid, ReasonType)
		if e.Details["key"] != tc.key || e.Details["line"] != tc.line {
			t.Errorf("%q: details = %v", tc.src, e.Details)
		}
	}
	m, err := Parse([]byte("service:\n  port: 8080\n  replicas: {min: 1, max: 3}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if *m.Service.Port != 8080 || m.Service.Replicas.Min != 1 || m.Service.Replicas.Max != 3 {
		t.Errorf("service = %+v", m.Service)
	}
	if _, err := Parse([]byte("service:\n  port:\n")); err != nil {
		t.Errorf("an empty port: %v", err)
	}
}

func TestParseRefusesTooManyKeyLines(t *testing.T) {
	env := func(n int) string {
		var b strings.Builder
		b.WriteString("env:\n")
		for i := range n {
			fmt.Fprintf(&b, "  K%d: v\n", i)
		}
		return b.String()
	}
	// The "env:" line is a key line too.
	if _, err := Parse([]byte(env(maxKeyLines - 1))); err != nil {
		t.Errorf("%d key lines: %v", maxKeyLines, err)
	}
	_, err := Parse([]byte(env(maxKeyLines)))
	e := refusedAt(t, "over the key lines", err, CodeTooLarge, "")
	if e.Details["limit"] != "keys" || e.Details["line"] != maxKeyLines+1 {
		t.Errorf("line = %v", e.Details["line"])
	}
	// The last line needs no newline to count.
	src := strings.TrimSuffix(env(maxKeyLines), "\n")
	_, err = Parse([]byte(src))
	refusedAt(t, "no final newline", err, CodeTooLarge, "")
	// Other bounded cost: 64 KiB of short keys is refused, not parsed.
	big := "env:\n" + strings.Repeat("  a: b\n", (MaxFileBytes-5)/7)
	_, err = Parse([]byte(big))
	refusedAt(t, "dense keys", err, CodeTooLarge, "")
}
