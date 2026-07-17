# Expected outcome — feat-telemetry (degraded mode)

Exercises: degraded mode (E2E-016) and the broken-range report (part of E2E-016).

There is intentionally **no** `diff.patch`: the runbook stages this workflow with an unscopable diff
(e.g. checkout so `git diff main...HEAD` fails, or point capture at a broken commit range).

## With unscopable diff, but reviews + memory present

- Capture degrades to `memory/MEMORY.md` + `reviews-001/issue_001.md` as the basis.
- The OpenTelemetry decision is promoted as `status: candidate` with `## Reconciliation` marked
  "unverified against code". It is written to its `AD-NNN.md` body but is **absent** from the index.
- The run summary reports the diff-scoping failure; the run does not crash.

## With neither diff nor reviews (remove `reviews-001/`)

- Capture promotes **nothing** (no fabricated evidence). No log files are created.

## Assertions

- No `proven` record is produced when the diff is unscopable.
- A broken/undefined commit range is reported as a scoping failure, not an exception.
