# Frontend stability — QA cycle plan

- **Scope:** the `front-fixes` branch stabilization of Session/Agent/Home, Network/Bridges, Tasks/Automation/Loops, and Memory/Knowledge data paths: bounded server-owned catalogs, exact totals/facets, cursor continuation, route preload identity, lossless live history, truthful health/metrics, workspace isolation, and derived-catalog recovery.
- **Type:** **planning** (`qa-report`, not a `qa-execution` verdict). No browser, CLI, HTTP, or scenario session was walked while producing this report.
- **Cadence tier:** **Targeted** — every new journey touched by the diff plus existing Session and Loop canaries.
- **Execution companion:** fresh isolated `northstar-pay` real-scenario lab, chosen because Network channels and cross-agent collaboration are a primary changed surface.
- **Tracker rule:** changed behavior is reset to `untested`; existing bug/fix/retest/evidence history is preserved. New behavior receives a new stable row. Planning does not turn any row `pass`.

This report is the planning index for the final branch QA pass. It records the impact flags, new flow maps, durable charters, taxonomy sweep, risk order, environment contract, and teardown obligation. The dated execution report is created with every matrix row `Pending` only after the implementation and scoped automated gates are frozen.

## 1. Tracker impact

### 1.1 Stale verdicts reset to `untested`

Thirty rows changed observable behavior and had a stale non-`untested` verdict. Only `qa_status` changed; all historical bug, fix, retest, commit, evidence, report, overlap, and note fields remain.

- **Session / Agent / Home (13):** `RT-011`, `RT-015`, `RT-017`, `RT-018`, `RT-022`, `RT-024`, `RT-039`, `RT-042`, `RT-044`, `RT-047`, `RT-050`, `RT-051`, `RT-058`.
- **Network / Bridges (7):** `NB-003`, `NB-004`, `NB-005`, `NB-007`, `NB-008`, `NB-019`, `NB-032`.
- **Tasks (1):** `TA-040`.
- **Automation / Loops (4):** `TA-052`, `TA-054`, `TA-056`, `LP-025`.
- **Memory / Knowledge (5):** `MS-001`, `MS-006`, `MS-008`, `MS-009`, `MS-015`.

`RT-039`, `NB-003`, `NB-004`, and `NB-008` were previously `blocked-verify`; the changed surfaces now require a fresh verdict rather than inheriting the old browser boundary.

### 1.2 Changed rows already `untested`

Twenty-four rows already had the correct status. They retain `untested`, receive a dated planning note, and use the current contract language where needed:

- **Session:** `RT-012`, `RT-023`, `RT-040`, `RT-041`, `RT-043`, `RT-045`, `RT-046`, `RT-052`.
- **Network / Bridges:** `NB-009`, `NB-010`, `NB-013`, `NB-015`, `NB-024`, `NB-027`.
- **Tasks / Automation / Loops:** `TA-002`, `TA-065`, `TA-067`, `TA-072`, `TA-073`, `TA-074`, `TA-078`, `LP-001`, `LP-033`, `LP-034`.

`RT-023` was already hard-cut to the fenced transcript contract and was not given a duplicate note.

### 1.3 New behavior rows

- **`NB-047` — Return to Network work without workspace bleed.** Owns URL-workspace identity during immediate post-switch actions plus workspace/surface/conversation-isolated last-read state and abandonment/resume.
- **`MS-059` — Recover Memory catalog after interrupted mutation.** Owns source-first crash consistency, the durable dirty marker, exact identity rebuild, and no cross-identity leakage.

The tracker now contains 348 rows, 16 columns per row, unique IDs, and valid `qa_status`, `fix_status`, and `retest_status` enums.

### 1.4 Tracker schema repair

Three historical Loop rows stored environment reasons in the `retest_status` enum. `LP-003`, `LP-029`, and `LP-046` now use the valid value `pending`; their provider/repository prerequisites remain in `notes`, and no scenario verdict changed.

## 2. Contract hard cuts reflected in scenarios

- **Session:** counted session pages with exact totals; `include_health` hydrates only returned IDs; bounded transcript REST pages use `before_sequence`; live transcript reconnect uses `after_sequence` plus `epoch`/`generation`; only explicit reset reasons replace the cached tail; scroll anchors use message identity and offset.
- **Home/status:** the Home card and shell share the canonical `/api/status` cache; only `health.status` may label a connected daemon degraded; Active Sessions is an exact backend total.
- **Network:** channels embed bounded recents; thread/direct catalogs and activity expose daemon-owned order, filters, exact totals, and continuation; message history separates bounded `before` pages from the incremental `after` tail; open-work total is distinct from loaded forensic rows.
- **Bridges:** list returns counted pages plus exact facets and page-scoped health; health SSE requires 1–200 explicitly loaded `bridge_ids` and emits stable-ID snapshots/diffs.
- **Tasks:** list/Kanban and actor-scoped Inbox preserve lean counted envelopes, exact facets/lane totals, server order, query-bound cursors, and explicit continuation. Rich pause/dependency/designation/claim fields remain detail-only.
- **Automation:** job/trigger catalogs expose counted pages and public `source=config|package|dynamic`, tri-state `enabled`, query/event/loop/scope filters, exact totals, and continuation. Recent detail metrics are labelled `Runs shown` and `Recent success`.
- **Loops:** list exposes `{loops,facets,page}`, server-owned filters, lean `last_run`, exact totals, and cursor continuation across web/CLI/native. Start bindings page jobs/triggers independently; the sampled catalog binding badge is removed.
- **Memory:** list exposes exact identity-scoped pages beyond 200 entries; semantic recall remains separate; readiness/dirty recovery is identity-specific; FTS is rebuilt by the appended repair migration; decision filename filtering occurs before the limit.

Historical reports and bug narratives remain historical. Live planning artifacts `J-11`, `CH-014`, `personas.md`, and `AB-005` were hard-cut from snapshot replay to bounded REST history plus fenced transcript SSE.

## 3. New journey inventory

| Journey | Value | Persona | Primary abandonment/resume |
|---|---|---|---|
| [`J-23`](../journeys/J-23-return-to-network-work.md) | Resume Network work with exact history/totals and no workspace bleed | Théo | connection drops during older-history load; return through deep link |
| [`J-24`](../journeys/J-24-triage-work-at-scale.md) | Triage tasks and manage automations in catalogs larger than one page | Bruno | close after several pages; return to persisted server state |
| [`J-25`](../journeys/J-25-browse-recover-knowledge.md) | Browse and recover durable knowledge without ghost rows or identity leaks | Rafa | process exits between source mutation and derived synchronization |

Each flow contains entry points, observable actions, branches/errors, side effects, a true end state, and an abandonment/resume path before its scenarios are executed.

## 4. Durable charter roster

### New charters

| Order | Charter | Journey | Persona | Tour | Box | Scenario scope |
|---|---|---|---|---|---|---|
| 1 | [`CH-037`](../charters/CH-037.md) | J-23 | Théo | Interrupt Tour | 60m | Network continuity, first-thread discoverability, paging, live tail, workspace isolation, bridge health |
| 2 | [`CH-038`](../charters/CH-038.md) | J-24 | Bruno | Feature Tour | 60m | Tasks/Inbox, Automation counted pages, recent metrics, Loop bindings |
| 3 | [`CH-039`](../charters/CH-039.md) | J-25 | Rafa | Interrupt Tour | 60m | Knowledge paging/search/decisions plus interrupted-sync recovery |

### Existing canaries reused

- **`CH-014` / J-11 / Théo / Interrupt Tour:** return to a live background session, fenced reconnect, workspace switch, truthful lifecycle.
- **`CH-018` / J-15 / Ada / Feature Tour:** bounded transcript REST/SSE plus HTTP/UDS/CLI parity and stop-race reads.
- **`CH-009` / J-09 / Marina / Back-Button Tour:** Loop Start bindings and automation targeting after the sampled catalog badge removal.

This is a Targeted branch cycle with six durable sessions. If the executable window must be risk-cut, the report marks unwalked rows `Skipped` with reasoning; coverage never shrinks silently.

## 5. Taxonomy sweep

| Dimension | Coverage or deliberate boundary |
|---|---|
| Journeys | J-23/J-24/J-25 map the previously unmapped Network, task/automation, and knowledge value flows; J-11/J-15/J-09 are adjacent canaries. |
| Functional | Public list/detail/action round-trips, exact totals/facets, cursor continuation, refresh/deep-link persistence, validation branches, and workspace/identity isolation ride inside each charter. |
| Experiential | Truthful completeness labels, loading/error states, perceived first paint, accessible Load more/Load older controls, and paper cuts are recorded in debriefs. |
| Edge/error/empty | Offline/resume, daemon/Network disabled, empty filters, multi-page catalogs, stale transcript fences, invalid Loop target, and interrupted Memory synchronization are explicit branches. |
| Cross-cutting | Workspace and identity isolation, HTTP/UDS/CLI/native parity, route preload/cache identity, real browser viewport checks, and Session/Loop adjacent canaries are in scope. |

Deliberate boundaries: this cycle is not a full mobile compatibility matrix, security audit, synthetic load test, or database-level QA session. Storage/race/contract behavior is proven by automated gates; persona verdicts use public surfaces only. Production-parity deviations are disclosed in the execution report.

## 6. Execution environment and evidence

1. Freeze implementation and pass the touched frontend/backend scoped gates.
2. Bootstrap a **fresh** isolated `northstar-pay` lab; never reuse the earlier forensics manifest.
3. Export `AGH_WEB_API_PROXY_TARGET` from the manifest and preserve the provider home policy it declares.
4. Create `docs/qa/reports/2026-07-11-northstar-pay.md` with every session matrix row `Pending` before the first walk.
5. Post exactly one in-persona operator kickoff, observe without evaluator prompts, and collect public CLI/API/Web evidence.
6. Capture deterministic Storybook PNGs for changed visible surfaces and real-app checkpoints/failures only.
7. Run the strict scenario auditor, dedup/file any findings, and update tracker/charter debriefs after each session.
8. Run the full project exit gate once after the last code change and record it verbatim.
9. Execute the manifest `TEARDOWN_COMMAND` on every terminal path and require `teardown.json` with `"clean": true`.

Lab-side bulk artifacts remain under `QA_OUTPUT_PATH`; the living tracker, reports, charters, journeys, and bug registry remain under `docs/qa/`.

## 7. Planning completeness

- Every new user-visible behavior has a stable existing or new scenario row.
- Every new journey has a flow before its scenario matrix.
- Every in-scope new journey has one persona, exactly one tour, and a bounded charter.
- Existing Session/Loop journeys are reused rather than duplicated.
- All affected verdicts reflect current uncertainty (`untested`) without erasing historical evidence.
- All five taxonomy dimensions are covered or deliberately bounded.
- No QA session was run and no pass/fail claim was made during planning.

Execution begins only after the last frontend lane is source-frozen and the scoped automated preconditions are green.
