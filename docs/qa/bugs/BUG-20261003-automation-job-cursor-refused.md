# BUG-20261003-automation-job-cursor-refused: A jobs cursor cannot read the next page of the same query

- **Status:** open
- **Fix commit:** pending
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-24, continue through the automation job catalog
- **Scenarios:** TA-052
- **Found:** 2026-10-03 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Reproduction

The isolated profile-recovery lab has 64 enabled monthly edition jobs, all owned by archive-editions
and scheduled for November 1. They were authored through the public CLI before the archive walk.

1. Run `compozy automation jobs --all-profiles --json`.
2. The response returns 50 jobs, total 64, has_more true, and an opaque next_cursor.
3. Repeat the same command with `--cursor` carrying that exact returned value.
4. The CLI refuses it: `cursor does not match this query`.
5. A fresh first page followed immediately by its cursor reproduces the refusal without mutation.

Expected: the unchanged query can consume its own continuation and read the remaining 14 jobs.
Actual: the public continuation is unusable. Other transports and trigger pagination are not yet
qualified by this finding; inspect their owning boundary during the separate fix loop.

## Evidence and scope

docs/qa/evidence/2026-10-02-untested/profile-archive-crash-setup-{jobs-before,
jobs-before-page-two,cursor-first,cursor-second,jobs-help}.json. The original creation receipt
retains all 64 definitions. This was discovered during preparation between persona walks; it is
not a completed Bruno charter. No database query, fabricated cursor or product-code mutation
established the reproduction. The archive recovery branch remains independently testable via its
complete public plan and documented larger page limit; that does not count as a pagination fix.

## Investigation

Pending. Trace cursor query binding across the CLI, transport and canonical automation page owner.
Preserve opaque-cursor validation and exact query scoping; do not drop the check, synthesize a
cursor, or label a larger first page as repaired continuation.
