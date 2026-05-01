Goal (incl. success criteria):

- Implement the accepted release-notes contract fix across `pr-release` and `looper`.
- Success means `pr-release` generates `RELEASE_NOTES.md` with only the release version heading plus manual release notes, `CHANGELOG.md` remains full, `looper` pins the fixed releasepr module, and required verification passes.

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

- In progress.

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

Now:

- Re-run `looper` `make verify`.

Next:

- Run final verification skill after `make verify`.

Open questions (UNCONFIRMED if needed):

- Whether user wants me to create/push the `v0.0.19` tag is UNCONFIRMED; implement code and report if publishing remains external.

Working set (files/ids/commands):

- `/Users/pedronauck/Dev/compozy/pr-release/internal/orchestrator/pr_release.go`
- `/Users/pedronauck/Dev/compozy/pr-release/internal/orchestrator/pr_release_test.go`
- `/Users/pedronauck/Dev/compozy/pr-release/README.md`
- `/Users/pedronauck/Dev/compozy/pr-release/docs/plans/2026-04-14-release-notes-design.md`
- `/Users/pedronauck/Dev/compozy/looper/.github/workflows/release.yml`
- `/Users/pedronauck/Dev/compozy/looper/.github/workflows/auto-docs.yml`
- `/Users/pedronauck/Dev/compozy/looper/test/release_config_test.go`
