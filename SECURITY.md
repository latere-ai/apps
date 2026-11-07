# Security

## Reporting a vulnerability

Report one privately, through
[GitHub's private vulnerability reporting](https://github.com/latere-ai/apps/security/advisories/new)
or by email to security@latere.ai. Do not open a public issue for it. The
[organization's security policy](https://github.com/latere-ai/.github/blob/main/SECURITY.md)
sets the response times, how disclosure works, and the safe harbor for
research done in good faith.

The module is before v1.0.0, so there is one release line: a fix ships
in the next release, and the [changelog](CHANGELOG.md) says which release
has it.

## What is protected

The module does no input or output of its own: a program hands it a
manifest's bytes, a tree of files, or a decision's limits, and gets back a
value or an error. A manifest and the files beside it come from whoever
pushed the app, so the server reads them as untrusted. Against that input,
the module commits to the following, and the named tests check each
property.

| Property | Tests |
|---|---|
| The module's own code imports no package that opens a file, a socket or a process (`os`, `os/exec`, `net`, `net/http`, `syscall`). | `manifest.TestNoPackageImportsIO` |
| A small manifest cannot cost a large amount of memory or time. `Parse` refuses a file over 64 KiB (`manifest.MaxFileBytes`) or with more than 1000 lines holding a key, and more than 32 levels of brackets or list markers, before the YAML parser runs. It refuses YAML anchors, aliases and tags, and more than one document, before any value is decoded, so no alias is ever expanded. | `manifest.TestParseRefusesFilesOverTheCap`, `manifest.TestParseRefusesTooManyKeyLines`, `manifest.TestParseRefusesDeepNesting`, `manifest.TestParseRefusesAnchorsAndAliases`, `manifest.TestParseRefusesTags`, `manifest.TestParseRefusesMoreThanOneDocument` |
| A key outside the grammar is refused with its dotted path and line, also when it is quoted, written in the JSON form, or under an anchor. | `manifest.TestManifestValidation`, `manifest.TestUnknownKeyNamesItsLine`, `manifest.TestParseReadsQuotedKeysAndJSON`, `manifest.TestParseRefusesAnchorsAndAliases` |
| Every path a manifest names stays inside the app's tree: a component's `path`, `static.dir` and `service.dockerfile.path` that is absolute or leaves the tree once cleaned is refused. | `manifest.TestManifestValidation`, `manifest.TestComponentsValidation`, `manifest.TestResolveChecksTheDockerfilePathEscape`, `manifest.TestResolveTakesADirectoryNamedDotDotFoo` |
| A header rule cannot set `Content-Length`, `Content-Encoding`, `Set-Cookie` or an `X-Latere-` header, in any letter case or with whitespace around the name. Every header name is an HTTP token, and no value holds a control character other than a tab, so a value cannot add a header line. | `manifest.TestManifestValidation`, `manifest.TestResolveRefusesHeaderNamesThatAreNotTokens` |
| A route and a public path hold only letters, digits and `. _ ~ / -`, and a route is already in its clean form. A health check is a path on the service itself: it cannot start with `//`, which names another host. | `manifest.TestResolveRefusesRoutesOutsideTheCharacterSet`, `manifest.TestResolveChecksPublicPathsAndHealth` |
| Names become DNS labels: a component or resource name, the app's `name` and every link are refused unless they are lowercase letters, digits and single hyphens, with no hyphen at either end. | `manifest.TestResolveChecksNamesAndLinks` |
| A memory, CPU or database size reads by the grammar Kubernetes uses, so `Resolve` passes no quantity the cluster refuses. A ceiling in `Options.Limits` that does not read is an error, never no ceiling. | `manifest.TestQuantityReadsTheKubernetesGrammar`, `manifest.TestResolveRefusesQuantitiesKubernetesRefuses`, `manifest.TestResolveRefusesLimitsThatDoNotRead` |
| A reference in an env value names only the app, a service component, a link, or a field of a declared data service, and `build.env` holds none, so a build never receives a value the platform provides at run time. | `manifest.TestComponentsValidation`, `manifest.TestReferences` |
| Every action inside one app carries the app's id, so a grant over one app covers that app only, and `app.list` and `app.create` need a grant over the kind. The actions, the kinds and the limits members are frozen. | `authorizer.TestAppActionsCarryTheAppID`, `authorizer.TestTheTableIsFrozen`, `authorizer.TestTheMembersAreFrozen` |
| `authorizer.DecodeLimits` refuses a limits object that does not parse, a count below zero, and an empty quantity, so a caller holds no ceiling it cannot read. | `authorizer.TestUnreadableLimits` |

Each test is named as `package.Test`.

## What is not defended

- **The tree a caller passes.** `Detect` and `Resolve` see only the names
  a `Tree` lists and the bytes it returns. A `Tree` that follows symbolic
  links or reads outside the app's directory is the caller's to bound.
  The module ships no `Tree`.
- **The bytes before `Parse`.** `Parse` refuses a long file only once it
  is in memory. A caller that reads `latere-app.yaml` from a repository
  caps its own read.
- **The limits a caller passes.** `Resolve` checks a manifest against the
  `Options.Limits` it is given. A client that passes larger figures than
  the platform's accepts what the server refuses.
- **Everything the server checks after `Resolve`.** The server refuses
  more, such as names `build.env` may not use. A manifest `Resolve`
  accepts can still be refused.
- **Detection is a guess.** `Detect` reads file names and a few markers,
  such as `output: 'export'` in a `next.config`, even inside a comment. It
  chooses how to build an app, not whether to trust it.
- **What serves the headers.** `Resolve` checks the form of a header rule.
  What the server that answers the app's visitors does with it is outside
  this module.
- **`codes.Codes` is a map.** Any package in a program can change an
  entry, and `codes.Lookup` returns the change.
- **Quantity strings in a limits object.** `DecodeLimits` checks that a
  quantity is not empty, not that it is a Kubernetes quantity.

## Audit

This module has had no external security audit.

## Supply chain

The module ships as source: there are no binaries and no images. A
release is a version tag on `main` with release notes. The Go tools check
each module download against the
[checksum database](https://sum.golang.org), so a tag cannot change after
it is published. To check what you have:

```sh
go mod verify
govulncheck ./...
```

`govulncheck` is in the quality bar that runs on every push, over the
module and its dependencies.
