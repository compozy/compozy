Goal (incl. success criteria):

- Implement the accepted release-notes contract fix across `pr-release` and `looper`, then restore the `pr-release` automatic release workflow so releases are produced by release PR + production workflow instead of manual tags/releases.
- Success means `pr-release` generates `RELEASE_NOTES.md` with only the release version heading plus manual release notes, `CHANGELOG.md` remains full, `looper` pins the fixed releasepr module, the public `go install github.com/compozy/releasepr@v0.0.19` works, `pr-release` CI workflow is valid, release PR dry-runs trigger automatically on newly opened release PRs, and required verification passes.

Constraints/Assumptions:

- No destructive git commands: no `git restore`, `git checkout`, `git reset`, `git clean`, or `git rm`.
- Preserve unrelated local `looper` edits in `internal/core/agent/session_test.go` and `web/src/systems/runs/components/run-detail-view.test.tsx`.
- Use root-cause fix in `pr-release`; no local workflow post-processing workaround.
- Target next releasepr module version is `github.com/compozy/releasepr@v0.0.19` unless existing release metadata says otherwise.
- Accepted plan persisted at `.codex/plans/2026-05-01-release-notes-contract.md`.

Key decisions:

- `RELEASE_NOTES.md` should not include the scoped conventional changelog body.
- Keep PR body behavior using scoped changelog plus manual release notes.
- Keep `CHANGELOG.md` generated from full changelog.

State:

- In progress on `pr-release` automatic release workflow restoration.

Done:

- Read latest `looper` and `pr-release` ledgers.
- Confirmed `pr-release` clean at `main`, tag `v0.0.18`, commit `10c8f86 fix: scope release notes changelog`.
- Confirmed `looper` already has local edits bumping workflows/tests to `v0.0.18` and unrelated test hardening.
- Persisted accepted plan.
- Patched `pr-release` so `RELEASE_NOTES.md` is built from the release version heading plus manual notes only.
- Kept scoped changelog behavior for release PR bodies and full changelog behavior for `CHANGELOG.md`.
- Updated `pr-release` tests/docs for the new contract.
- `pr-release` focused orchestrator regression passed.
- `pr-release` `go test ./...` passed.
- `pr-release` `make lint` passed after removing dead `generateChangelog` mode parameter.
- `pr-release` `make test` passed.
- Updated `looper` workflows and release config test target from `releasepr@v0.0.18` to `releasepr@v0.0.19`.
- `looper` focused release workflow test passed: `go test ./test -run TestReleaseWorkflowsUseScopedReleaseNotesGenerator -count=1`.
- Existing local session test hardening passed: `go test ./internal/core/agent -run TestSessionPublishBehavior -count=1`.
- Existing local run detail view test hardening passed: `bun run --cwd web test -- src/systems/runs/components/run-detail-view.test.tsx`.
- First `looper` `make verify` failed in `frontend:test`: full web suite exposed `run-detail-view.test.tsx` not flushing lazy transcript panel before assertion.
- Fixed the existing run detail test helper to render first, then settle dynamic imports inside a follow-up React `act` cycle.
- Focused run detail view test passed after the fix.
- Full web test suite passed: `bun run --cwd web test` (`46` files, `236` tests).
- Published `pr-release` commit `668c486 fix: publish release notes body contract` and annotated tag `v0.0.19`.
- Verified GitHub Releases initially only had `v0.0.17`; created the missing GitHub Release object for `v0.0.19` at `https://github.com/compozy/releasepr/releases/tag/v0.0.19`.
- Verified `releasepr` Release workflow run `25235050968` succeeded in `release-pr` but skipped production release; log showed `latest_tag: v0.0.19`, `has_changes: false`, and `No changes detected since last release`.
- Verified `releasepr` CI workflow runs fail with no jobs because `.github/workflows/ci.yml` has `needs: changes` but no `changes` job.
- Verified default clean `go run github.com/compozy/releasepr@v0.0.19 version` still fails on `sum.golang.org` 404, while the same command succeeds with `GONOSUMDB=github.com/compozy/releasepr`.
- Added `GONOSUMDB=github.com/compozy/releasepr` to looper release and auto-docs workflows and covered it in release config tests.
- Focused looper workflow config test passed: `go test ./test -run TestReleaseWorkflowsUseScopedReleaseNotesGenerator -count=1`.
- Clean looper module-resolution smoke passed with `GONOSUMDB=github.com/compozy/releasepr go run github.com/compozy/releasepr@v0.0.19 version`.
- Full looper verification passed: `make verify` completed with frontend lint/typecheck/tests/build, Go fmt/lint `0 issues`, Go tests `DONE 3009 tests, 3 skipped`, build, Playwright e2e `5 passed`, and `All verification checks passed`.
- Re-tested the exact public install command after Go proxy/checksum DB propagation. `sum.golang.org` now returns checksums for `github.com/compozy/releasepr v0.0.19`, and clean `go install github.com/compozy/releasepr@v0.0.19` exits `0`.
- Removed the temporary `GONOSUMDB=github.com/compozy/releasepr` workflow/test changes because the default public command now works without bypassing the checksum DB.
- Confirmed looper workflows still pin `PR_RELEASE_MODULE: github.com/compozy/releasepr@v0.0.19` and no longer contain `GONOSUMDB`.
- Focused looper workflow config test passed again after bypass removal: `go test ./test -run TestReleaseWorkflowsUseScopedReleaseNotesGenerator -count=1`.
- Full looper verification passed again after bypass removal: `make verify` completed with frontend lint/typecheck/tests/build, Go fmt/lint `0 issues`, Go tests `DONE 3009 tests, 3 skipped`, build, Playwright e2e `5 passed`, and `All verification checks passed`.
- New request: fix `../pr-release` itself so automatic release process works again and manual GitHub releases/tags are not needed.
- Re-read `pr-release/AGENTS.md` and required bugfix/test/Go/final verification skills.
- Verified current `pr-release` CI run `25235051401` fails before jobs start because `.github/workflows/ci.yml` has `needs: changes` and `if: needs.changes.outputs.backend == 'true'` but no `changes` job.
- Verified current `pr-release` release workflow dry-run only listens to `pull_request` `synchronize`, so a newly opened automated release PR is not dry-run validated on open.
- Added root workflow regression tests in `../pr-release/workflow_config_test.go`.
- Confirmed the regression tests fail before the fix on both issues: undefined `needs.changes` and missing `opened`/`reopened` dry-run PR events.
- Patched `../pr-release/.github/workflows/ci.yml` to remove the undefined `changes` dependency gate.
- Patched `../pr-release/.github/workflows/release.yml` so release PR dry-run triggers on `opened`, `synchronize`, and `reopened`.
- Focused workflow regression tests now pass: `go test . -run 'Test(CIWorkflowConfig|ReleaseWorkflowConfig)' -count=1`.
- `../pr-release` lint passed: `make lint` reported `0 issues`.
- `../pr-release` tests passed: `make test` reported `DONE 159 tests`.
- `../pr-release` diff whitespace check passed: `git diff --check`.

Now:

- Commit and push the verified `pr-release` workflow fix, then observe GitHub Actions.

Next:

- If verification passes, push the `pr-release` workflow fix and verify GitHub Actions creates/validates the next automatic release PR.

Open questions (UNCONFIRMED if needed):

- Whether the subsequent automatically created release PR should be merged immediately is UNCONFIRMED; do not merge without explicit instruction.

Working set (files/ids/commands):

- `/Users/pedronauck/Dev/compozy/pr-release/internal/orchestrator/pr_release.go`
- `/Users/pedronauck/Dev/compozy/pr-release/internal/orchestrator/pr_release_test.go`
- `/Users/pedronauck/Dev/compozy/pr-release/README.md`
- `/Users/pedronauck/Dev/compozy/pr-release/docs/plans/2026-04-14-release-notes-design.md`
- `/Users/pedronauck/Dev/compozy/looper/.github/workflows/release.yml`
- `/Users/pedronauck/Dev/compozy/looper/.github/workflows/auto-docs.yml`
- `/Users/pedronauck/Dev/compozy/looper/test/release_config_test.go`
- GitHub Release: `https://github.com/compozy/releasepr/releases/tag/v0.0.19`
- Release workflow run: `https://github.com/compozy/releasepr/actions/runs/25235050968`
- Failing CI run with no jobs: `https://github.com/compozy/releasepr/actions/runs/25235051401`
- `/Users/pedronauck/Dev/compozy/pr-release/.github/workflows/ci.yml`
- `/Users/pedronauck/Dev/compozy/pr-release/.github/workflows/release.yml`
