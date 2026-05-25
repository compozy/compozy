# Release lifecycle and CI behavior

## Contents

- The three stages
- What triggers the release-PR job
- What the release-PR job produces
- What triggers the dry-run job
- pr-release does not tag or publish
- What triggers the production release
- Branch and PR naming
- RELEASE_BODY.md vs RELEASE_NOTES.md
- Mental model for debugging "why no release?"

## The three stages

1. **Release PR** — on normal pushes to the default branch, pr-release computes
   the next version and opens/updates a `release/vX.Y.Z` branch + pull request
   containing the version bumps, changelog, and release body.
2. **Dry-run validation** — when that release PR is opened/updated, a CI job
   runs `pr-release dry-run` to validate the release end-to-end without
   publishing.
3. **Production release** — when the release PR merges, the merge commit (whose
   subject starts with `release:` or `ci(release):`) triggers tagging and a
   GoReleaser publish.

## What triggers the release-PR job

On push to the default branch, the job runs **unless** the head commit subject
starts with `release:`, `ci(release):`, or `Merge pull request`, or the commit
author is `github-actions[bot]`. It also runs on manual dispatch with mode
`release-pr`. This is why ordinary `feat:` / `fix:` commits produce a release PR
but the release commit itself does not loop.

## What the release-PR job produces

After `pr-release pr-release --enable-rollback --ci-output`, CI inspects the
current branch. If it matches `^release/v[0-9]+\.[0-9]+\.[0-9]+`, a release PR
exists and follow-up checks (CI + a dispatched dry-run) are triggered.
If no release branch was produced (no conventional changes since the last tag),
the job reports "No release PR branch produced" and stops — this is expected,
not a failure. Force one with `--force` (or the workflow's `force_release`
dispatch input).

## What triggers the dry-run job

The dry-run job runs when a pull request whose title starts with
`release: Release ` or `ci(release): Release ` is opened/synchronized/reopened
against the default branch, or on manual dispatch with mode `dry-run` (passing
`head_ref` and `pr_number`).

## pr-release does not tag or publish

pr-release's responsibility ends at the opened/updated release PR. It does
**not** create git tags and does not publish releases. Tagging and publishing
are the consuming repo's own release job (below). Do not expect a tag to appear
just from running `pr-release pr-release`.

## What triggers the production release

A push to the default branch whose head commit subject starts with `release:`
or `ci(release):` — i.e. the release PR being merged. The consumer's release
job (not pr-release) then:

1. Derives the version with `git cliff --bumped-version` (fallback: parse it
   from the commit subject `Release vX.Y.Z`).
2. Creates and pushes an annotated tag `vX.Y.Z`.
3. Runs GoReleaser with
   `--release-notes=RELEASE_BODY.md`,
   `--release-header-tmpl=.goreleaser.release-header.md.tmpl`,
   `--release-footer-tmpl=.goreleaser.release-footer.md.tmpl`.

Do not hand-author `release:` commits on the default branch — that is the
trigger that publishes a production release.

## Branch and PR naming

- Release branch: `release/vMAJOR.MINOR.PATCH`.
- Release PR title: `release: Release vX.Y.Z` (or `ci(release): Release vX.Y.Z`).
- These exact prefixes are matched by the CI `if:` conditions; renaming them
  breaks the dry-run and production-release triggers.

## RELEASE_BODY.md vs RELEASE_NOTES.md

- `RELEASE_BODY.md` — only the **current** release section; consumed by
  GoReleaser for the GitHub Release body.
- `RELEASE_NOTES.md` — the committed **historical** document; the current
  release is prepended while older sections are preserved.
- `.release-notes/` holds active custom notes (from `add-note`) folded into the
  body, then archived to `.release-notes/archive/vX.Y.Z/` once the release
  branch is prepared. A `.release-notes/.gitkeep` keeps the directory tracked.

## Mental model for debugging "why no release?"

Check in this order:
1. Were the commits conventional (`feat:`/`fix:`/etc.)? Non-conventional
   commits yield no version bump → no release PR. (`release-notes.md`)
2. Was the push commit one of the skipped kinds (bot / `release:` /
   `ci(release):` / `Merge pull request`)? Then the release-PR job did not run
   by design.
3. Did owner/repo and token resolve? (`configuration.md`,
   `troubleshooting.md`)
4. Did the release PR title keep the required prefix so the dry-run job fired?
5. Wrong/initial version? The checkout was likely shallow — require
   `fetch-depth: 0` + `fetch-tags: true`. For a tagless first release, set
   `INITIAL_VERSION`.
