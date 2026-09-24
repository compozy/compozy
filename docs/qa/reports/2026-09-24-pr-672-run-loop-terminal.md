# QA Run Report — 2026-09-24 — PR 672 run-loop terminal output

- **Scope:** Issue 671 awaited child terminal output, downstream rendering, validation, and recovery after daemon restart.
- **Cadence tier:** targeted
- **Build:** `6749eabd42b3b17d5ab3b37263661499fc96ac6b` · **Environment:** isolated local QA lab, branch-built daemon and public CLI/HTTP surfaces
- **Started:** 2026-09-24T19:42Z · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Power User | desktop / wifi-fast / en-US | CH-await-child-loop-restart |

## Flows in Scope

- `J-await-child-loop` — run two ordered awaited children across a daemon restart (`../journeys/J-await-child-loop.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-await-child-loop-restart | J-await-child-loop / LP-run-loop-await-child-ordering | Bruno | Interrupt Tour | Fixed | [#671](https://github.com/compozy/compozy/issues/671) | `81a193db8d9822b85616230d6a3a5a85b6663623` |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-await-child-loop-restart — Bruno

- **Ran:** 2026-09-24T19:46Z → 2026-09-24T19:49Z (box respected: yes).
- **Findings:** Public CLI and HTTP reads showed parent `looprun-dfe321ae14454b7f` with first child `looprun-42c030078ebd7306` awaiting, while the receipt and second child were pending. A daemon stop/start preserved that child ID; the runs list contained exactly one child. Resuming its durable wait settled the declared output to `{"loop_run_id":"looprun-42c030078ebd7306","status":"done"}`, rendered `First handoff looprun-42c030078ebd7306 finished with done`, and only then started second child `looprun-ceb800b0cfab8935`. After its wait resumed, all three runs were `done`; its undeclared node output remained `child_loop_status:done`. Fresh HTTP and CLI reads agreed.
- **Bugs filed/updated:** Issue #671 is fixed in this PR; no new user-facing defect observed.
- **Scenarios settled:** LP-run-loop-await-child-ordering → pass.
- **Paper cuts:** None observed in this targeted walk.
- **Surprises:** Restart reconciliation logged two `terminal run lifecycle identity is incomplete` warnings, but public read and resume paths completed normally.
- **Suggested next charter:** Packaged release smoke after merge.

Restorable receipts: `docs/qa/evidence/2026-09-24-pr-672-run-loop-terminal/` (Skeeper namespace `compozy`, pinned by `skeeper.lock`). The strict lab auditor passed with zero blockers and warnings; its result is `docs/qa/evidence/2026-09-24-pr-672-run-loop-terminal/qa-audit-report.json`.

## What Was Fixed

Issue #671's declared terminal output and the review remediation in `81a193db8d9822b85616230d6a3a5a85b6663623` were retested through the public runtime. The validator accepted the supported declaration and rejected an enum-constrained `status` and an extra output field with `run_loop_output_shape_invalid`.

## Paper Cuts

None observed.

## Runtime Errors Observed

The daemon logged `terminal run lifecycle identity is incomplete` twice during restart reconciliation. Both children and the parent subsequently reached `done`; no user-visible failure was observed.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

The structured and legacy scalar outputs can be observed in the same parent run through public detail reads.

## Final Status

- **Targeted QA:** PASS. One of one in-scope journeys completed after a real daemon restart; CLI and HTTP terminal reads matched.
- **Local gate:** `make gate` passed on this code head before the earlier commit and push. Its cached status is recorded at `docs/qa/evidence/2026-09-24-pr-672-run-loop-terminal/verify-gate.log`.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0.
- **PR readiness:** Pending exact-head CI completion and required code-owner approval.
