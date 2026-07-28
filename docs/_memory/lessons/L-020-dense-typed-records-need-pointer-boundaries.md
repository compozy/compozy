# L-020 — Dense typed orchestration records need pointer boundaries

**Class:** Architecture / Code style
**Date discovered:** 2026-05-05 (orch-improvs recurring `gocritic` failure pattern)
**Evidence sources:** Repeated lint failures across historical free-mode slices and the
orchestration-improvements workstream.

## Context

The orchestration-improvements workstream added typed, side-table-backed records to replace
`metadata_json` for orchestration profiles, run reviews, review verdict notifications, and task
context bundles. Each new typed record is large by design: it carries
selector arrays, lineage fields, review history, and bundle data that used to be opaque JSON.

Across the workstream, every transport/helper/test boundary that handled one of these records
by value tripped `gocritic hugeParam` or `rangeValCopy` and blocked `make verify`. The pattern
recurred in:

- `task.Run` review lineage (free slice 020): the first attempt added review fields directly on
  `Run`, but `Run` is hot-path and is copied through scheduler / claim / lease paths. The fix
  grouped the new review state under `Run.Review *RunReviewLineage` so the optional state pays its
  pointer cost only when present.
- `task.RunReviewRequestedNotification`: observer interfaces for review routing were
  drafted as value receivers; the notification carries reviewed-run provenance plus review-row
  state. The fix passed the notification by pointer and cloned contained task/run/review values
  before async/best-effort dispatch.
- Task execution profile request/record paths (free slices 032 / 036): native/HTTP/UDS/CLI
  helpers initially copied `TaskExecutionProfile` requests by value through transport. The fix
  passed request/record values by pointer at those helper/client/test boundaries.
- Sandbox profile session-start helper (free slice 026): the sandbox override helper accepted
  `workspace.ResolvedWorkspace` by value and tripped `hugeParam`. The fix mutated the local
  resolved snapshot through a pointer.

In every case the runtime correctness was unchanged; the fix was a signature shape that already
matched the data's size.

## Root cause

Queryable orchestration state grows fast: side tables, selector arrays, optional lineage,
diagnostic fields. Once the row's typed Go shape exceeds the `gocritic hugeParam` threshold,
every helper/client/test that takes the record by value pays a copy that the linter rejects.
Hot-path structs (run/task/value records that flow through scheduler/claim/lease/transport) feel
the same pressure: appending fat optional state directly onto them forces the entire workspace
into pointer reshapes downstream, even on call sites that never read the new state.

This is structural, not a one-off oversight. The next queryable orchestration feature will hit
the same wall on the same day.

## Rule

> When a typed orchestration record is added to replace `metadata_json`, plan the signature shape
> before adding fields:
>
> 1. Helpers, clients, and test boundaries that handle the new record SHOULD take a pointer
>    parameter from the outset. Treat value-by-copy as the suspicious choice for any record that
>    is intentionally side-table-backed.
> 2. Hot-path structs (`task.Run`, scheduler/claim payloads, transport DTOs that travel through
>    every active-run code path) MUST collect optional dense state behind a typed nested pointer
>    (e.g., `Run.Review *RunReviewLineage`) instead of inlining the fields. Hot-path callers that
>    never read the new state pay no extra copy.
> 3. When `gocritic hugeParam` flags a new boundary, do not silence it. Fix the signature; the
>    lint is reporting structural cost.

## Operationalization

- Pair every "typed instead of metadata_json" decision with a
  signature plan: which helpers/clients/tests take pointers; which hot-path structs receive an
  optional pointer field; which observer/notifier interfaces hand off pointers and clone contained
  values for async dispatch.
- Land the signature shape in the same diff that introduces the new typed record. Discovering the
  shape mid-`make verify` costs a churn cycle and a lint dance.
- For value vs pointer judgement at hot-path boundaries, lean on the `Run.Review` pattern as the
  canonical fix.

## Anti-pattern

- Inlining new dense optional state directly on `task.Run` / hot-path structs because "it reads
  better".
- Suppressing `gocritic hugeParam` on a transport/helper signature instead of moving to pointer
  parameters.
- Treating these failures as one-off lint annoyances instead of structural feedback.

## Source

- Historical workflow memory entries on hot-value review fields, profile transport boundaries,
  observer notification value passing, and resolved workspace helpers.
- `internal/task/lease.go` — `Run.Review *RunReviewLineage` nested optional pointer
- `internal/daemon/native_profile_tools.go`, `internal/cli/client.go`, `internal/cli/task.go`,
  and `internal/api/contract/tasks.go` — profile helper/client/contract pointer boundaries
