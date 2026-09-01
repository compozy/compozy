# QA Run Report — 2026-08-31 — loop-result-fix

- **Scope:** Durable oversized Loop action results, exact task-run result paging, bounded Web disclosure/copy, spec-cycle path-only task fan-out, bounded native tool-catalog paging, and cancellation cleanup timestamp ordering.
- **Cadence tier:** targeted
- **Build:** working tree before final commit · **Environment:** isolated labs `consumer-saas-growth-20260831-181713-956331` and `tool-list-pagination-20260831-20260831-205804-197783`; real daemon, CLI, HTTP, UDS, native tool, Web, and Codex ACP sessions
- **Started:** 2026-08-31T15:03:15-03:00 · **Completed:** 2026-08-31T18:05:15-03:00 · **Status:** pass

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | Power User | desktop / wifi-fast / en-US | CH-loop-result-budget-paging |
| Cora | Casual User | laptop / wifi-fast / en-US | CH-web-task-result-copy |
| Bruno | Power User | desktop / wifi-fast / en-US | CH-spec-cycle-path-restart |
| Rafa | Power User | desktop / wifi-fast / en-US | CH-artifact-recovery-paging (adjacent canary) |

## Flows in Scope

- `J-complete-partial-loop` — keep large action outputs durable and every public read bounded (`../journeys/J-complete-partial-loop.md`)
- `J-01` — implement an authored task graph from referenced task files across restart (`../journeys/J-01-arrive-and-use-run.md`)
- `J-14` — adjacent canary for the established tool-artifact paging contract (`../journeys/J-14-read-a-finished-transcript.md`)
- `J-agent-marketplace-parity` — enumerate the callable native catalog without exceeding the configured result envelope

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-loop-result-budget-paging | J-complete-partial-loop / LP-oversized-action-result-fails | Ada | Garbage Tour | Pass | | |
| 2 | CH-loop-result-budget-paging | J-complete-partial-loop / TA-task-run-result-paging | Ada | Garbage Tour | Pass | | |
| 3 | CH-web-task-result-copy | J-complete-partial-loop / TA-web-task-result-disclosure | Cora | Feature Tour | Pass | | |
| 4 | CH-spec-cycle-path-restart | J-01 / LP-spec-cycle-path-fanout | Bruno | Interrupt Tour | Pass | | |
| 5 | CH-artifact-recovery-paging | J-14 / ET-tool-result-artifact-recovery | Rafa | Garbage Tour | Skipped | Existing `blocked-verify` retention charter is unchanged; affected artifact regression suites remain the adjacent automated canary. | |
| 6 | Existing ET-035 contract | J-agent-marketplace-parity / ET-035 | Ada | Feature Tour | Pass | | |
| 7 | Existing Loop cancellation contract | J-recover-loop-node-failure / LP-cancel-vs-kill | Bruno | Interrupt Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### Result budget and paging

- A 71,694-byte transform completed with `result: null`, `result_bytes: 71694`, and `result_ref: sha256:97eca41229b21547388b09aa0f26e9a46f7fee61bc2a85daa8cc4169a670da3f`.
- CLI/UDS reconstructed the value from offsets `0`, `16384`, `32768`, `49152`, and `65536`. HTTP and `compozy__task_run_result` returned the same descriptor and 16 KiB page shape.
- After a daemon restart, the same five public CLI reads reconstructed the same SHA-256. An inline 1,944-byte task result returned one valid JSON page with `eof=true`.
- A 307,200-byte transform exceeded the 256 KiB action budget. Its run failed as `action_result_too_large`, named `return_payload` and `transform`, published no result, and retained no lease.
- Invalid offset returned HTTP 400. A missing run returned the masked HTTP 404 shape. Canonical store/service tests covered digest corruption, exact multibyte boundaries, and foreign-workspace masking.

### Web result disclosure and copy

- The task Overview deep link showed a closed 71,694-byte result and made no `/result` request while closed.
- Opening fetched only offset 0. Next rendered bytes 16,385–32,768 in the bounded code viewport. Copy fetched all five pages and changed its accessible label to `Copied result`.
- The browser reported no runtime errors. Durable screenshots and request logs live under the lab's `qa/` directory.

### spec-cycle hard cut and restart

- A public Loop called `ext__spec_cycle__import_tasks` for one authored task. Its 365-byte result contained `path` and `body_ref`, and serialized no `body`.
- The same result remained readable after daemon restart. Bundled-loop and coordinator regression suites own prompt path usage, fan-out ordering, and hydrated continuation.

### Autonomous startup dogfood

- One operator kickoff activated seven Codex agents across four Network channels and 11 durable tasks. No second operator prompt was sent.
- The observer recorded 34 progress transitions across the initial and continuation windows, no stall, matching API/CLI catalogs, and 11/11 completed tasks.

### Native tool catalog paging

- Public `compozy tool invoke compozy__tool_list` returned 275 unique, sorted tools in three inline pages of 100, 100, and 75 entries with stable offsets and no truncation or artifact references.
- The default call matched the first 100-entry page. Every compact row retained the input-schema digest, risk, toolsets, availability, and policy decision; `compozy__tool_info` retained the full schema, backend, and description.
- `limit=101` and `offset=-1` both returned `tool_invalid_input` with `schema_invalid`, never an internal error.

### Cancellation cleanup timestamp ordering

- A CI E2E interleaving persisted a run-owned Goal binding after the Loop cancellation request timestamp. The old cleanup path attempted to write `failed_at` before `created_at`, so SQLite rejected the transaction and the failed teardown polluted the next journey.
- The canonical Goal binding lifecycle suite now forces that timestamp order and verifies both `failed_at` and cleanup `created_at` equal the binding creation time. It passes with the integration tag and race detector.
- The public-daemon Loop read journey then passed with the race detector, including the kill cleanup after IT-027 and the ordered HTTP/UDS/CLI page checks in IT-032.

## What Was Fixed

The first ET-035 walk found that numeric schema bounds were not enforced by the generic native input validator: `limit=101` was accepted and `offset=-1` reached a slice boundary. The handler now validates both fields and returns the typed `tool_invalid_input/schema_invalid` result. The pre-commit cleanup also split two production files that had crossed the repository's 500-line architecture cap. A later CI E2E exposed cancellation-before-binding timestamp ordering; the store now clamps one shared terminal timestamp to the binding creation boundary without weakening the schema constraint.

## Paper Cuts

| Persona | Where (journey/step) | Felt | Sharpness | Outcome |
|---|---|---|---|---|
| Cora | Web result copy | Browser automation could not read the system clipboard because the browser denied read permission. | Low | The UI reported `Copied result`, all five network pages were observed, and the exact single-decode copy invariant remained covered by the canonical hook test. |

## Runtime Errors Observed

- Expected probes only: HTTP 400 for a negative result offset, HTTP 404 for a missing result, `action_result_too_large` for the above-budget action, and typed native input errors for invalid tool-list bounds.
- Web console errors: none.
- The isolated daemon logged background model-catalog warnings for unconfigured non-Codex providers. Those warnings predate and do not traverse the task-result path.
- The intentional restart classified stopped ACP sessions as interrupted while reloading their persisted state; every startup task was already terminal, and result bytes remained readable.

## Human Verifications Needed

None for the changed behavior.

## Decisions for a Human

None.

## Learnings

- The result resource deliberately reuses the byte-page envelope shape, but keeps task-run ownership and lifecycle separate from session tool artifacts.
- The closed Web disclosure is observable at the network boundary: zero result requests while closed, one page on open, and the remaining ordered pages only when the operator copies.
- Restart proof is strongest when the post-restart reader recomputes the original content-addressed digest instead of comparing descriptors alone.
- Catalog enumeration stays robust when its compact list is paged independently from the full descriptor read.

## Taxonomy Coverage

- Journeys: J-complete-partial-loop, J-01, and J-agent-marketplace-parity own the changed value paths; J-14 is the adjacent paging canary.
- Functional: exact bytes, budgets, restart, workspace masking, lease release, and path-only import are in scope.
- Experiential: bounded disclosure, loading/retry/copy feedback, keyboard access, and plain-language failure recovery are in scope.
- Edge/error/empty: no result, invalid range, multibyte boundary, above-budget failure, restart, and failed read are in scope.
- Cross-cutting: CLI/HTTP/UDS/Host/native/Web parity, workspace isolation, and bounded native catalog discovery are in scope; mobile authoring is skipped because the Loop editor is desktop-only.

## Final Status

- **Exit gate (QA walks):** pass; delivery still requires the repository's separate `make gate` and exact-head PR CI contract.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 4/4 changed journeys walked; six changed scenarios pass; the unchanged session tool-artifact retention charter remains `blocked-verify` and was skipped.
- **Verdict:** PASS — changed behavior is ready for the delivery gate.
