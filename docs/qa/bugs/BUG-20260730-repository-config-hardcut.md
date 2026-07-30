# BUG-20260730-repository-config-hardcut: Repository config blocked workspace registration

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-09 register and run the project, step 1
- **Scenarios:** TA-079
- **Found:** 2026-07-30 · **Report:** docs/qa/reports/2026-07-28-untested-full.md

## Summary

Ada could not register the Compozy repository in a fresh current-source daemon because its tracked workspace config still used removed beta keys.

## Reproduction

- **Charter:** CH-current-config-hardcut · **Tour:** Restart Tour
- **Environment:** isolated current-source daemon / CLI / en-US

1. Run `compozy workspace add` for this repository from a fresh runtime home.
2. Observe config loading.

**Expected:** Current Loop defaults load and the workspace registers.
**Actual:** Registration failed on removed `defaults.*`, `tasks`, and `tasks.run.*` keys.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-root-fixes-retest-20260730-072350-328928-lab/qa-artifacts/qa/evidence/root-fixes/public-replay.md`

## Fix

- **Root cause:** The repository config and bundled task-authoring guidance had not been hard-cut to `loops.defaults.*` and free-form task type routing.
- **Fix commit:** cbae345
- **Regression test:** Documented fresh-runtime workspace registration and `config validate` replay; runtime schema coverage lives in `internal/config/loops_test.go`.

## Verification

- **Retested:** 2026-07-30, same persona/journey · **Report:** docs/qa/reports/2026-07-28-untested-full.md
- **Result:** Config validation returned `status: valid`; workspace registration succeeded and exposed both bundled Loops.
