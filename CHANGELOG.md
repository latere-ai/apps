# Changelog

Every tag has a section here, and the section is the body of the GitHub
release. A tag without one is refused at the pre-push and fails the release
workflow. Write under `Unreleased` as work lands; `lateregate release vX.Y.Z`
turns that into the tag's section, commits, tags and pushes.

A section says what changed for whoever uses the release, not what was
committed: the commit log already holds that.

## Unreleased

### Added

- **`manifest_too_large`,** a 400 for a `latere-app.yaml` over 64 KiB
  (`manifest.MaxFileBytes`) or with more than 1000 lines holding a key.
  The details carry `limit` (`bytes` or `keys`) and `max`.
- `manifest_invalid` has four new reasons: `depth` (more than 32 levels
  of brackets or list markers, counted in quoted values too), `anchor` (a
  YAML anchor or alias), `tag` (a YAML tag such as `!!binary`) and
  `documents` (more than one YAML document).

### Changed

- Built with Go 1.27.4, which fixes GO-2026-6859.

- **`manifest.Parse` refuses YAML anchors, aliases, tags and a second
  document.** The grammar uses none of them. A manifest that does is
  refused with `manifest_invalid`; write the values out in one document.
- `service.port`, `service.replicas.min` and `service.replicas.max` must
  be integers. `8080.5`, `"8080"` and `1.9` are refused with reason
  `type`; before, they were cut to an integer.
- A component or resource name may not end with a hyphen or hold two in a
  row, and the app's `name` and every item of `links` must be a slug: 1
  to 40 lowercase letters, digits and single hyphens, with no hyphen at
  either end. A link listed twice is refused.
- A route and a public path use only letters, digits and `. _ ~ / -`, and
  a health check may not start with `//`.
- A header rule's names must be HTTP tokens, and its values may hold no
  control character other than a tab.
- Memory, CPU and database sizes are read by the Kubernetes quantity
  grammar: an exponent such as `1e3` and the suffixes P, E, Pi and Ei now
  read, and hexadecimal numbers and digit underscores do not.
- **`manifest.Resolve` with the zero `Options.Limits` checks against
  `DefaultLimits`.** Before, it refused every manifest. A `Limits` whose
  memory, CPU or database size does not read is now an error from
  `Resolve` rather than no ceiling.
- **The JSON Schema states every rule that needs neither the app's files
  nor the platform's limits,** so an editor refuses what `Resolve` would:
  names and slugs, unique links, a route on each component of the
  components form, header rules, routes, public paths, health checks,
  quantities, paths that leave the tree, references in env values, and a
  cache with a size. `links` no longer has `maxItems`: the count is the
  platform's limit. A value written as `8080.0` still passes the schema
  and is refused by `Parse`, since JSON Schema counts it as an integer.
- The hints of `source_too_many_files`, `detect_failed`,
  `upstream_error`, `upstream_timeout`, `misdirected` and
  `kind_undetectable` name steps that work: no `.latereignore`, no
  `latere app` commands, no Dockerfile, no platform address.

### Fixed

- A quoted key, such as `"kind": static`, and a manifest in JSON form are
  read as keys. Before, they were refused as unknown.
- A leading UTF-8 byte order mark is ignored instead of making the first
  key unknown.
- A directory named `..foo` is inside the tree. A route's refusal names
  its line. `service.migrate` is limited to 1000 characters, not bytes.
- `manifest.Detect` reads a `package.json` cut at 64 KiB up to the cut,
  so the scripts and dependencies before it still count. Detection takes
  time linear in the number of top-level files.

### Security

- **`manifest.Parse` bounds the memory and time a manifest can cost,**
  through the limits on size, keys and nesting and the refusal of
  aliases. Before, a small file could cost far more memory than its size.
  An anchored mapping also hid unknown keys from the check.
- `manifest.Resolve` checks a header rule's names and values in full, so a
  reserved header cannot pass in another form.
- A ceiling in `Options.Limits` that did not read was treated as no
  ceiling, and quantities Kubernetes refuses were accepted.

## v0.0.4 - 2026-11-05

### Changed

- **`manifest` and `authorizer` use `encoding/json/v2`.** Manifests and
  limits encode to the same members as before, under either JSON package, so
  a consumer that writes these types gets the omission rules this package
  documents. The keys of a map, such as a manifest's `env`, are not sorted
  by `encoding/json/v2` unless the caller passes `json.Deterministic(true)`.
  `package.json` files are read as leniently as before: duplicate keys,
  invalid UTF-8 and any letter case in names are accepted.
- Requires `latere.ai/x/pkg` v0.113.0.

## v0.0.3 - 2026-11-01

### Added

- **`app_quota_exceeded`,** a 429 for an account at its number of apps. The
  details carry `limit`, `value` and `max`.

### Changed

- The refusals of too many components, links or data services no longer
  mention a plan or suggest moving to one. No plan changes these limits.

### Security

- **Built and tested with Go 1.27.3,** which fixes GO-2026-6611,
  GO-2026-6612, GO-2026-6613, GO-2026-6617 and GO-2026-6809. A program that
  imports this module builds with its own Go, so build it with 1.27.3 or
  later.

## v0.0.2 - 2026-10-04

- `codes` and `manifest`: the manifest's reference and its schema are named
  at the addresses the docs serve them from, under `/docs/apps/`. The hint
  of `manifest_unknown_key` pointed at `platform.latere.ai/docs/app/manifest`
  and the schema's `$id` at `/docs/app/latere-app.schema.json`; neither
  address answers. They are `platform.latere.ai/docs/apps/manifest` and
  `https://platform.latere.ai/docs/apps/latere-app.schema.json`.

- `codes`: `repository_name_taken`, 409. An app's repository lives under
  its owner's name, beside the owner's other repositories, so an app whose
  slug names a repository the owner already has is refused with it rather
  than given another name.

- `codes`: `build_timeout` says the build ran longer than its time limit.
  It named 15 minutes, which no limit of the build is; the limits are the
  platform's and the details name the one a build reached. No code, status
  or hint changes.

- `codes`: the hints of the codes an app's visitor and the API answer today
  name what works now (push a commit, push a `v` tag, the lists of deploys
  and releases) instead of `latere app` commands that do not exist yet.
  `unknown_application`, `no_deploy`, `no_preview`, `id_too_short`,
  `app_not_found` and `release_not_found` change; no code, status or
  message changes.

- `latere.ai/x/pkg` v0.90.2, the release past GO-2026-6615 and GO-2026-6505.
  No OpenTelemetry module is in this module's build.

- `manifest`: the grammar of `latere-app.yaml`. `Parse` refuses a key the
  platform does not know with its line, `Detect` tells a static site from a
  service by the files of a tree, `Resolve` applies the defaults and every
  rule (components and routes, ranges against the platform's limits,
  references, links, data services) and answers with the code and key a
  manifest is refused with, and `Schema` is the JSON Schema of the file.
- `codes`: the codes a manifest, a deploy's limits and a release are
  refused with, from `manifest_unknown_key` to `release_not_found`, and
  `ref_not_found` for a branch or commit the app repository does not have.
- `codes`: the codes of a build (`source_fetch_failed`, `install_failed`,
  `build_failed`, `static_dir_missing`, `build_timeout` and the rest), of an
  app's address at the edge (`unknown_application`, `no_deploy`,
  `no_preview`, `starting`, `rate_limited` and the rest), `private`, and
  `capacity_exhausted`, each with its status, sentence and hint.

## v0.0.1 - 2026-10-01

- `codes`: the registry of every error code Latere Apps answers with, each
  with its HTTP status, its user sentence and its hint, and `Lookup` to read
  one by name. `authorizer_unavailable` is the answer when the platform could
  not check a caller's access.
- `authorizer`: the vocabulary an authorization endpoint for Apps is written
  against: fifteen actions over four kinds as `authz.Vocabulary`, the
  resource each question carries, and `WireLimits`, the limits an allow may
  grant, with `DecodeLimits`. Every action within one app carries the app's
  id, so a grant naming one app covers that app and no other.
