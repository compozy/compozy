Goal (incl. success criteria):

- Implement the accepted release-notes contract fix across `pr-release` and `looper`.
- Success means `pr-release` generates a historical `RELEASE_NOTES.md`, a current-version-only `RELEASE_BODY.md`, `looper` pins the fixed releasepr module, `v0.2.1` release notes/body are repaired, the GitHub release `v0.2.1` is updated if credentials allow, and required verification passes.

Constraints/Assumptions:

- No destructive git commands: no `git restore`, `git checkout`, `git reset`, `git clean`, or `git rm`.
- Preserve unrelated local `looper` edits in `internal/core/agent/session_test.go` and `web/src/systems/runs/components/run-detail-view.test.tsx`.
- Use root-cause fix in `pr-release`; no local workflow post-processing workaround.
- Target next releasepr module version is `github.com/compozy/releasepr@v0.0.20` unless existing release metadata says otherwise.
- Accepted plan must be persisted at `.codex/plans/2026-05-01-release-notes-contract.md`.

Key decisions:

- `RELEASE_NOTES.md` is historical and prepends/replaces the current version section while preserving prior version sections.
- `RELEASE_BODY.md` is the current-version-only publication artifact for GoReleaser/GitHub Releases.
- Keep PR body behavior using scoped changelog plus manual release notes.
- If no manual notes exist, release publication falls back to the scoped conventional changelog.
- Keep `CHANGELOG.md` generated from full changelog.

State:

- Implementing the accepted follow-up plan after real `v0.2.1` produced an effectively empty release body.

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
- Pushed `pr-release` commit `9a747dd fix: restore automatic release workflows`; GitHub Release workflow run `25235803107` succeeded and created release PR `#9` for `release/v0.0.20`.
- GitHub CI run `25235803104` now starts a real `test` job instead of failing workflow parsing, but the job failed at `Setup Bun`: `bun ci` reported `lockfile had changes, but lockfile is frozen`.
- Root cause: tracked `bun.lock` still contains stale dev dependencies while current `package.json` has no dependencies.
- Removed the stale empty Bun lockfile via `bun install --lockfile-only`; `bun ci` now passes locally.
- Added package manifest regression coverage so `bun.lock` cannot be committed when `package.json` declares no dependency sections.
- Focused workflow/package config tests pass: `go test . -run 'Test(CIWorkflowConfig|ReleaseWorkflowConfig|PackageManifestConfig)' -count=1`.
- `../pr-release` lint passed after lockfile fix: `make lint` reported `0 issues`.
- `../pr-release` tests passed after lockfile fix: `make test` reported `DONE 161 tests`.
- `../pr-release` diff whitespace check passed after lockfile fix: `git diff --check`.
- Pushed `pr-release` commit `9698860 fix: align bun lockfile with manifest`.
- Verified GitHub CI run `25235885446` succeeded on `main` after the lockfile fix.
- Verified GitHub Release workflow run `25235885442` now fails in `Run PR Release Orchestrator` because it treats existing remote branch `release/v0.0.20` as new and attempts a non-fast-forward normal push.
- Root cause identified in `internal/orchestrator/pr_release.go`: `createdInSession` uses only local branch existence and ignores `remoteExists`, so clean CI runners misclassify pre-existing remote release branches.
- Patched `../pr-release` saga branch bookkeeping to track local branch creation, remote branch pre-existence, and remote branch rollback ownership separately.
- Added regression coverage for the clean-runner case where the release branch exists remotely but not locally; it now expects `PushBranchForce`.
- Updated rollback tests so new-branch cases explicitly report `RemoteBranchExists=false` during branch creation.
- Focused regression passed: `go test ./internal/orchestrator -run 'TestPRReleaseOrchestrator_Execute/Should_force_push_when_release_branch_already_exists_remotely' -count=1`.
- Orchestrator package tests passed: `go test ./internal/orchestrator -count=1`.
- `../pr-release` lint passed: `make lint` reported `0 issues`.
- `../pr-release` tests passed: `make test` reported `DONE 162 tests`.
- `../pr-release` diff whitespace check passed: `git diff --check`.
- Pushed `pr-release` commit `766ec79 fix: update existing release branch`.
- Verified GitHub Release workflow run `25236159969` succeeded and updated PR #9 from old head `91620a6` to `ea4ec8c`.
- Verified GitHub CI run `25236159971` succeeded on `main` after `766ec79`.
- Observed PR #9 did not get new `pull_request` CI/dry-run runs after the force-push; GitHub issue events show `head_ref_force_pushed` by `github-actions[bot]`, matching GitHub's documented `GITHUB_TOKEN` event suppression behavior.
- Patched `.github/workflows/release.yml` so the release-pr job explicitly dispatches CI and release dry-run workflows for the release branch via `workflow_dispatch`.
- Added workflow config regression coverage for dispatch mode, dry-run inputs, `actions: write`, branch checkout ref, and dispatched dry-run env.
- Focused release workflow config test passed: `go test . -run 'TestReleaseWorkflowConfig' -count=1`.
- `../pr-release` lint passed after dispatch workflow fix: `make lint` reported `0 issues`.
- `../pr-release` tests passed after dispatch workflow fix: `make test` reported `DONE 163 tests`.
- `../pr-release` diff whitespace check passed after dispatch workflow fix: `git diff --check`.
- Pushed `pr-release` commit `c6ec9a4 fix: dispatch release pr checks`.
- Verified GitHub Release workflow run `25236362937` succeeded on `main`; `Dispatch Release PR Checks` succeeded.
- Verified GitHub CI run `25236362924` succeeded on `main`.
- Verified dispatched PR-branch CI run `25236378047` succeeded on `release/v0.0.20` at head `a13e3b9`.
- Verified dispatched PR-branch Release dry-run run `25236378838` succeeded on `release/v0.0.20` at head `a13e3b9`.
- Verified commit check-runs for PR #9 head `a13e3b9379d78c94ae353280e4e494d14bc45dc3`: `test=success`, `Dry-Run Release Check=success`; PR #9 merge state is `CLEAN`.
- Verified PR #9 diff only includes `CHANGELOG.md`, `RELEASE_NOTES.md`, and `package.json`; remote `RELEASE_NOTES.md` content is just `## 0.0.20 - 2026-05-01`.
- New issue: public Compozy `v0.2.1` release body and `main:RELEASE_NOTES.md` contain only `## 0.2.1 - 2026-05-01`.
- Root cause confirmed: `releasepr@v0.0.19` intentionally writes `RELEASE_NOTES.md` as heading + manual notes; with no `.release-notes/*.md` for `0.2.1`, it discarded scoped changelog details and overwrote historical `0.2.0` release notes.
- User selected the corrected contract:
  - `RELEASE_NOTES.md`: historical, preserving old release sections.
  - GitHub Release body: current version only.
  - Empty manual notes fallback: scoped changelog, not heading-only.
- Persisted the accepted follow-up plan to `.codex/plans/2026-05-01-release-notes-contract.md`.
- Patched `pr-release` to generate `RELEASE_BODY.md` for publication and historical `RELEASE_NOTES.md` with same-version replacement.
- Updated `pr-release` dry-run GoReleaser args to use `RELEASE_BODY.md`.
- Updated `pr-release` tests/docs for the new contract.
- Focused upstream tests passed:
  - `go test ./internal/orchestrator -run 'TestPRReleaseOrchestrator_generateChangelog|TestDryRunOrchestrator_Execute/Should_successfully_execute_dry-run_validation|TestPRReleaseOrchestrator_commitChanges' -count=1`
- Broader upstream checks passed:
  - `go test ./internal/orchestrator -count=1`
  - `go test . -run 'TestReleaseWorkflowConfig' -count=1`
  - `go test ./...`
  - `make lint`
  - `make test` (`DONE 165 tests`)
- Published upstream `releasepr` commit `073ed3a fix: split release notes history and body`.
- Published upstream tag `v0.0.20`.
- Verified module resolution:
  - `go run github.com/compozy/releasepr@v0.0.20 version`
  - `GOPROXY=direct go run github.com/compozy/releasepr@v0.0.20 version`
- Updated `looper` workflows to `github.com/compozy/releasepr@v0.0.20`.
- Updated `looper` release workflow to ensure/publish `RELEASE_BODY.md` instead of historical `RELEASE_NOTES.md`.
- Added `RELEASE_BODY.md` for `0.2.1` and regenerated `RELEASE_NOTES.md` as historical `0.2.1` + restored GitHub `v0.2.0` body.
- Added release config regression for current body vs historical notes.
- Focused `looper` release tests passed:
  - `go test ./test -run 'TestReleaseWorkflowsUseScopedReleaseNotesGenerator|TestReleasePublicationUsesCurrentBodyAndHistoricalNotes|TestGoReleaserConfigSupportsFirstRelease' -count=1`
- Updated public GitHub Release `v0.2.1` with `RELEASE_BODY.md`; verified body now includes `Bug Fixes` and `Binary release`.
- Final `looper` verification passed: `make verify` completed with frontend lint/typecheck/tests/build, Go fmt/lint `0 issues`, Go tests `DONE 3010 tests, 3 skipped`, build, Playwright e2e `5 passed`, and `All verification checks passed`.

Now:

- Prepare final summary.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `/Users/pedronauck/Dev/compozy/pr-release/internal/orchestrator/pr_release.go`
- `/Users/pedronauck/Dev/compozy/pr-release/internal/orchestrator/pr_release_test.go`
- `/Users/pedronauck/Dev/compozy/pr-release/README.md`
- `/Users/pedronauck/Dev/compozy/pr-release/docs/plans/2026-04-14-release-notes-design.md`
- `/Users/pedronauck/Dev/compozy/looper/.github/workflows/release.yml`
- `/Users/pedronauck/Dev/compozy/looper/.github/workflows/auto-docs.yml`
- `/Users/pedronauck/Dev/compozy/looper/test/release_config_test.go`
- `/Users/pedronauck/Dev/compozy/looper/RELEASE_NOTES.md`
- `/Users/pedronauck/Dev/compozy/looper/RELEASE_BODY.md`
- GitHub Release: `https://github.com/compozy/releasepr/releases/tag/v0.0.19`
- Release workflow run: `https://github.com/compozy/releasepr/actions/runs/25235050968`
- Failing CI run with no jobs: `https://github.com/compozy/releasepr/actions/runs/25235051401`
- Release PR created by automation: `https://github.com/compozy/releasepr/pull/9`
- `/Users/pedronauck/Dev/compozy/pr-release/.github/workflows/ci.yml`
- `/Users/pedronauck/Dev/compozy/pr-release/.github/workflows/release.yml`
