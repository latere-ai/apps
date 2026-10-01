# Changelog

Every tag has a section here, and the section is the body of the GitHub
release. A tag without one is refused at the pre-push and fails the release
workflow. Write under `Unreleased` as work lands; `lateregate release vX.Y.Z`
turns that into the tag's section, commits, tags and pushes.

A section says what changed for whoever uses the release, not what was
committed: the commit log already holds that.

## Unreleased

- `codes`: the registry of every error code Latere Apps answers with, each
  with its HTTP status, its user sentence and its hint, and `Lookup` to read
  one by name.
- `authorizer`: the vocabulary an authorization endpoint for Apps is written
  against: fifteen actions over four kinds as `authz.Vocabulary`, the
  resource each question carries, and `WireLimits`, the limits an allow may
  grant, with `DecodeLimits`.
- `codes`: `authorizer_unavailable`, the answer when the platform could not
  check a caller's access; `workspace_membership_required` is gone, since
  the platform decides membership; and the sentences of `internal_error`,
  `forbidden`, `app_not_found` and `repository_quota_exceeded` name an
  account or an organization rather than a workspace or a plan.
