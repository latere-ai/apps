# Contributing

Thank you for your interest. This file is for the people and agents who
change this module. If you use it, the [README](README.md) and the
[API reference](https://pkg.go.dev/latere.ai/x/apps) are for you.

## The tree

| Path | What it holds |
|---|---|
| `codes/` | Every error code Latere Apps answers with: its HTTP status, its user sentence and its hint |
| `manifest/` | The grammar of `latere-app.yaml`: `Parse`, `Detect`, `Resolve`, and `schema.json` with `Schema` |
| `manifest/testdata/` | Example manifests the tests accept, and invalid ones they refuse with a named code |
| `authorizer/` | The vocabulary of an authorization endpoint for Apps: the actions, the resource each question carries, and the limits an allow may grant |

## Getting set up

You need Go 1.27 or later and `git`. Nothing else: the tests need no
network, no account and no server.

```sh
git clone https://github.com/latere-ai/apps.git && cd apps
git config core.hooksPath .githooks   # run the checks before each commit and push
go tool lateregate                    # the whole quality bar, the same one CI runs
```

`go tool lateregate list` names the gates, and `go tool lateregate <name>`
runs one of them. The bar covers formatting, lint, the tests with and
without the race detector, a coverage floor of 90% in each package, known
vulnerabilities, and the license notice on every file.

## The rules a change keeps

Clients compile this module in, and the Latere Apps server answers by
it. A client built from an older release must keep working, so:

- **Nothing is renamed or removed.** A code of `codes`, and an action, a
  kind, a resource field or a limits member of `authorizer`, keeps its
  name once a release carries it. A new one is appended, and the frozen
  lists in the `authorizer` tests grow with it.
- **A code comes first.** A new error code is an entry in `codes` before
  anything answers with it. Every refusal of `manifest` is a code of
  `codes`.
- **A manifest key is in three places at once:** the grammar in
  `manifest/manifest.go`, the JSON Schema in `manifest/schema.json`, and
  the platform's manifest reference. The tests hold the first two
  together.
- **Few dependencies.** `codes` imports the standard library only.
  `manifest` adds `github.com/goccy/go-yaml`, whose errors carry the line
  of a node. `authorizer` adds `latere.ai/x/pkg/authz`, the shared
  authorization contract. A client imports none of the server's
  dependencies.
- **No address of ours outside a default or an example.** This
  repository is public.

## Sending a change

1. Fork the repository and make a branch.
2. Make one logical change per commit, and stage files by name.
3. Write the subject as `scope: what changed`, in lowercase, for example
   `manifest: refuse a header path without a leading slash`.
4. Open a pull request.

A bug fix comes with a test that fails without the fix. A change a user
would notice gets a line under `## Unreleased` in `CHANGELOG.md`; an
incompatible one says so there.

Maintainers push to `main` directly, and CI runs the same bar on every
push and pull request. A release is cut with
`go tool lateregate release vX.Y.Z`, which turns `Unreleased` into the
tag's section. If you plan a large change, open an issue first.

## Writing

Every sentence has one reader, and its register follows that reader: a
code's `Message` and `Hint` are for the user, package documentation for
the developer of a client, and comments for whoever changes the code. An
error has one code, one fixed user sentence and one developer detail in
a separate field. The rule and its checklist are in
[`docs/writing/registers.md`](https://github.com/latere-ai/pkg/blob/main/docs/writing/registers.md)
in `latere-ai/pkg`. Use American English, and no em dashes.

## Reporting a vulnerability

Do not open an issue. [`SECURITY.md`](SECURITY.md) says where to send the
report.
