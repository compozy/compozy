# BUG-20260911-goal-turn-history-missing: Run Inspect omits Goal turn history

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Lea and Marina
- **Journey Step:** J-26 follow judge rejection and J-27 inspect turn evidence
- **Scenarios:** GL-004; TA-101; LP-run-detail-story-redesign
- **Found:** 2026-09-11
- **Report:** docs/qa/reports/2026-09-10-qa-execution-unblock.md

## Reproduction and evidence

Start the three-turn counter Goal through Web in sess-c0d1802db2c72d21, Run looprun-4e25e5a98540b15d. Real Cursor worker/judge produce count1/rejected, count2/rejected, count3/approved. Public CLI/HTTP/UDS turn reads retain exact blocking issues, criterion evidence and sequence1/2/3. The independently read file advances1/2/3. Open the Run from the Goal chip, expand Inspect, visit Graph, Nodes and Generations, and open the Goal node/details. Only node-level attempt/outcome details are available; the Goal turn timeline and its rejection diagnostics are absent. Reload preserves Done but does not restore that history.

Evidence: goal-convergence-watch.json; goal-convergence-final-turns-http.json; goal-convergence-final-turns-uds.json; goal-convergence-run-full.txt; goal-convergence-graph-detail-full.txt; goal-convergence-run-reloaded.png. The first-rejection-named screenshot was captured after turn3 had begun and does not prove a visible rejection. Origin stop confirmed before diagnosis.

## Cause and contract reconciliation

PR452 (1533e7b64) deleted the Goal turn adapter, query/hook and timeline components during the cockpit replacement. types.ts explicitly notes the missing Web consumer. Its PR description promises that no operator depth is removed and Inspect remains the complete observability home; LP-run-detail-story-redesign still explicitly requires Goal criterion diagnostics and warnings in the timeline. This is a lost read path, not a retired acceptance contract. Preserve the new Inspect layout and restore access to the authoritative paged Goal-turn read there; do not resurrect the old cockpit.

No backend mutation, migration or public wire change is required. Owning layer: Web Loop read boundary and Inspect presentation. Reuse canonical loops-api/query-options/loop-run-page suites for scoped reads, pagination and visible ordered judge diagnostics.

## Fix and impact audit

Restore the existing paged Goal-turn API consumer and timeline inside the current Inspect disclosure. Query caches retain complete page envelopes and server cursors, scoped by workspace, Run and Profile. Goal-turn stream events wake only that canonical read; no local event merge reconstructs outcomes. Loading and failed reads remain distinct from empty history. The current Run story and its four Inspect lanes remain intact.

- Native tools, CLI, HTTP/UDS DTOs: unchanged; the Web consumes the existing read endpoint.
- Extensibility/hooks/config and workspace persistence: unchanged, no migration. Workspace and Run path identity flow through the adapter; Profile identity partitions the cache. No cross-workspace invalidation.
- Official skill and site docs: no changed public agent operation or instruction; no update required.
- Web/QA: Loop Inspect regains ordered result, stop reason, verdict, blockers, criteria, warnings and evidence references. GL-004, TA-101 and LP-run-detail-story-redesign own the replay.

Canonical red reproduction: two new Inspect cases failed before the fix. Focused root Turbo validation then passed 241 tests across the five existing adapter/query/stream/component/route suites. Required gate, build and fresh replay are pending.

## Retest

Lea confirmed the fix in fresh session sess-6c96f86912d1ae77, Run looprun-db7c807f29f666ad, using real Cursor/Grok4.6 High Fast workers and judge. File1/2/3, two rejections then approval, current pending nullable outcomes, exact blockers and evidence references, live Inspect updates and reload agree with independent CLI/HTTP/UDS reads. Origin stopped. Evidence: docs/qa/evidence/2026-09-10-qa-execution-unblock/goal-history-retake-proof.json. GL-004 passes. Broader TA-101 and LP diagnostic branches remain explicitly unverified.

Root Turbo production build and typecheck passed. Initial gate failed the unchanged TaskStatusProjectionObserver disabled-conversation case during SQLite WAL checkpoint on close; all three focused repetitions passed and existing observer cleanup joins its goroutine. No test or production code was weakened. Required gate retry passed all affected lanes: Go, Web771files/7213tests, typecheck and lint0warnings/0errors (goal-turn-history-gate-retry.txt).

Local fix commit: `00890e940`. Explicit lint-staged --no-stash and commitlint passed before commit.
