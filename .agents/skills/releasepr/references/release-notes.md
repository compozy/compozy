# Commit conventions and custom release notes

## Conventional commits drive the version bump

pr-release computes the next semantic version from commits since the last tag
using `git-cliff` (config: `cliff.toml` in the repo). Use Conventional Commits:

- `fix: ...` → patch bump.
- `feat: ...` → minor bump.
- `feat!: ...` or a `BREAKING CHANGE:` footer / `!` after the type → major bump.
- Other types (`chore:`, `docs:`, `refactor:`, `test:`, `ci:`, `build:`,
  `perf:`, `style:`) are grouped in the changelog per `cliff.toml` and
  generally do not force a higher bump on their own.

If no commit since the last tag warrants a bump, no release PR is produced.
Force a release anyway with `pr-release pr-release --force`.

Never hand-write a commit whose subject starts with `release:` or
`ci(release):` on the default branch — that prefix triggers the production
release job (see `release-workflow.md`).

## Custom release notes with `add-note`

Use `add-note` for human-written highlights that conventional commit subjects
alone do not capture:

```bash
pr-release add-note --title "Shared layout package" --type feature
pr-release add-note --title "Drop Node 16" --type breaking --body "Node 18+ required."
```

- `--type` must be one of `feature`, `fix`, `breaking`, `highlight`
  (case-insensitive; normalized).
- The file is written to `.release-notes/<slug>-<unixtimestamp>.md`, where
  `<slug>` is the lowercased title with non-alphanumerics collapsed to `-`
  (empty slug falls back to `release-note`).
- It has YAML frontmatter `title:` and `type:`. Without `--body`, pr-release
  opens `$EDITOR` on the file; if `$EDITOR` is unset it leaves a placeholder
  body (`<!-- Write your release note here. ... -->`) and prints the path.
- Commit the resulting `.release-notes/*.md` file like normal source.

## How notes flow into a release

1. Active `.release-notes/*.md` files are collected and rendered into the
   release body grouped by type heading, alongside the git-cliff changelog.
2. When the release branch is prepared, active notes are **moved** into
   `.release-notes/archive/vX.Y.Z/` (same filenames). A `.release-notes/.gitkeep`
   is created if missing so the directory stays tracked.
3. The current release section lands in `RELEASE_BODY.md` (GitHub Release body)
   and is prepended to the historical `RELEASE_NOTES.md`.

Archiving is transactional: a move failure rolls back prior moves. Do not
manually move files out of `.release-notes/` mid-release — let the tool archive
them.

## Practical guidance

- Prefer precise conventional commit subjects; they are the source of truth for
  versioning and the bulk of the changelog.
- Add `add-note` entries for cross-cutting or user-facing highlights that
  deserve prose beyond a one-line commit.
- Use `breaking` notes (and `feat!:` / `BREAKING CHANGE:`) deliberately — they
  drive a major bump and headline the release.
