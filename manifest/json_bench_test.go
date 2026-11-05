// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"fmt"
	"strings"
	"testing"
)

// benchPackageJSON builds a package.json of about n bytes: a build script, a
// vite devDependency and as many pinned dependencies as it takes to reach n.
func benchPackageJSON(n int) string {
	var b strings.Builder
	b.WriteString(`{"name":"storefront","version":"1.4.2","private":true,"type":"module",` +
		`"scripts":{"dev":"vite","build":"tsc -b && vite build","preview":"vite preview","test":"vitest run"},` +
		`"devDependencies":{"vite":"^6.2.0","typescript":"~5.7.2","vitest":"^3.0.5"},"dependencies":{`)
	for i := 0; b.Len() < n-8; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `"@scope/package-%04d":"^%d.%d.%d"`, i, i%9, i%17, i%23)
	}
	b.WriteString("}}")
	return b.String()
}

// BenchmarkJSONDetectPackageJSONUnmarshal stands for component detection on a
// repository whose component root holds a package.json: Detect reads the file
// (capped at MaxDetectRead) and decodes its scripts and dependencies.
func BenchmarkJSONDetectPackageJSONUnmarshal(b *testing.B) {
	for _, size := range []struct {
		name  string
		bytes int
	}{
		{"small", 512},
		{"medium", 10 << 10},
		{"large", 60 << 10},
	} {
		pkg := benchPackageJSON(size.bytes)
		tr := tree{
			"package.json":   pkg,
			"vite.config.ts": "export default {}",
			"bun.lock":       "",
			"src/main.ts":    "console.log(1)",
		}
		det, err := Detect(tr, ".")
		if err != nil || det == nil || det.Kind != KindStatic {
			b.Fatalf("setup: Detect = %+v, %v; want a static component", det, err)
		}
		b.Run(size.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(pkg)))
			for b.Loop() {
				if _, err := Detect(tr, "."); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
