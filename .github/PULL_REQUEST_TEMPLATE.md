## What this changes

<!-- What is different after this change for a client that imports the module: a code, a manifest key, an action of the authorizer vocabulary. -->

## Why

<!-- The problem this solves. Link the issue if there is one. -->

## Checklist

- [ ] `go tool lateregate` passes on your machine.
- [ ] A bug fix includes a test that fails without the fix.
- [ ] No code, action, kind, resource field or limits member is renamed
      or removed; a new one is appended.
- [ ] A new manifest key is in the grammar, in `manifest/schema.json` and
      in their tests together.
- [ ] Every caller of an exported identifier still compiles, or
      `CHANGELOG.md` names the incompatible change under `Unreleased`.
