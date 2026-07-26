# QA Run Report — 2026-07-19 — hermes-comparison

- **Scope:** Full-program QA cycle for the `hermes-comparison` implementation (US-001..US-016 from
  `.compozy/tasks/hermes-comparison/_user_stories.md` — defects D1–D7, U1, A1, G2, orchestration
  O1–O5, plus the W3 retry consolidation and five-rate pricing companions). Implementation is
  source-final (state.yaml iteration 46, 24/24 criteria, `make verify` PASS 2026-07-19T18:19:27Z).
- **Cadence tier:** full
- **Build:** `295b68990` (branch `hermes-comparison`) · **Environment:** fresh `agh-qa-bootstrap`
  lab — qa-execution fills the machine-readable bootstrap block below before the first session.
- **Started:** 2026-07-19 (planning phase) · **Status:** in-progress

## Bootstrap Manifest

- **Scenario:** `hermes-comparison-consumer-saas-growth-20260719-190252-199062`
- **Playbook:** `consumer-saas-growth` (rotation after the latest `northstar-pay` run)
- **Manifest:** `/Users/pedronauck/dev/qa-labs/agh-hermes-comparison-consumer-saas-growth-20260719-190252-199062-lab/qa-artifacts/qa/bootstrap-manifest.json`
- **Lab root:** `/Users/pedronauck/dev/qa-labs/agh-hermes-comparison-consumer-saas-growth-20260719-190252-199062-lab`
- **Runtime workspace:** `/Users/pedronauck/dev/qa-labs/agh-hermes-comparison-consumer-saas-growth-20260719-190252-199062-lab/project`
- **Runtime home:** `/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/aghqa-a8e82008f022/runtime`
- **UDS:** `/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/aghqa-a8e82008f022/runtime/aghd.sock`
- **HTTP / Web proxy:** `http://127.0.0.1:61527` / `AGH_WEB_API_PROXY_TARGET=http://127.0.0.1:61527`
- **Provider home:** `/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/aghqa-a8e82008f022/provider`
- **Evidence root:** `/Users/pedronauck/dev/qa-labs/agh-hermes-comparison-consumer-saas-growth-20260719-190252-199062-lab/qa-artifacts`
- **Browser:** `browser-use`; **fresh lab:** `reused_lab=false`; **kickoff:** pending

Bootstrap recovery: the first generic lab lacked the playbook required by `real-scenario-qa` and
was never used for product evidence. It was torn down before this lab was created;
`/Users/pedronauck/dev/qa-labs/agh-hermes-comparison-20260719-20260719-190105-273349-lab/qa-artifacts/qa/teardown.json`
records `"clean": true` with no survivors.

## Release-Readiness Criteria (blocking — stated up front, per COPY.md claim standards)

The program closes as **ready** only when ALL of the following hold. `make verify` green is
necessary but NOT sufficient (SD-005); these scenario walks are what closes the program.

1. **Coverage:** every journey in scope below walked by its assigned persona; every scenario in the
   matrix settled to a terminal `qa_status` (`pass` / `blocked-verify` / `skipped`-with-reasoning).
   Every verdict carries SD-006 forensic evidence: timestamp + exact command + observed output.
2. **Zero unfiled reds:** every red observation registers a deduped content-addressed
   `docs/qa/bugs/BUG-20260719-symptom-slug.md` with forensic reproduction. A red scenario is a
   production bug, never a test to weaken.
3. **Blocking bugs:** any open Blocks-Completion or Data-Loss bug on these scenarios blocks
   release. Several Trust-Damage findings on one journey also block. Specifically blocking by
   construction (safety invariants): an auto-approve on grant-store error (SI-3), a raw planted
   secret in any durable store or stream (SI-10/11), a fake `$` for included/unknown cost (SI-13),
   dropped or rejected queued work at the capacity cap (SI-26), a persistable daemon-lifecycle
   schedule (SI-17), and silent context loss on compacted-then-resumed sessions (SI-8).
4. **E2E directive:** `make test-e2e-runtime` AND `make test-e2e-web` green in the lab; the
   highest-risk UI flows (approval grants view, clarify card, suggestions card, cost badges)
   driven through the browser; CLI/HTTP/UDS/native parity exercised on agent-manageable surfaces.
5. **Teardown (L-029):** every terminal path ends with `eval "$TEARDOWN_COMMAND"` or
   `make qa-reap`; `teardown.json` cited with `"clean": true`; lab pids registered under
   the bootstrap QA output path's `qa/pids/` directory.
6. **Final Status** must distinguish "verified in lab" from "covered by unit/integration only" per
   scenario — no conflation.

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Théo | Power User | desktop / wifi-fast / en-US | CH-crash-resume-compaction, CH-approval-grant-memory, CH-clarify-answer-roundtrip, CH-session-affordances-truth |
| Ada | Power User (non-human, native-tool) | desktop / wifi-fast / en-US | CH-runaway-work-bounded, CH-workspace-run-capacity, CH-subprocess-health-recovery, CH-runnable-capabilities-truth, CH-mcp-client-operates-agh, CH-memory-batch-integrity, CH-wake-dedup-stress |
| Bruno | Power User | desktop / wifi-fast / en-US | CH-schedule-recovery-guard, CH-suggestions-consent |
| Dora | Power User (runtime administrator — added to personas.md this cycle) | desktop / wifi-fast / en-US | CH-secret-redaction-sweep, CH-drain-without-loss |
| Rafa | Casual User | desktop / wifi-fast / en-US | CH-truthful-cost-provenance, CH-artifact-recovery-paging |
| Omar | Power User | desktop / wifi-fast / en-US | CH-bridge-overload-taxonomy |

Story-persona mapping (recorded reconciliation): _user_stories.md "Operator" → Théo (session
surface) / Bruno (automation surface); "Autonomy operator" → Bruno (authoring) and Ada
(kernel/structured); "Administrator" → Dora (new persona, rationale in `docs/qa/personas.md`);
"Managed agent" and "External integrator" → Ada (the external MCP client is Ada driving a
third-party client instead of native tools).

## Flows in Scope

New this cycle (each with Mermaid flow, true end state, and ≥1 abandonment path):

- `J-answer-agent-requests` — a decision answered once is remembered, revocable, never re-asked (`../journeys/J-answer-agent-requests.md`)
- `J-drain-daemon-safely` — restart or deploy without killing work (`../journeys/J-drain-daemon-safely.md`)
- `J-keep-secrets-contained` — a leaked secret never reaches disk or stream (`../journeys/J-keep-secrets-contained.md`)
- `J-operate-agh-from-mcp-client` — operate AGH from an unmodified MCP client (`../journeys/J-operate-agh-from-mcp-client.md`)
- `J-offer-runnable-capabilities` — only runnable capabilities offered; dead ones recover (`../journeys/J-offer-runnable-capabilities.md`)
- `J-bound-runaway-work` — runaway or wedged work is bounded and explained (`../journeys/J-bound-runaway-work.md`)

Planted by implementation, reconciled this cycle:

- `J-operate-bounded-task-capacity` — work within workspace capacity (`../journeys/J-operate-bounded-task-capacity.md`)
- `J-diagnose-task-session-health` — diagnose and recover a task session (`../journeys/J-diagnose-task-session-health.md`)

Existing journeys reused (flows already present):

- `J-11` return-to-running-session · `J-14` read-a-finished-transcript · `J-24`
  triage-work-at-scale · `J-20` catalog curation (companion) · `J-connect-bridge-provider`
  (companion, retry taxonomy)

## Session Matrix & Results

Risk-ordered: highest-impact journey × highest-blast-radius tour first. All rows Pending —
qa-execution updates this table in place.

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-secret-redaction-sweep | J-keep-secrets-contained / RT-secret-redaction-boundary | Dora | Garbage | Skipped | No public secret-injection fixture was available without a second provider prompt. | |
| 2 | CH-crash-resume-compaction | J-11 / RT-session-context-rebuild; RT-pressure-context-compaction; MS-workspace-checkpoint-continuity | Théo | Interrupt | Skipped | The active one-kickoff lab exposed no public mid-turn crash/compaction fixture. | |
| 3 | CH-approval-grant-memory | J-answer-agent-requests / ET-native-tool-approval-grants | Théo | Interrupt | Skipped | No live approval request existed; creating one required another provider prompt. | |
| 4 | CH-runaway-work-bounded | J-bound-runaway-work / TA-lease-recovery-attempt-budget; TA-action-run-liveness; TA-loop-failure-breaker; TA-exact-claim-single-owner | Ada | Garbage | Skipped | Exact-claim and breaker fault injection was unavailable through the public lab surfaces. | |
| 5 | CH-workspace-run-capacity (existing, re-run) | J-operate-bounded-task-capacity / TA-workspace-run-capacity | Ada | Feature | Skipped | The declared 11-run playbook stayed below the configured workspace cap. | |
| 6 | CH-schedule-recovery-guard | J-24 / TA-schedule-catchup-overlap; TA-daemon-lifecycle-command-guard; TA-055 | Bruno | Interrupt | Skipped | No schedule/restart fixture was configured in this playbook. | |
| 7 | CH-drain-without-loss | J-drain-daemon-safely / RT-daemon-drain-admission; MS-daemon-memory-reporting; RT-002 | Dora | Interrupt | Skipped | Draining the sole daemon would terminate the active observer envelope. | |
| 8 | CH-clarify-answer-roundtrip | J-answer-agent-requests / RT-session-clarification-roundtrip | Théo | Feature | Skipped | No pending clarification was produced by the single provider kickoff. | |
| 9 | CH-truthful-cost-provenance | J-14 / RT-session-cost-provenance; TA-task-run-cost-provenance; ET-model-source-five-rate-pricing; MS-042; MS-045; MS-055; MS-056; ET-053 | Rafa | Money | Skipped | CLI proved the live session as `included`, but the lab could not drive the required actual/estimated variants or full Web matrix. | |
| 10 | CH-suggestions-consent | J-24 / TA-automation-suggestions | Bruno | Feature | Pass | Dismissed `sugcat_61e4df77633b613a`; status became `dismissed` and Job count remained zero. | |
| 11 | CH-subprocess-health-recovery | J-diagnose-task-session-health / RT-subprocess-health-escalation | Ada | Feature | Skipped | No controlled provider-crash fixture was available through a public command. | |
| 12 | CH-runnable-capabilities-truth | J-offer-runnable-capabilities / ET-skill-activation-gates; RT-mcp-dead-recovery; ET-001; ET-002 | Ada | Feature | Skipped | The lab had no disposable MCP target or skill-gate fixture to kill and recover. | |
| 13 | CH-mcp-client-operates-agh | J-operate-agh-from-mcp-client / ET-workspace-host-api-mcp | Ada | Feature | Skipped | `agh mcp serve` was available, but bootstrap provided no registered external MCP client. | |
| 14 | CH-artifact-recovery-paging | J-14 / ET-tool-result-artifact-recovery | Rafa | Garbage | Skipped | No live tool result crossed the retention/offload threshold in this playbook. | |
| 15 | CH-session-affordances-truth | J-11 / RT-session-lifecycle-affordances; RT-session-cwd-resume | Théo | Feature | Skipped | Exact CWD/resume/title assertions lacked a public deterministic fixture in the running lab. | |
| 16 | CH-memory-batch-integrity | J-11 / MS-atomic-memory-batch | Ada | Garbage | Skipped | No public trace surface exposed the required generation-scoped batch injection. | |
| 17 | CH-wake-dedup-stress | J-operate-bounded-task-capacity / TA-task-wake-dedup | Ada | Garbage | Skipped | Wake replay/dedup injection was not exposed by the playbook's public surfaces. | |
| 18 | CH-bridge-overload-taxonomy | J-connect-bridge-provider / NB-bridge-overload-recovery | Omar | Network | Skipped | No disposable overloaded bridge was configured in the isolated lab. | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Story → Scenario → Journey → Charter Matrix

| Story | Verifies | Scenario(s) | Journey | Charter (row) |
|---|---|---|---|---|
| US-001 | D1, ADR-001 | ET-native-tool-approval-grants | J-answer-agent-requests | CH-approval-grant-memory (3) |
| US-002 | D7, ADR-001 | RT-session-clarification-roundtrip | J-answer-agent-requests | CH-clarify-answer-roundtrip (8) |
| US-003 | D4, ADR-002 | RT-session-context-rebuild (+D6 page-back via ET-tool-result-artifact-recovery) | J-11 | CH-crash-resume-compaction (2) |
| US-004 | D3, ADR-003, B-301 | RT-pressure-context-compaction; MS-workspace-checkpoint-continuity; MS-atomic-memory-batch; RT-session-lifecycle-affordances | J-11 | CH-crash-resume-compaction (2), CH-memory-batch-integrity (16), CH-session-affordances-truth (15) |
| US-005 | D2, ADR-007/ADR-010 | TA-schedule-catchup-overlap; TA-daemon-lifecycle-command-guard (+TA-055) | J-24 | CH-schedule-recovery-guard (6) |
| US-006 | ADR-010 | RT-daemon-drain-admission | J-drain-daemon-safely | CH-drain-without-loss (7) |
| US-007 | U1, ADR-006 | RT-session-cost-provenance; TA-task-run-cost-provenance (+five-rate companions) | J-14 / J-24 | CH-truthful-cost-provenance (9) |
| US-008 | A1, ADR-007 | TA-automation-suggestions | J-24 | CH-suggestions-consent (10) |
| US-009 | G2, ADR-005 | RT-secret-redaction-boundary | J-keep-secrets-contained | CH-secret-redaction-sweep (1) |
| US-010 | ADR-008 | ET-workspace-host-api-mcp | J-operate-agh-from-mcp-client | CH-mcp-client-operates-agh (13) |
| US-011 | ADR-009 §2, ADR-010 §5 | ET-skill-activation-gates (skills half); RT-mcp-dead-recovery (dead-entity half) | J-offer-runnable-capabilities | CH-runnable-capabilities-truth (12) |
| US-012 | O1, §3.10 | TA-lease-recovery-attempt-budget (minted this cycle) | J-bound-runaway-work | CH-runaway-work-bounded (4) |
| US-013 | O2, §3.10 | TA-loop-failure-breaker | J-bound-runaway-work | CH-runaway-work-bounded (4) |
| US-014 | O3, §3.10 | TA-exact-claim-single-owner (minted this cycle) | J-bound-runaway-work | CH-runaway-work-bounded (4) |
| US-015 | O4, §3.10 | TA-action-run-liveness | J-bound-runaway-work | CH-runaway-work-bounded (4) |
| US-016 | O5, §3.10 | TA-workspace-run-capacity (AC-1..3); TA-task-wake-dedup (EC-1) | J-operate-bounded-task-capacity | CH-workspace-run-capacity (5), CH-wake-dedup-stress (17) |
| D5 (no US) | ADR-010 §4, ADR-011 | RT-subprocess-health-escalation (+MS-daemon-memory-reporting for the §3.5 memory probe) | J-diagnose-task-session-health / J-drain-daemon-safely | CH-subprocess-health-recovery (11), CH-drain-without-loss (7) |
| D6 (no US) | §3.1 offload | ET-tool-result-artifact-recovery | J-14 | CH-artifact-recovery-paging (14) |
| W3 retry (gated, shipped) | ADR-010 §6 | NB-bridge-overload-recovery | J-connect-bridge-provider | CH-bridge-overload-taxonomy (18) |

## Coverage Map — defects, gaps, and active ADRs

Every item is verified by ≥1 story/scenario above. ADR-004 is deferred (no scenario, by design);
ADR-011/ADR-012 are implementation ADRs verified inside their owning rows.

| Item | Verified by |
|---|---|
| D1 (allow_always lie) | US-001 → ET-native-tool-approval-grants |
| D2 (silent schedule skip) | US-005 → TA-schedule-catchup-overlap |
| D3 (dead compaction) | US-004 → RT-pressure-context-compaction |
| D4 (silent resume loss) | US-003 → RT-session-context-rebuild |
| D5 (health without action) | RT-subprocess-health-escalation (+ memory probe MS-daemon-memory-reporting) |
| D6 (unrecoverable overflow) | ET-tool-result-artifact-recovery |
| D7 (RequiresInteraction dead-end) | US-002 → RT-session-clarification-roundtrip |
| U1 (cost never estimated) | US-007 → RT-session-cost-provenance + TA-task-run-cost-provenance |
| A1 (no automation authoring) | US-008 → TA-automation-suggestions |
| G2 (secrets unredacted) | US-009 → RT-secret-redaction-boundary |
| O1 (unbounded crash loops) | US-012 → TA-lease-recovery-attempt-budget |
| O2 (sibling-reset breaker) | US-013 → TA-loop-failure-breaker |
| O3 (double manual claim) | US-014 → TA-exact-claim-single-owner |
| O4 (wedged heartbeaters) | US-015 → TA-action-run-liveness |
| O5 (saturation drops) | US-016 → TA-workspace-run-capacity + TA-task-wake-dedup |
| ADR-001 (grants + clarify) | US-001, US-002 |
| ADR-002 (replay fallback) | US-003 |
| ADR-003 (compaction + summaries) | US-004 |
| ADR-005 (redaction + SECURITY.md) | US-009 |
| ADR-006 (cost provenance) | US-007 |
| ADR-007 (suggestions, no blueprints) | US-005, US-008 |
| ADR-008 (mcp serve) | US-010 |
| ADR-009 §2 (when.* gates) | US-011 (skills half) |
| ADR-010 (reliability batch) | US-005 (guard), US-006 (drain), US-011 (dead entities), D5 (health), memory probe, W3 retry |
| ADR-011 (crash fixture / late consumer) | RT-subprocess-health-escalation walk (row 11) |
| ADR-012 (cost aggregate row) | RT-session-cost-provenance Web capture (row 9) |
| ADR-004 | Deferred 2026-07-14 — deliberately NOT covered; no scenario exists (zero-legacy: no inert coverage) |

## Reconciliation & Duplicate Folds

- **One distinct scenario row per story — 16/16.** The QA Execution Contract requires one tracker
  row per story US-001..US-016; the story matrix above maps each story to its own primary scenario
  file (companions ride the same charter rows without substituting for a story's primary).
- **Fold reversed (corrected 2026-07-19):** an initial planning fold put US-012 and US-015 on one
  file (`TA-action-run-liveness`). `_tests.md` assigns O1 and O4 to distinct invariant owners
  (UT-102–104/IT-035 vs UT-105–110/IT-038), so the fold was reversed: US-012 now owns the minted
  `TA-lease-recovery-attempt-budget` and `TA-action-run-liveness` is scoped to US-015 only, with
  the shared-budget coupling (UT-109) cross-linked via `overlaps`. Both share the risk-grouped
  charter CH-runaway-work-bounded (row 4), which is structurally valid — one journey, one tour,
  one persona.
- **Mints (2):** `TA-exact-claim-single-owner` (US-014) — no existing scenario owned the contested
  exact-claim invariant (`TA-workspace-run-capacity` owns the capacity race, not same-RunID
  ownership); `TA-lease-recovery-attempt-budget` (US-012) — per the reversed fold above.
- **Area-code note:** the QA Execution Contract hinted areas RT (US-001..004), TA (US-005/008/
  012..016), ET (US-010..011), MS (US-006..007/009). Implementation planted some flags under
  different existing codes (US-001 → ET, US-006/009 → RT, US-007 → RT+TA, memory probe → MS). Ids
  are content-addressed and never renamed (state-schema rule); the planted ids win and this
  divergence is recorded here instead of renaming. All ids stay within the README's registered
  area codes; no new area was minted.
- **No same-behavior/same-symptom duplicate pairs found** among the 30 planted + 2 minted scenario
  files (checked against `overlaps` fields and the untested inventory); no fold-delete was needed.
  `MS-workspace-checkpoint-continuity` is confirmed in scope: it covers US-004's checkpoint
  summaries (ADR-003 §2), not the deferred ADR-004 workspace checkpoints.
- **Persona normalization:** ad-hoc frontmatter names (`Operator/Agent`, `Runtime operator`,
  `Administrator`, `Autonomy operator`, `Agent`, `Tessa and Omar`) normalized to canonical
  personas (Théo/Ada/Bruno/Dora/Omar). `Dora` added to `personas.md` (runtime-administrator
  audience introduced by this program).
- **Journey links filled:** 8 planted scenarios had empty `journey:`; all now link an existing or
  new journey. `J-24-triage-work-at-scale` references normalized to the canonical `J-24` id.
- **Charter reuse:** `CH-workspace-run-capacity` (planted by implementation) re-run as-is —
  charters are immutable; no sibling was written. No other existing charter mission covered the
  new scenarios (checked CH-bridge-progress-stress, CH-long-provider-replies,
  CH-mid-turn-bridge-restart before writing CH-bridge-overload-taxonomy).

## Open-Bug Routing

Registry census at planning time: 1 `open`, 16 `fixed` (awaiting retest), 90 `verified`.

- `BUG-20260713-telegram-route-shapes` (open, Friction/P2, bridge routing-shape contract): OUT of
  hermes-comparison scope — channels/bridges/communications are excluded by the TechSpec (§1
  non-goals). Remains owned by the bridge/network workstream; not blocking this program's
  release-readiness verdict.
- The 16 `fixed` bugs link only scenarios owned by other programs (Loops, Marketplace, Network,
  Goal, model-selector); none references any scenario in this cycle's matrix (verified by grep —
  zero hits for all planted ids across `docs/qa/bugs/`). Their retests belong to their owning
  cycles' targeted tiers, not this one.
- Any new red found by this cycle files a content-addressed `BUG-20260719-symptom-slug` after
  symptom-dedup against the registry.

## Taxonomy Sweep (five dimensions per journey — deliberate skips recorded)

- **Journeys:** primary dimension — every matrix row walks entry → true end state with
  abandonment/resume paths mapped in each journey file.
- **Functional:** carried inside every scenario's expected observable + SD-006 forensic contract
  (parity across CLI/HTTP/UDS/native/Web is the recurring functional check).
- **Experiential:** clarify card keyboard/refresh sweep (row 8), cost badge/full-width provenance
  row (row 9, ADR-012), suggestions card (row 10), artifact viewer states (row 14), title/verifier
  timeline (row 15) — each requires `agh-ui-screenshot` captures. **Deliberate skip:** a dedicated
  Sol (screen-reader) session — this cycle's new UI is limited to cards/badges inside surfaces
  whose a11y contract is owned by the session-improvements J-13/CH-020 lens; the clarify card
  keyboard path is covered in row 8. Recorded as a skip, not coverage.
- **Edge/error/empty:** owned by the tour mix — Garbage (rows 1, 4, 14, 16, 17), Interrupt
  (rows 2, 3, 6, 7), Network (row 18); crash windows, timeout sentinels, cap races, and injected
  failures are explicit must_try items.
- **Cross-cutting:** regression canaries = the 9 reset companion scenarios folded into rows 6, 7,
  9, 12 (TA-055, RT-002, five-rate set, ET-001/002); isolation probes (workspace A/B) appear in
  rows 1, 2, 4, 5, 10, 12, 13, 14, 16, 17. **Deliberate skip:** responsive 375/768/1280 sweeps —
  no new layout-bearing surface beyond the cards captured above; structured-only journeys record
  "not applicable" in their taxonomy notes.

## Planning Validation (this phase)

- Frontmatter validation + tracker view (absolute paths — the canonical form for this tree):
  `rtk python3 /Users/pedronauck/Dev/compozy/agh3/.agents/skills/qa-report/scripts/materialize_state.py /Users/pedronauck/Dev/compozy/agh3/docs/qa`
  → `docs/qa/state.csv: 544 scenarios`, zero rejections (state.csv is generated and gitignored).
- Inventory commands (recursive form — see recovery note in task memory):
  `rtk grep -rl "qa_status: untested" docs/qa/scenarios/`,
  `rtk grep -rh '\*\*Status:\*\*' docs/qa/bugs/ | sort | uniq -c`.

## Session Debriefs

### Real-scenario companion attempt 1 — `consumer-saas-growth` — FAIL

- One evidenced Priya kickoff released 11 deterministic runs across seven registered agents and
  four channels; no follow-up provider prompt was sent.
- Public Task state at the end of the window showed 10/11 declared tasks completed. The remaining
  lifecycle-email task returned to `ready` after its run failed: its owner prompt required Product
  Design review on `design-review`, while the immutable run channel and declared reviewer were
  `lifecycle-cadence` / Lifecycle Marketing. The playbook source now assigns the correct reviewer
  per surface and validates cleanly.
- The runtime observer stopped receiving rows after 14 bootstrap/controller actions and diagnosed
  all eleven tasks unstarted despite the public Task state above. This is filed as
  `BUG-20260719-autonomous-progress-unobservable` and linked to `RT-073`.
- Strict audit verdict: **FAIL**, 10 blockers (C7/C8/C9/C10/C11/C12/C14 plus three C17
  collaboration minimums). Deliverable parsing passed with 18 valid non-Markdown artifacts; Task
  root/run minimums passed.
- Audit:
  `/Users/pedronauck/dev/qa-labs/agh-hermes-comparison-consumer-saas-growth-20260719-190252-199062-lab/qa-artifacts/qa/qa-audit-report.json`.
- Mandatory teardown:
  `/Users/pedronauck/dev/qa-labs/agh-hermes-comparison-consumer-saas-growth-20260719-190252-199062-lab/qa-artifacts/qa/teardown.json`
  records `clean: true` and zero survivors.

### Real-scenario companion attempt 2 — `consumer-saas-growth` — FAIL

- One live Codex kickoff in `sess-e6c733a3850795e5` produced a provider decision to keep exposure
  on HOLD until `first_save` emitted exactly once and Data Science refreshed event volume. No
  additional provider prompt was sent.
- Eight of 11 declared tasks completed. All three Analytics Engineering-owned runs remained
  unclaimed and reached `needs_attention` after ten convergence cycles even though
  `sess-8a62120da5a4dd11` remained active.
- All three scheduled disruptions were delivered through their declared public/runtime-compatible
  surfaces. The knowledge file, Network message, and Task event were persisted, but none produced
  the expected post-trigger recovery within its deadline.
- The observer ended with `stall_detected=true` and again classified ten completed/terminal tasks
  as unstarted because runtime lifecycle progress was absent from the journey log. The fresh
  reproduction is appended to `BUG-20260719-autonomous-progress-unobservable`.
- The suggestions-consent journey passed: dismissing one pending suggestion created no Automation
  Job. Session usage also truthfully reported `cost_status=included`, but the full cost matrix was
  not reachable in this playbook.
- Observer evidence:
  `/Users/pedronauck/dev/qa-labs/agh-hermes-comparison-consumer-saas-growth-r2-20260719-202601-971723-lab/qa-artifacts/qa/observer-result.json`.
- Strict evidence audit: **PASS**, zero blockers and zero warnings;
  `/Users/pedronauck/dev/qa-labs/agh-hermes-comparison-consumer-saas-growth-r2-20260719-202601-971723-lab/qa-artifacts/qa/qa-audit-report.json`.
- Mandatory teardown:
  `/Users/pedronauck/dev/qa-labs/agh-hermes-comparison-consumer-saas-growth-r2-20260719-202601-971723-lab/qa-artifacts/qa/teardown.json`
  records `clean: true` and zero survivors.

## What Was Fixed

- CLI Task-run and Task-inspect TOON/human projections now serialize numeric `RunStatus` values
  through their semantic `String()` contract. The canonical regression failed before the fix and
  passes afterward; the bounded complete CLI race lane passes in 109.077s. A rebuilt CLI against
  the live lab rendered `failed` and `completed` for the exact affected runs.
- The `consumer-saas-growth` Frontend Engineer and Lifecycle Marketer prompts now agree with the
  task-declared reviewers and immutable channels for landing variants versus the lifecycle email.

## Paper Cuts

| Persona | Where (journey/step) | Felt | Sharpness | Outcome |
|---|---|---|---|---|

## Runtime Errors Observed

- `task run list -o toon` rendered run statuses as control characters before the CLI projection
  correction (`completed=\u0005`, `failed=\u0006`).
- The operator's bounded Network wake ended after exactly five minutes with durable
  `prompt_cancel` and `provider_failure` markers. The bounded cancellation matches
  `max_wake_wall_time=5m`; the Web marker wording "canceled by operator" is misleading because no
  second operator action occurred.
- The lifecycle-email run failed with a required-review/channel mismatch from the playbook prompt;
  the produced TSX artifact itself was present and valid.
- `observe-runtime.py` reported a stall because no runtime-owned progress reached the journey log;
  the independent Task read showed autonomous completion had continued.
- Attempt 2 reproduced the same observer stall. It also left the three runs owned by active
  Analytics Engineering session `sess-8a62120da5a4dd11` in `needs_attention`; all three expected
  disruption recoveries expired without a post-trigger agent verdict.

## Human Verifications Needed

## Decisions for a Human

## Learnings

- (planning) Implementation-planted QA flags carried ad-hoc persona names and empty journey links;
  reconciliation — not re-minting — closed the gap without breaking any content-addressed id.

## Final Status

- **Exit gate (full automated suite):** **PASS** — source-final `make verify` exited 0; evidence:
  `/Users/pedronauck/dev/qa-labs/agh-hermes-comparison-consumer-saas-growth-r2-20260719-202601-971723-lab/qa-artifacts/qa/logs/final-make-verify.log`.
- **Issues by user impact:** one open High/P1 Blocks-Completion issue,
  `BUG-20260719-autonomous-progress-unobservable`; attempt 2 also left three runs owned by an
  active session in `needs_attention` and missed every disruption-recovery deadline.
- **Coverage:** 1/18 planned charter sessions passed · 17/18 skipped with explicit public-fixture
  reasons · behavioral companion completed 8/11 declared tasks · 3/3 probes delivered · 0/3
  expected recoveries observed.
- **Verdict:** **FAIL** — the automated monorepo gate passes, but the fresh behavioral run does not
  meet the autonomous collaboration/recovery contract. Attempt-2 teardown is clean with zero
  surviving processes.
