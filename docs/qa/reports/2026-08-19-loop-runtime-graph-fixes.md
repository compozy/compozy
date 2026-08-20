# QA Run Report — 2026-08-19 — loop-runtime-graph-fixes

- **Scope:** Five Loop runtime and graph fixes: total history, daemon-owned output settlement, fan-out filtering, typed runtime input binding, and runtime speed parity.
- **Cadence tier:** targeted
- **Build:** `cacf1de7` · **Environment:** fresh isolated local lab; CLI, HTTP, UDS, native tools, runtime/provider, and Web required
- **Started:** 2026-08-19T23:46:34Z · **Status:** closed
- **Automated precondition:** `make gate` passed at fingerprint `7dee81b3f3a5c69f8fe05a1cf2c1f0c5c974915e` (`.cache/gate/logs/full-1787181048.log`).

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Dora | Power User | desktop / wifi-fast / en-US | CH-compozy-runtime-input-preflight |
| Lea | New User | desktop / wifi-fast / en-US | CH-typed-loop-entity-inputs |
| Bruno | Power User | desktop / wifi-fast / en-US | CH-compozy-mixed-runtime-delivery, CH-loop-graph-runtime-safety |

## Flows in Scope

- `J-02` — Preview a Loop without creating a run or ACP session (`../journeys/J-02-dry-run-preview.md`).
- `J-01` — Run an authored task graph and observe durable runtime truth (`../journeys/J-01-arrive-and-use-run.md`).
- `J-complete-partial-loop` — Author and complete a routed partial Loop (`../journeys/J-complete-partial-loop.md`).

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-compozy-runtime-input-preflight | J-02 / LP-runtime-validation-preflight | Dora | Garbage Tour | Pass | | |
| 2 | CH-typed-loop-entity-inputs | J-01 / LP-select-typed-loop-entities | Lea | Garbage Tour | Pass | | |
| 3 | CH-compozy-mixed-runtime-delivery | J-01 / LP-runtime-provenance-observation | Bruno | Feature Tour | Pass | | |
| 4 | CH-loop-graph-runtime-safety | J-complete-partial-loop / LP-fan-out-filtering; LP-run-agent-output-ownership | Bruno | Feature Tour | Fixed | orphaned next-generation task | folded into Issue 2 |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-compozy-runtime-input-preflight — Dora

- **Ran:** 2026-08-19T23:50:00Z → 2026-08-19T23:55:05Z (box respected: yes)
- **Findings:** Direct runtime input validation and fast-speed dry-run passed across CLI, HTTP, and UDS. Mixed interpolation, missing input, invalid speed, and an unknown provider all failed before persistence or ACP spawn with deterministic diagnostics.
- **Bugs filed/updated:** None.
- **Scenarios settled:** LP-runtime-validation-preflight → pass.
- **Paper cuts:** None.
- **Surprises:** The remote Codex auth probe has no configured status command, but this validation-only session required no provider process and the local provider catalog remained available.
- **Suggested next charter:** CH-typed-loop-entity-inputs.

### CH-typed-loop-entity-inputs — Lea

- **Ran:** 2026-08-19T23:55:35Z → 2026-08-20T00:00:00Z (box respected: yes)
- **Findings:** The Loop run form exposed the exact Codex model, reasoning, and a dedicated Fast switch. The selected fast runtime dry-ran successfully, while an independent runs read proved the preview created no second run. The same runtime object completed a real provider-backed run.
- **Bugs filed/updated:** None.
- **Scenarios settled:** LP-select-typed-loop-entities → pass.
- **Paper cuts:** The first direct deep link opened before workspace scope was chosen and showed “Unable to load loop”; the standard workspace switch recovered the flow. This is existing shell behavior, not a regression in the changed runtime input control.
- **Surprises:** The model catalog is large, but provider filtering and exact model labels remained available.
- **Suggested next charter:** CH-compozy-mixed-runtime-delivery.

### CH-compozy-mixed-runtime-delivery — Bruno

- **Ran:** 2026-08-20T00:01:00Z → 2026-08-20T00:10:00Z (box respected: yes)
- **Findings:** CLI, HTTP, UDS, `compozy__loop_status`, SSE, and Web Inspect reported the same input-owned Codex `gpt-5.4` runtime, medium reasoning, fast speed, and applied outcome. The values survived a daemon restart. A second workspace received 404 for CLI/HTTP/UDS reads, could not resolve the native tool, and received no SSE frame.
- **Bugs filed/updated:** None.
- **Scenarios settled:** LP-runtime-provenance-observation → pass.
- **Paper cuts:** The cross-workspace SSE route opens an empty 200 stream instead of returning 404; it disclosed no run event or payload.
- **Surprises:** The read-only Web inspector exposed both per-field source labels and the original `runtime_applied` event without offering an edit control.
- **Suggested next charter:** CH-loop-graph-runtime-safety.

### CH-loop-graph-runtime-safety — Bruno

- **Ran:** 2026-08-20T00:11:00Z → 2026-08-20T00:57:00Z (box respected: yes)
- **Findings:** Four raw records filtered to two ordered source records, one batch, and one worker lane before `max_fan_out: 1`. Zero-match produced zero lanes. Predicate errors failed by default and exited successfully with `on_eval_error: exit`. A provider-backed worker completed with both schema-required IDs; a 17 KB transform output externalized to SHA-256 and the consumer resolved the identical digest. While its lease was active, heartbeat succeeded and complete/fail were denied by session lineage.
- **Bugs filed/updated:** A failed coordinator left its generation-2 source task `ready`. The terminal and coordinator-failure transactions now cancel every open descendant, including ready-only and `needs_attention` descendants; the same public failure reproduction leaves zero open tasks.
- **Scenarios settled:** LP-fan-out-filtering → pass; LP-run-agent-output-ownership → pass.
- **Paper cuts:** None.
- **Surprises:** Requests for exact 70 KB, 20 KB, and 17 KB agent-authored strings exceeded the provider's practical structured-response behavior. Each attempt failed `invalid_output`; none settled `succeeded`. The successful storage walk therefore separated concerns: live worker ownership used a compact schema, while a deterministic 17 KB transform exercised content-addressed persistence and downstream hydration.
- **Suggested next charter:** Re-run the same ownership probe with another live provider when a second authenticated provider is available.

## What Was Fixed

- Terminal Loop settlement now cancels ready descendants that have no task run, instead of only canceling active task runs.
- Coordinator execution failure now performs the same descendant drain in its failure transaction and publishes the child status transition. Reproduction `looprun-e8aafad8e89ada7d` ended `failed` with its generation-2 task `canceled` and zero open tasks.

## Paper Cuts

- Cross-workspace run SSE accepts a subscription and emits zero bytes rather than rejecting the unknown run. No data crossed the workspace boundary.

## Runtime Errors Observed

- Expected negative paths: mixed runtime interpolation, missing runtime input, invalid speed, unknown provider, default filter predicate failure, and provider responses without a valid JSON object.
- The first oversized-output probe predated the coordinator-failure cleanup fix and retained a draining run in the disposable lab. The post-fix reproduction proves the production path now drains its descendants; teardown removes the lab process envelope.

## Human Verifications Needed

None identified yet.

## Decisions for a Human

None identified yet.

## Learnings

- Runtime provenance is useful only when every surface reports both the selected value and why it won; the Web inspector and SSE frame matched the structured CLI/API payloads.
- Filtering must be observed through the materialized chunks, not inferred from a successful run. The public status payload showed exactly `publish_one` and `publish_three` in one batch.
- A terminal cleanup test must include a ready task without a run and a `needs_attention` run. Testing only a queued worker run missed the orphan found here.

## Experiential Lens Pass

- **J-01, operator-control lens:** Runtime selection remained editable only before start. After start, every surface was read-only and durable across restart, so audit truth could not drift from execution truth.
- **J-complete-partial-loop, interruption/recovery lens:** Invalid output never became success, worker terminal calls could not seize daemon ownership, and coordinator failure drained its next-generation task. Zero-match and predicate-error policies produced explicit, inspectable outcomes.

## Final Status

**Verdict: PASS.** All five flagged scenarios are settled with public-surface evidence.

- QA verification evidence: `/Users/pedronauck/dev/qa-labs/compozy-loop-runtime-graph-fixes-20260819-234724-890442-lab/qa-artifacts/qa/logs/final-make-verify.log`
- The strict evidence audit and mandatory lab teardown run after this report is frozen. The repository-wide `make gate-full` remains the final workstream gate.
