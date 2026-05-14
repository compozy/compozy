# Goal (incl. success criteria):

- Implement accepted plan to fix compozy reviews fix ACP terminal failure and reviews watch CodeRabbit stale/current-head status handling.
- Success: focused provider/daemon/agent tests pass and make verify passes.

# Constraints/Assumptions:

- Use rtk prefix for shell commands.
- No destructive git commands.
- Work in /Users/pedronauck/Dev/compozy/looper; do not modify /Users/pedronauck/Dev/compozy/agh2 unless explicitly asked for live smoke.
- Tests must prove behavior, not mocks. Fix production code if tests reveal bugs.

# Key decisions:

- Persisted accepted plan to .codex/plans/2026-05-13-reviews-fix-watch.md.
- Add current_settled watch state for CodeRabbit status-success/no-current-review-object path.
- Implement real ACP terminal support rather than suppressing Codex tool use.

# State:

- Implemented provider/watch current_settled behavior and ACP terminal support; focused tests and full make verify passed.

# Done:

- Root-cause investigation completed in Plan Mode.
- Applicable skills loaded: golang-pro, systematic-debugging, no-workarounds, testing-anti-patterns.
- Persisted accepted plan under .codex/plans/.
- Added provider current_settled state and CodeRabbit status-gate behavior.
- Updated review watch readiness to accept current_settled and added daemon coverage for clean and child-fix paths.
- Added ACP terminal command lifecycle support and agent/client terminal tests.
- Focused tests passed:
  - rtk go test ./internal/core/provider/coderabbit -count=1
  - rtk go test ./internal/daemon -run 'ReviewWatch|WaitForCurrentReview' -count=1
  - rtk go test ./internal/core/agent -count=1
- Final verification passed:
  - rtk make verify (first run found errcheck issues; fixed terminal cleanup error handling; rerun exit 0)
- Removed unrelated formatter churn in CHANGELOG.md, RELEASE_BODY.md, and package.json caused by make verify formatting.

# Now:

- Completion audit against plan deliverables.

# Next:

- Report results to user.

# Open questions (UNCONFIRMED if needed):

- None.

# Working set (files/ids/commands):

- .codex/plans/2026-05-13-reviews-fix-watch.md
- .codex/ledger/2026-05-13-MEMORY-reviews-fix-watch.md
- internal/core/provider/\*
- internal/daemon/review_watch.go
- internal/core/agent/\*
