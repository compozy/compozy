## SMOKE-001: Release Verification Gate

**Priority:** P0 (Critical)
**Type:** Smoke
**Status:** Not Run
**Estimated Time:** 45 minutes
**Created:** 2026-04-23
**Last Updated:** 2026-04-23
**Automation Target:** E2E
**Automation Status:** Existing
**Automation Command/Spec:** `make verify`
**Automation Notes:** Canonical repository gate includes frontend verification, Go formatting/lint/race tests/build, and daemon-served Playwright E2E.

### Objective

Verify the release candidate passes the same full gate required by project policy and CI.

### Preconditions

- Go, Bun, Node, and Playwright dependencies are available.
- Worktree state is recorded.

### Test Steps

1. Run `make verify`.
   **Expected:** Command exits 0 and prints `All verification checks passed`.
2. Inspect output for warnings and failures.
   **Expected:** No lint issues, test failures, build errors, or untriaged warnings.

### Edge Cases

| Variation            | Input                    | Expected Result                                          |
| -------------------- | ------------------------ | -------------------------------------------------------- |
| Missing dependency   | Absent Bun/Go/Playwright | Gate fails with actionable prerequisite                  |
| Pre-existing failure | Frontend token mismatch  | Root cause identified and fixed or documented as blocker |

### Related Test Cases

- TC-FUNC-001
- TC-INT-001
- TC-UI-001
