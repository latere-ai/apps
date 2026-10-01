// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"testing"

	"latere.ai/x/apps/codes"
)

// shop is a tree with a static web component and a service api.
var shop = tree{
	"apps/web/index.html":   "<html></html>",
	"apps/api/go.mod":       "module api",
	"apps/api/main.go":      "package main",
	"apps/worker/worker.js": "",
}

const twoComponents = `name: my-app
components:
  web:
    kind: static
    path: apps/web
    route: /
  api:
    kind: service
    path: apps/api
    route: /api
    env:
      LOG_LEVEL: info
env:
  NODE_ENV: production
  API_URL: ${{ components.api.url }}
  SELF: ${{ app.url }}
`

// TestComponentsValidation: two roots, an overlapping route, a missing
// path, a worker, and the shorthand keys beside components: are each
// refused with the documented code; a valid web and api manifest resolves
// to two components and accepts a reference to the api's URL.
func TestComponentsValidation(t *testing.T) {
	r, err := resolve(twoComponents, shop)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Components) != 2 || r.Components[0].Name != "api" || r.Components[1].Name != "web" ||
		r.Components[0].Kind != KindService || r.Components[0].Route != "/api" || r.Components[1].Path != "apps/web" ||
		r.Components[0].Env["LOG_LEVEL"] != "info" {
		t.Fatalf("components = %+v", r.Components)
	}

	for _, tc := range []struct {
		name, src, code string
	}{
		{"two roots", "components:\n  a: {kind: static, path: apps/web, route: /}\n  b: {kind: static, path: apps/web, route: /}\n", CodeRootConflict},
		{"no root", "components:\n  a: {kind: static, path: apps/web, route: /a}\n", CodeRootMissing},
		{"overlapping route", "components:\n  a: {kind: static, path: apps/web, route: /}\n  b: {kind: service, path: apps/api, route: /x}\n  c: {kind: service, path: apps/api, route: /x}\n", CodeRouteConflict},
		{"missing path", "components:\n  a: {kind: static, path: apps/nope, route: /}\n", CodePathMissing},
		{"path escapes", "components:\n  a: {kind: static, path: ../x, route: /}\n", CodeInvalid},
		{"a worker", "components:\n  web: {kind: static, path: apps/web, route: /}\n  w: {kind: worker, path: apps/worker, start: node worker.js}\n", CodeKindUnsupported},
		{"a cron", "components:\n  web: {kind: static, path: apps/web, route: /}\n  n: {kind: cron, path: apps/api, schedule: \"0 3 * * *\"}\n", CodeKindUnsupported},
		{"shorthand beside", "kind: static\ncomponents:\n  a: {kind: static, path: apps/web, route: /}\n", CodeShapeConflict},
		{"bad name", "components:\n  Web: {kind: static, path: apps/web, route: /}\n", CodeInvalid},
		{"double hyphen", "components:\n  a--b: {kind: static, path: apps/web, route: /}\n", CodeInvalid},
		{"route not clean", "components:\n  a: {kind: static, path: apps/web, route: /a/}\n", CodeInvalid},
		{"route missing", "components:\n  a: {kind: static, path: apps/web}\n", CodeInvalid},
		{"too many", "components:\n  a: {kind: static, path: apps/web, route: /}\n  b: {kind: static, path: apps/web, route: /b}\n  c: {kind: static, path: apps/web, route: /c}\n  d: {kind: static, path: apps/web, route: /d}\n", CodeComponentQuota},
		{"reference to a static component", "components:\n  web: {kind: static, path: apps/web, route: /}\nenv:\n  X: ${{ components.web.url }}\n", CodeUnknownReference},
		{"reference to nothing", "env:\n  X: ${{ secrets.token }}\n", CodeUnknownReference},
		{"reference in build env", "build:\n  env:\n    X: ${{ app.url }}\n", CodeUnknownReference},
		{"unknown link reference", "env:\n  X: ${{ links.api.url }}\n", CodeUnknownReference},
		{"unknown resource field", "resources:\n  c: {type: cache}\nenv:\n  X: ${{ resources.c.password }}\n", CodeUnknownReference},
		{"undetectable component", "components:\n  a: {path: apps/worker, route: /}\n", CodeKindUndetectable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tr := Tree(shop)
			if tc.name == "reference to nothing" || tc.name == "reference in build env" || tc.name == "unknown link reference" || tc.name == "unknown resource field" {
				tr = site
			}
			_, err := resolve(tc.src, tr)
			refused(t, err, tc.code, "")
		})
	}

	// Every code a manifest is refused with is an entry of the registry,
	// so the API and the CLI render the same sentence for it.
	for _, code := range []string{CodeUnknownKey, CodeShapeConflict, CodeInvalid, CodeUnknownReference, CodeKindUnsupported,
		CodePathMissing, CodeRouteConflict, CodeRootMissing, CodeRootConflict, CodeLinkSelf, CodeKindUndetectable,
		CodeDockerfile, CodeReplicasUnsupported, CodeComponentQuota, CodeLinkQuota, CodeResourceQuota} {
		if _, ok := codes.Lookup(code); !ok {
			t.Errorf("%s is not in the registry", code)
		}
	}
}

// TestReferences: a reference to the app, a sibling service, a link, or a
// resource field resolves; Expand fills the ones it is given and leaves
// the rest as written.
func TestReferences(t *testing.T) {
	src := "links: [api]\nresources:\n  db: {type: postgres}\nenv:\n  A: ${{ app.slug }}\n  B: ${{ links.api.url }}\n  C: ${{ resources.db.url }}\n"
	if _, err := resolve(src, site); err != nil {
		t.Fatal(err)
	}
	got := Expand("https://${{ app.slug }}.x and ${{ links.api.url }}", map[string]string{"app.slug": "my-app"})
	if got != "https://my-app.x and ${{ links.api.url }}" {
		t.Errorf("Expand = %q", got)
	}
}
