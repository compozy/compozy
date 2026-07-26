# QA Run Report — 2026-07-11 — Northstar Pay

- **Scope:** targeted `front-fixes` stabilization of Session/Agent/Home, Network/Bridges, Tasks/Automation/Loops, and Memory/Knowledge data paths, exercised inside the broad `northstar-pay` real-scenario playbook.
- **Cadence tier:** targeted product regression pass inside a broad release-grade scenario.
- **Build:** implementation commit `f6748f2f`, rebased onto `origin/main` at `dadf35f1`.
- **Environment:** fresh isolated lab `northstar-pay-20260711-153916-425791`; daemon `http://127.0.0.1:44473`; Web `http://127.0.0.1:3000`; no HTTP/API mocks.
- **Provider:** real Claude `claude-sonnet-4-6`; one operator kickoff; no follow-up provider prompt.
- **Started:** 2026-07-11T15:45:04.035542Z.
- **Verdict:** **BLOCKED** — the frontend/runtime regression fixes are verified, but the Northstar Pay autonomous scenario did not produce the required Task runs, deliverables, disruption recoveries, or collaboration loops.

## Result Separation

This run produced two independent results that must not be conflated:

1. **Stabilization regression result — verified.** `BUG-0029` through `BUG-0032` have real public-surface retest evidence. Network first-thread discoverability, native workspace identity, exact Memory health counts, and causal Network history all behave as fixed.
2. **Northstar Pay autonomous-playbook result — BLOCKED.** The single Product Manager turn ended normally, but the declared team did not activate. The strict auditor correctly prevents the successful regression checks from being promoted to a release-grade playbook pass.

## Personas and Charters

| Persona | Primary charter | Evidence disposition |
|---|---|---|
| Théo | CH-037 Network continuity; CH-014 Session return | Real browser evidence, with partial-contract limits called out below |
| Bruno | CH-038 work triage at scale | Scale/list evidence only; broad task and automation stories were not fully walked |
| Rafa | CH-039 durable knowledge | Exact Memory-health repair verified; broader recovery journey not fully walked |
| Marina | CH-009 Loop binding detail | Storybook and source evidence only; no complete real journey |
| Ada | CH-018 CLI/API Session operation | Read-only samples only; no full fenced-live-session journey |
| Sofia Mendes / Maya Singh | Northstar Pay autonomous collaboration | Failed at the team-activation boundary; `BUG-0028` |

## Run Coverage Matrix

Every entry below has a terminal disposition for this run. `Skipped` means the row remains `untested` in `docs/qa/state.csv` for a future complete walk; partial evidence is not promoted to a product verdict.

| Scenario IDs | Run disposition | What this run established |
|---|---|---|
| `NB-012`, defect-specific portion of `NB-020` | Fixed | The corrected native descriptor and CLI help exposed the public-thread grammar and implicit creation; real Claude then created `thread_war-room-launch-001`. The separate native backend identity defect was fixed and retested without another provider prompt. |
| `MS-011` | Fixed | Exact health was observed as global `4`, workspace `206`, indexed `210`, orphaned `0` through API and native-tool surfaces. |
| `NB-010`, defect-specific causal-order portion | Fixed | A real pre-rebase branch database upgraded through local v67/v68/v69; those operations occupy final registry v70/v71/v72. All 125 rows were preserved and public `before`/`after` pages plus Web DOM order were causal. Filters, invalid-ID branches, and live-append viewport anchoring were not walked, so the complete scenario remains `untested`. |
| `RT-073` | Fail — `BUG-0028` | The single kickoff did not activate collaborators, Task runs, reviews, disruption recovery, or deliverables. |
| `NB-005`, `NB-009`, `TA-002`, `TA-040`, `TA-052`, `TA-054`, `TA-056`, `TA-065`, `MS-001`, `MS-006`, `MS-008`, `MS-009`, `MS-015`, `MS-059` | Skipped after partial scale evidence | Real catalog pages, totals, continuation, and screenshots were sampled at scale, but each tracker row contains additional filters, mutations, error states, or recovery behavior not fully exercised. |
| `RT-041`, `RT-043`, `RT-045` | Skipped after partial persistence evidence | A stopped session retained one persisted `Sofia Mendes` transcript marker through navigation to Tasks, browser back, and reload. No live background session, workspace switch, transient 5xx, or lifecycle transition while away was exercised. |
| `NB-003`, `NB-004`, `NB-007`, `NB-008`, `NB-013`, `NB-015`, `NB-019`, `NB-024`, `NB-027`, `NB-032`, `NB-047` | Skipped | The complete Network/Bridge contracts were not walked. In particular, no direct-room history or cross-workspace last-read/unread/write isolation verdict is inferred from thread-only evidence. |
| `RT-015`, `RT-023`, `RT-024`, `RT-050`, `RT-051`, `RT-022`, `RT-012`, `RT-042` | Skipped | No attach, fenced live SSE, inspect/health matrix, stop/finalize race, or complete HTTP/UDS parity journey ran in this scenario. |
| `LP-033`, `LP-034`, `LP-035` | Skipped | Loop binding source and Storybook evidence did not constitute a real operator journey. |

The following changed rows were explicitly outside this targeted execution and remain `untested`: `RT-011`, `RT-017`, `RT-018`, `RT-039`, `RT-040`, `RT-044`, `RT-046`, `RT-047`, `RT-052`, `RT-058`, `TA-067`, `TA-072`, `TA-073`, `TA-074`, `TA-078`, `LP-001`, and `LP-025`.

## Bootstrap and Runtime

- **Manifest:** `/home/pedronauck/dev/qa-labs/agh-northstar-pay-20260711-153916-425791-lab/qa-artifacts/qa/bootstrap-manifest.json`.
- **Scenario contract:** `/home/pedronauck/dev/qa-labs/agh-northstar-pay-20260711-153916-425791-lab/qa-artifacts/qa/scenario-contract.json`.
- **Behavioral charter:** `/home/pedronauck/dev/qa-labs/agh-northstar-pay-20260711-153916-425791-lab/qa-artifacts/qa/behavioral-scenario-charter.yaml`.
- **Workspace:** `/home/pedronauck/dev/qa-labs/agh-northstar-pay-20260711-153916-425791-lab`.
- **Runtime home:** `/tmp/aghqa-093029b162b3/runtime`.
- **Registry workspace:** `ws_dc76b2b52e82493a`; durable workspace identity `01KX8XCTQRVF7GZ9N815GVSXYK`.
- **Provider session:** `sess-40ede54a94b77e0f`; turn `turn-cff59efdc929d215`; healthy `end_turn` at event sequence 212.
- **Final retest daemon:** PID `1446547`, upgraded in place through pre-rebase branch GlobalDB v69 (the same cleanup is final registry v72).
- **Web startup side effect:** onboarding created `sess-4ae1c2e999864db0`; it is not counted as a playbook collaborator or provider proof.

## Playbook Compliance

| Contract item | Observed | Required | Result |
|---|---:|---:|---|
| Declared agents | 11 | 11 | Met declaratively |
| Differentiated roles | 11 | 11 | Met declaratively |
| Declared channels | 10 | 10 | Met declaratively |
| Provider-backed sessions with observed decisions | 1 | 1 | Met |
| Task roots declared | 12 | 12 | Met declaratively |
| Task dependencies declared | 12 | 12 | Met declaratively |
| Task runs | 0 | 12 | Blocker C6 |
| Same objects observed through CLI/API/Web/runtime | 2 | 3 | Blocker C8 |
| Artifacts used later | 0 | 2 | Blocker C10 |
| Completed disruption probes | 0 | 3 | Blocker C11 |
| Non-Markdown deliverables | 0 | at least 4 | Blocker C16 |
| Peer messages | 0 | 14 | Blocker C17 |
| Complete review cycles | 0 | 2 | Blocker C17 |
| Resolved disagreements | 0 | 1 | Blocker C17 |
| Active channels in the journey log | 0 | 5 | Blocker C17 |

No declared TSX page, TSX component, TypeScript module/test, Go service stub, or shell script was produced. The auditor mechanically finds generic Markdown files by extension, but none is one of the declared product runbooks; this report does not use that extension-only false positive as evidence.

## Autonomous Collaboration Failure

The 12 seeded Tasks existed before kickoff as `nsp-001` through `nsp-012`, each `ready` and assigned to its declared agent pool, but with `auto_enqueue_on_ready=0`. The Product Manager did not list or start those tasks. It instead created seven duplicate Tasks, all unowned and also not auto-enqueued, then posted one public-thread checklist and asked owners to pick up work.

No declared collaborator session subsequently started, and no Task acquired a run. Because the evidence does not yet distinguish an ambiguous playbook handoff from a missing runtime activation path, `BUG-0028` leaves the root cause `UNCONFIRMED`. No second prompt was sent to wake the scenario or manufacture collaboration.

Evidence:

- `/home/pedronauck/dev/qa-labs/agh-northstar-pay-20260711-153916-425791-lab/qa-artifacts/qa/notes/bug-0028-autonomy-stall.json`
- `/home/pedronauck/dev/qa-labs/agh-northstar-pay-20260711-153916-425791-lab/qa-artifacts/qa/provider-attempt.json`
- `/home/pedronauck/dev/qa-labs/agh-northstar-pay-20260711-153916-425791-lab/qa-artifacts/qa/journey-log.jsonl`
- `/tmp/aghqa-093029b162b3/runtime/sessions/sess-40ede54a94b77e0f/events.db`

## Verified Fixes

### BUG-0029 — first-thread contract discoverability

The native Network descriptor, CLI help/local validation, OpenAPI, protocol, generated TypeScript/CLI references, and bundled AGH skill now expose the exact thread ID grammar and state that the first valid send creates the thread. Real Claude read the corrected CLI help and created `thread_war-room-launch-001`, message `msg-8de42886c15d2879`.

### BUG-0030 — native Network workspace identity

The native adapter had forwarded the durable directory identity where the Network runtime requires the registry identity. The adapter now sends `ResolvedWorkspace.ID`; the Hosted MCP safe error mask remains unchanged. A no-prompt native invocation created `thread_native_probe_002`, message `msg-ad53876f2f7b8673`, with matching public readback and audit evidence.

### BUG-0031 — exact Memory health counts

Memory health reused the prompt-oriented source scan capped at 200. It now uses an uncapped physical-header counter while prompt scan, catalog, and orphan semantics remain independent. The real workspace returned global `4`, workspace `206`, indexed `210`, orphaned `0` after restart.

### BUG-0032 — causal Network message ordering

Protocol timestamps have one-second resolution and message IDs are random, so `(timestamp, message_id)` reordered causally adjacent messages. Append-only GlobalDB v71 adds a durable causal sequence while retaining scoped `message_id` cursors; the pre-rebase QA build carried the same operation as local v68. The real 125-message history preserved old rowids `118..242`; the five-message tail changed from `118,113,109,108,121` to `121,122,123,124,125`. Five `before` pages reconstructed `1..125` exactly once, and `after` message 120 returned `121..125` without a gap or duplicate.

## Cross-Surface Evidence

| Surface | Evidence | Result |
|---|---|---|
| CLI | `bug-0025-native-send-retest.json`; `bug-0027-network-ordering-retest.json`; bounded catalog checks indexed by `frontend-scale-preconditions.json` | Public contracts and causal pages observed |
| API | Exact Memory health and Network history in `bug-0026-memory-health-retest.json` and `bug-0027-network-ordering-retest.json` | Correct values/order observed |
| Web | `network-thread-history-v68.png`, `session-continuity-initial.png`, `session-continuity-reload.png` | Network causal order and stopped-session persistence observed |
| Runtime | pre-rebase v67→v68→v69 upgrade (final v70→v71→v72), exact sequence preservation, provider event history | Durable migration and real provider turn observed |
| Provider | `provider-attempt.json`, session `sess-40ede54a94b77e0f` | One healthy real turn; autonomous continuation absent |

Canonical lab evidence paths:

- `/home/pedronauck/dev/qa-labs/agh-northstar-pay-20260711-153916-425791-lab/qa-artifacts/qa/notes/bug-0025-native-send-retest.json`
- `/home/pedronauck/dev/qa-labs/agh-northstar-pay-20260711-153916-425791-lab/qa-artifacts/qa/notes/bug-0026-memory-health-retest.json`
- `/home/pedronauck/dev/qa-labs/agh-northstar-pay-20260711-153916-425791-lab/qa-artifacts/qa/notes/bug-0027-network-ordering-retest.json`
- `/home/pedronauck/dev/qa-labs/agh-northstar-pay-20260711-153916-425791-lab/qa-artifacts/qa/notes/frontend-continuity-retest.json`
- `/home/pedronauck/dev/qa-labs/agh-northstar-pay-20260711-153916-425791-lab/qa-artifacts/qa/notes/frontend-scale-preconditions.json`
- `/home/pedronauck/dev/qa-labs/agh-northstar-pay-20260711-153916-425791-lab/qa-artifacts/qa/screenshots/live/network-thread-history-v68.png`
- `/home/pedronauck/dev/qa-labs/agh-northstar-pay-20260711-153916-425791-lab/qa-artifacts/qa/screenshots/live/session-continuity-initial.png`
- `/home/pedronauck/dev/qa-labs/agh-northstar-pay-20260711-153916-425791-lab/qa-artifacts/qa/screenshots/live/session-continuity-reload.png`

## Browser and Visual Disclosure

The bootstrap selected `browser-use`. After its local CDP service was reloaded, Chrome required a human “Allow remote debugging” click. No human interaction was supplied. A one-shot Playwright Chromium process was used as the explicit fallback, received the same isolated API proxy target, and was closed in `finally`.

The Network DOM placed labels `121..125` in strictly increasing positions. The session URL remained exactly `/agents/product-manager-agent/sessions/sess-40ede54a94b77e0f` after initial load, navigation to `/tasks`, browser back, and full reload; the `Sofia Mendes` marker appeared exactly once at each checkpoint. Both screenshots truthfully show the session as stopped, so this evidence is not a live-session/SSE verdict.

The blank `tasks-before-workspace.png` capture came from a contaminated pre-workspace harness state and is not evidence.

## Automated Verification

Fresh scoped evidence before report closure:

- `make web-lint`: zero warnings.
- Root Turbo Web typecheck/test: 356 files and 3,033 tests passed.
- `make lint`: zero Go issues.
- `make build`: passed.
- Complete GlobalDB package under the official race runner policy, `CGO_ENABLED=1 go test -race -parallel=4 ./internal/store/globaldb -count=1`: passed in 571.525 seconds before the final rebase, including every historical migration plus the then-local v67/v68/v69 identities (final v70/v71/v72).
- Focused Memory, Network, API, native-tool, store, and Web suites passed under their required race/type/lint lanes.

The authoritative final `make verify` passed against the implementation digest reviewed as `SHIP`. It completed all frontend formatting, lint, typecheck, test, and build lanes; Go codegen, lint, race tests, and build lanes; and the package-boundary audit. The Go lane completed 13,123 tests with two helper-only skips in 2,110.453 seconds. The capacity-sensitive packages also passed under the corrected combined package/subtest budget, including Automation, Daemon, Extension, Heartbeat, Soul, and GlobalDB.

Passing monorepo evidence:

- `/home/pedronauck/dev/qa-labs/agh-northstar-pay-20260711-153916-425791-lab/qa-artifacts/qa/logs/final-make-verify.log`

The earlier capacity-saturated run is retained separately as regression evidence. It led to the root scheduler-budget fix; no test, race detector, assertion, or timeout was weakened:

- `/home/pedronauck/dev/qa-labs/agh-northstar-pay-20260711-153916-425791-lab/qa-artifacts/qa/logs/failed-make-verify-pre-scheduler.log`

## Strict Auditor

The final strict auditor exited 2 with 15 substantive scenario-contract blockers under C6, C8, C10, C11, C16, and C17, plus warning C99. C14 cleared after the passing `make verify` evidence was indexed. Exit 2 therefore describes the incomplete autonomous Northstar Pay playbook, not a branch-verification failure: there were no Task runs, reused declared artifacts, completed disruption probes, required non-Markdown deliverables, peer messages, review cycles, resolved disagreements, or active collaboration channels. The autonomous scenario remains truthfully `BLOCKED`; the four targeted stabilization regressions remain verified.

Audit artifacts:

- `/home/pedronauck/dev/qa-labs/agh-northstar-pay-20260711-153916-425791-lab/qa-artifacts/qa/qa-audit-report.json`
- `/home/pedronauck/dev/qa-labs/agh-northstar-pay-20260711-153916-425791-lab/qa-artifacts/qa/qa-audit-report.md`

The `--api-base-url` invocation also emits warning C99 because the auditor intentionally does not implement API deep equality; direct CLI/API/Web/runtime evidence above owns those assertions.

## Issues

- `BUG-0029` — verified.
- `BUG-0030` — verified.
- `BUG-0031` — verified.
- `BUG-0032` — verified.
- `BUG-0028` — open; blocks the broad autonomous-playbook verdict but is not evidence that the four stabilization fixes regressed.

## Process Hygiene

The manifest teardown stopped daemon PID `1446547`, Vite PID `1314317`, Web Storybook PID `1338948`, and UI Storybook PID `1339719`. A post-teardown port audit found one QA-specific Chrome opened on Storybook at PID `1339187`; it was added to the lab PID registry and the official teardown was repeated. The earlier personal Chrome on debugging port 9222 predates this lab and was left untouched.

Final evidence `/home/pedronauck/dev/qa-labs/agh-northstar-pay-20260711-153916-425791-lab/qa-artifacts/qa/teardown.json` records `clean: true`, no survivors, and all five lab-owned PIDs. Ports 3000, 44473, 6006, and 6007 are no longer listening.

## Final Status

**BLOCKED.** The branch's targeted frontend/runtime regression fixes are verified on real persisted data and real browser surfaces. The Northstar Pay release-grade scenario is not approved because autonomous Task activation, collaboration, deliverables, and disruption recovery did not occur. No evidence was invented, no missing artifact was authored by the observer, and no second provider prompt was used to hide the stall.

[QA_BOOTSTRAP]
manifest_path=/home/pedronauck/dev/qa-labs/agh-northstar-pay-20260711-153916-425791-lab/qa-artifacts/qa/bootstrap-manifest.json
lab_root=/home/pedronauck/dev/qa-labs/agh-northstar-pay-20260711-153916-425791-lab
runtime_home=/tmp/aghqa-093029b162b3/runtime
base_url=http://127.0.0.1:44473
verification_report=/home/pedronauck/Projects/agh/docs/qa/reports/2026-07-11-northstar-pay.md
health_status=fresh
teardown_path=/home/pedronauck/dev/qa-labs/agh-northstar-pay-20260711-153916-425791-lab/qa-artifacts/qa/teardown.json
[/QA_BOOTSTRAP]
