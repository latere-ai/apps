# Changelog

Every tag has a section here, and the section is the body of the GitHub
release. A tag without one is refused at the pre-push and fails the release
workflow. Write under `Unreleased` as work lands; `lateregate release vX.Y.Z`
turns that into the tag's section, commits, tags and pushes.

A section says what changed for whoever uses the release, not what was
committed: the commit log already holds that.

## Unreleased

- `manifest`: the grammar of `latere-app.yaml`. `Parse` refuses a key the
  platform does not know with its line, `Detect` tells a static site from a
  service by the files of a tree, `Resolve` applies the defaults and every
  rule (components and routes, ranges against the platform's limits,
  references, links, data services) and answers with the code and key a
  manifest is refused with, and `Schema` is the JSON Schema of the file.
- `codes`: the codes a manifest, a deploy's limits and a release are
  refused with, from `manifest_unknown_key` to `release_not_found`.
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
