# CLI command reference

The CLI is Cobra-based. Run `pr-release <command> --help` for built-in help.
Six commands exist: `pr-release`, `plan`, `release-body`, `dry-run`, `add-note`, `version`.

## `pr-release` — create or update the release PR

Orchestrates the full release-PR workflow: checks for changes since the last
release, calculates the next version, creates/updates the `release/vX.Y.Z`
branch, bumps package versions (when a tools workspace is present), generates
the changelog and release body, and creates or updates the pull request.

| Flag                  | Type   | Default | Behavior |
| --------------------- | ------ | ------- | -------- |
| `--force`             | bool   | false   | Proceed/refresh even if no releasable changes are detected. Idempotent — a no-op when nothing changed. Every observed consumer passes it in CI for deterministic PR creation. |
| `--dry-run`           | bool   | false   | Run all steps without pushing or opening/updating the PR. |
| `--ci-output`         | bool   | false   | Emit CI-friendly output. Use in GitHub Actions. |
| `--skip-pr`           | bool   | false   | Run steps but skip PR creation (for local testing). |
| `--enable-rollback`   | bool   | false   | On any step failure, automatically roll back to the prior repo state (saga compensation). Standard for CI. |
| `--rollback`          | bool   | false   | Roll back a previously failed release session instead of running a release. |
| `--session-id`        | string | (none)  | Session ID to roll back; with `--rollback`, uses the latest session if omitted. |

Standard CI invocation (the de-facto convention across all observed consumers):
`pr-release pr-release --force --enable-rollback --ci-output`. `--force` here is
not "force a release with no changes" — it makes the job idempotent so re-runs
deterministically refresh the release PR and it no-ops when nothing changed.

## `plan` — prove explicit release identity

Validates an operator-supplied release ref, version, and channel without
mutating the repository or publishing. It resolves the ref and requires it to
equal the checked-out `HEAD`, derives the Git tag exactly once, and rejects the
plan if that tag already exists locally or on `origin`.

| Flag        | Type   | Default | Behavior |
| ----------- | ------ | ------- | -------- |
| `--ref`     | string | required | Git ref or revision that must resolve to `HEAD`. |
| `--version` | string | required | Strict, unprefixed SemVer such as `0.3.0-beta.1`; a leading `v` is rejected. |
| `--channel` | string | required | One of `beta`, `stable`, or `legacy`. |
| `--format`  | string | `json`  | `json` emits the plan object; `github` emits ordered `key=value` lines for `$GITHUB_OUTPUT`. |

The plan also emits `previous_tag`, `git_range`, and `initial_release`.
`git_range` is
`<previous_tag>..<release_commit>` and is empty only when no strict semantic-
version predecessor is reachable on the first-parent release line. In that
case `initial_release` is true. Beta plans include prerelease tags while
choosing the predecessor; stable and legacy plans exclude prerelease tags.

The channel policy is closed:

| Channel  | Version shape | GitHub | npm tag | Homebrew |
| -------- | ------------- | ------ | ------- | -------- |
| `beta`   | `-beta...` prerelease | prerelease, not latest | `beta` | skip upload |
| `stable` | no prerelease | stable, latest | `latest` | publish |
| `legacy` | no prerelease | stable, latest | `latest` | publish |

`legacy` identifies a downstream maintenance profile or ref; pr-release does
not hard-code a legacy branch name. The consumer must use the emitted values
for tag creation and publishing without re-parsing the version or inferring a
channel.

```bash
pr-release plan --ref main --version 0.3.0-beta.1 --channel beta
pr-release plan --ref legacy/v0.2 --version 0.2.16 --channel legacy --format github
```

## `release-body` — render one explicit release range

Renders the Markdown body for an explicit release without writing files. Pass
the outputs from `plan` unchanged:

| Flag      | Type   | Default | Behavior |
| --------- | ------ | ------- | -------- |
| `--tag`   | string | required | Target semantic-version Git tag. |
| `--range` | string | empty | Exact `<previous>..<commit>` range emitted by a non-initial plan. |
| `--initial` | bool | false | Explicitly render the first release when `release_initial` is true. Mutually exclusive with `--range`. |

```bash
pr-release release-body \
  --tag "$RELEASE_TAG" \
  --range "$RELEASE_GIT_RANGE" > release-body.md
```

Stdout contains Markdown only. The command passes the range directly to
`git-cliff` and includes only custom `.release-notes` files introduced within
that range for the target core version. A command with neither selector fails
closed; do not treat an absent workflow output as a first release. Diagnostics
go to stderr.

## `dry-run` — validate the release PR

Runs the dry-run orchestrator (always internally `DryRun=true`): performs the
validation steps a release PR must pass, without pushing or opening anything.

| Flag          | Type | Default | Behavior |
| ------------- | ---- | ------- | -------- |
| `--ci-output` | bool | false   | Emit CI-friendly output. |

This is the command the dry-run CI job runs against an open release PR. It
reads `GITHUB_ISSUE_NUMBER` in CI when a result comment should target a pull
request. It does not infer a version or channel; an explicit-release workflow
must run `plan` as its identity gate.

`pr-release --dry-run` and the `dry-run` command are not identical:
`pr-release --dry-run` exercises the release-PR orchestrator in no-write mode;
`dry-run` runs the dedicated PR-validation orchestrator.

## `add-note` — create a custom release note

Writes a markdown file to `.release-notes/` that is folded into the release
body and later archived. See `release-notes.md` for the full lifecycle.

| Flag      | Required | Behavior |
| --------- | -------- | -------- |
| `--title` | yes      | Note title; also slugified into the filename. |
| `--type`  | yes      | One of `feature`, `fix`, `breaking`, `highlight` (case-insensitive). |
| `--body`  | no       | Inline markdown body. If omitted, opens `$EDITOR` on the new file; if `$EDITOR` is unset, the file is created with a placeholder and the path is printed. |

The created file is `.release-notes/<slug>-<unixtime>.md` with YAML
frontmatter (`title`, `type`). The command prints `Created <path>`.

Example:

```bash
pr-release add-note --title "Shared layout package" --type feature
pr-release add-note --title "Drop Node 16" --type breaking --body "Node 18+ now required."
```

## `version` — print build metadata

Prints three lines: `Version`, `Commit`, `Built`. Non-release builds fall back
to `dev` / `unknown`. Use as an install smoke test; takes no flags.

## Notes on flag combinations

- `--rollback` is mutually meaningful only with a prior failed session; pair
  with `--session-id` to target a specific one.
- `--skip-pr` and `--dry-run` are for local experimentation; CI uses neither.
- `--ci-output` only changes output formatting; it does not imply `--dry-run`.
