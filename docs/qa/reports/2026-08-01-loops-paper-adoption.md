# QA Run Report — 2026-08-01 — Loops Paper Adoption

- **Scope:** Metric ratchet, rejected-gate succession, repair context, generation provenance, breaker semantics, Loop config files, and persisted run-detail truth across CLI, HTTP, UDS, native tools, SSE, Web, docs, and the official skill.
- **Cadence tier:** targeted, with adjacent safety checks, one destructive Loop CRUD canary, and one rotated Northstar Pay real-scenario observation.
- **Build:** final post-rebase branch content, including the completed deep-review remediation and
  runtime E2E reconciliation.
- **Environment:** fresh isolated northstar-pay-20260801-135009-390014 lab plus the post-rebase
  `/tmp/compozy-loops-postrebase.N1iPVr` runtime; daemon targets 60717 and 61336 respectively.
- **Started:** 2026-08-01T13:49:19Z · **Status:** PASS — QA walks, evidence indexing, full verification, and teardown complete.

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | Operator | desktop / wifi-fast / en-US | repair succession and runaway-work safety |
| Bruno | Delivery operator | desktop / wifi-fast / en-US | ratchet truth, deep link, config files, destructive CRUD canary |
| Cora | Non-technical owner | desktop / wifi-fast / en-US | plain-language run outcome |
| Sofia Mendes | Northstar Pay Founder/PM | desktop / wifi-fast / en-US | one-kickoff autonomous observation |

## Flows in Scope

- J-improve-loop-with-feedback — reject, repair, improve, restore, and inspect a bounded Loop.
- J-bound-runaway-work — prove action liveness, lease recovery, exact claims, and breakers remain bounded.
- J-01 — follow one exact run deep link and compare durable truth across public surfaces.
- J-06 — delete one workspace Loop through intentional confirmation while preserving built-ins.
- northstar-pay — observe one autonomous multi-workspace launch after exactly one provider kickoff.

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-loop-repair-succession | LP-revise-repair-context | Ada | Interrupt Tour | Pass | | |
| 2 | CH-loop-repair-succession | LP-dod-reject-retry | Ada | Interrupt Tour | Pass | | |
| 3 | CH-loop-ratchet-truth | LP-ratchet-climb-restore | Bruno | Feature Tour | Pass | | |
| 4 | CH-runaway-work-bounded | TA-loop-failure-breaker | Ada | Garbage Tour | Pass | | |
| 5 | CH-runaway-work-bounded | TA-lease-recovery-attempt-budget | Ada | Garbage Tour | Pass | | |
| 6 | CH-runaway-work-bounded | TA-action-run-liveness | Ada | Garbage Tour | Pass | | |
| 7 | CH-runaway-work-bounded | TA-exact-claim-single-owner | Ada | Garbage Tour | Pass | | |
| 8 | CH-compozy-run-plain-language | LP-loop-run-deep-link | Cora | Feature Tour | Pass | | |
| 9 | CH-compozy-run-plain-language | LP-runtime-provenance-observation | Cora | Feature Tour | Pass | | |
| 10 | CH-loop-goal-delete | LP-toggle-loop-goal | Bruno | Feature Tour | Skipped | The planned single adjacent canary used the destructive delete branch. | |
| 11 | CH-loop-goal-delete | LP-delete-custom-loop | Bruno | Feature Tour | Pass | | |
| 12 | northstar-pay behavioral charter | Autonomous launch coordination | Sofia Mendes | Scenario playbook | Blocked (human decision) | BUG-20260719-autonomous-progress-unobservable remains open; runtime work itself progressed. | |
| 13 | CLI config-file regression | LP-loop-config-file-snake-case | Bruno | Feature Tour | Fixed | BUG-20260801-loop-config-file-snake-case | 38b2d40 |
| 14 | CH-untested-013-13-bruno | TA-071 | Bruno | Network Tour | Pass | | |
| 15 | CH-extension-dev-link-invoke | ET-extension-dev-reload-loop | Bruno | Feature Tour | Fixed | BUG-20260801-extension-cli-workspace-reads | 98feabf |

Status legend: Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)

## Session Debriefs

### Feedback, ratchet, and repair

The official runtime E2E lane passed the canonical ratchet climb, regression restore, rejected
definition-of-done repair, producer-scoped revise, max-revision, SSE replay, restart, and workspace
isolation cases. The official Web lane passed the exhausted-run detail contract.

The manual run looprun-f3ef2b188e60580b then proved the same persisted object end to end:

- terminal status exhausted at generation 3;
- best generation 1 with score 0.70;
- generation 2 from stop_when scored 0.60;
- generation 3 used ratchet_restore with parent generation 1 and score 0.50;
- CLI, HTTP, UDS, and compozy__loop_status payloads were structurally identical;
- SSE replay retained generation origin, parent, gate verdict, and terminal cause;
- Web rendered Exhausted, Best result · Gen 1 · 0.70, 3/3 rounds, and the matching digest.

Evidence:
- /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260801-135009-390014-lab/qa-artifacts/qa/official-e2e-results.json
- /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260801-135009-390014-lab/qa-artifacts/qa/loop/same-run-parity-v6.json
- /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260801-135009-390014-lab/qa-artifacts/qa/loop/sse-events-v6.txt
- /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260801-135009-390014-lab/qa-artifacts/qa/loop/loop-run-v6-full.png

### Safety and liveness controls

Fresh race-enabled targeted checks passed for:

- a wedged action terminalizing and releasing the scheduler;
- persistent node failure stalling with circuit_breaker despite a successful sibling;
- repeated expired-lease recovery exhausting to needs_attention with lease_recovery_exhausted;
- contested claims producing exactly one winner and exact-target retries refusing fallback.

Evidence:
- /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260801-135009-390014-lab/qa-artifacts/qa/loop/adjacent-safety-tests.json

### Public-surface parity and isolation

The owner workspace ws_81fca708554cea13 returned the same run through structured CLI, HTTP, UDS,
compozy__loop_status, and compozy__loop_runs. Neighbor ws_e3479b1e0da923f2 received 404/not-found
results over HTTP, UDS, and CLI; the native registry denied the foreign invocation without exposing
run data. The browser deep link initially demonstrated the expected active-workspace boundary, then
opened the exact run after selecting loop-qa.

Evidence:
- /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260801-135009-390014-lab/qa-artifacts/qa/loop/workspace-isolation-v6.json
- /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260801-135009-390014-lab/qa-artifacts/qa/loop/browser-url-v6.txt

### Destructive CRUD canary

A disposable workspace Loop named qa-delete-canary was published and opened in Web. An incorrect
confirmation kept Delete disabled; the exact name enabled it. Deletion removed only the workspace
definition, redirected to the catalog, preserved the two read-only built-ins, and produced no browser
errors.

Evidence:
- /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260801-135009-390014-lab/qa-artifacts/qa/loop/delete-canary-result.json
- /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260801-135009-390014-lab/qa-artifacts/qa/loop/delete-canary-confirmation.png

### Gate-verdict SSE redaction and reconnect

The public run `looprun-1acce5b114ce72d5` intentionally failed its definition-of-done command.
Its retained verdict and `output_ref` carried `compozy_claim_[REDACTED]`, while the raw claim token
was absent. Reconnecting with `after_sequence=1` returned the ordered retained events with the same
gate identity and sanitized diagnostic, proving that the new useful output survives both
persistence and resume without weakening the redaction boundary.

Evidence:
- `docs/qa/scenarios/TA-071.md`
- `/tmp/compozy-loops-postrebase.N1iPVr/teardown.json`

### Workspace extension development loop

Bruno linked `post-rebase-probe` to workspace `ws_44d9a2ed896b6e2e`. The first public CLI pass
exposed a missing management surface: `extension list/status` could not select the workspace dev
overlay even though HTTP/UDS already supported the scope. BUG-20260801-extension-cli-workspace-reads
was fixed in `98feabf` with command and transport regressions.

The fresh post-fix walk then listed and inspected the active overlay by stable workspace ID, invoked
its tool, edited the source, reloaded generation `c9f43a2e…`, and received `Final reload result for
workspace overlay` from the next invocation. Logs remained readable. Removal unlinked the overlay;
a new scoped list omitted it and scoped status returned not found. No model was invoked.

Evidence:
- `docs/qa/bugs/BUG-20260801-extension-cli-workspace-reads.md`
- `internal/cli/extension_test.go`
- `internal/cli/client_test.go`
- `/tmp/compozy-loops-postrebase.N1iPVr/teardown.json`

### Northstar Pay one-kickoff observation

Exactly one kickoff was posted to PM session sess-8c62d4db9addb6b6. All 12 declared tasks received
runs; 10 completed and 2 ended in explicit typed blocked states. Three planned disruptions were
observed and recovered:

- partner_timeout recovered through platform messages;
- pricing_claim_violation was re-routed through the QA monitor, restored to approved copy, and
  cleared by compliance; recovery took about 6m19 after corrected routing;
- canary_error_budget_breach triggered PAUSE + ROLLBACK and completed its recovery task.

The 1,800-second observer still declared a false stall because it reads only controller-authored
journey-log rows. Independent task reads showed the real 10-completed/2-blocked result. This deduped
into the existing open BUG-20260719-autonomous-progress-unobservable rather than creating a duplicate.

Evidence:
- /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260801-135009-390014-lab/qa-artifacts/qa/provider-attempt.json
- /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260801-135009-390014-lab/qa-artifacts/qa/task-terminal-statuses.json
- /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260801-135009-390014-lab/qa-artifacts/qa/observation-summary.json
- /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260801-135009-390014-lab/qa-artifacts/qa/cli/partner-timeout-recovery.json
- /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260801-135009-390014-lab/qa-artifacts/qa/cli/pricing-compliance-verdict.json
- /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260801-135009-390014-lab/qa-artifacts/qa/cli/canary-error-budget-recovery.json

## What Was Fixed

### BUG-20260801-loop-config-file-snake-case

The strict yaml.v3 decoder rejected every documented snake_case LoopConfig field because the domain
type declared JSON tags only. A red regression reproduced both JSON and YAML failures through
loop run --config-file and loop configure --file. Matching YAML tags were added to the full public
LoopConfig field set. The canonical CLI suite passed in green while preserving unknown-field
rejection.

The rebuilt CLI then accepted JSON to start looprun-04fb1c8afcbf8c55 with iteration_cap 3 and accepted
YAML to persist iteration_cap 3, no_progress_window 10, and gate_max_revisions 10.

Evidence:
- /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260801-135009-390014-lab/qa-artifacts/qa/loop/config-file-json-run-status-v6.json
- /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260801-135009-390014-lab/qa-artifacts/qa/loop/config-file-yaml-configure-v6.json

Earlier task checkpoints also hardened the shipped E2E fixtures and production paths for sandbox
binding, exact tool approvals, extension author identity, provider negotiation diagnostics, Dream
route config, fixture confidence, migration-34 harnesses, MCP timeout/default/editor behavior,
clarification races, OAuth callbacks, cost provenance, session focus suppression, workspace session
windows, and provider override envelopes. Both official E2E lanes passed after those fixes.

The MCP manual-auth timeout found during Task 01 is recorded separately in
`docs/qa/reports/2026-08-01-loops-paper-task01-mcp-manual-timeout.md`; its verified fix is part of
`38b2d40`.

### Post-task deep-review remediation

The required one-round deep review found 21 defects. Remediation closed all 21 across exact SSE and
hook contracts, fan-out identity/history, ratchet succession, workspace isolation, durable event
projection, Windows manual MCP auth, Web stream reconstruction, and responsive Loop/session UI.

The refreshed official Web E2E lane passed same-workspace session onboarding and E2E-022
cross-workspace creation on the remediated tree. Deterministic Storybook captures verified the Loop
catalog, Loop recent runs, and session-create dialog at 320px and 1440px. The catalog keeps one Best
element in the DOM and reflows Best + Run together on narrow screens; recent runs retain status,
identity, trigger, generation/best, time, and navigation affordance without horizontal clipping.

Evidence:
- docs/qa/evidence/2026-08-01-loops-paper-adoption/loop-catalog-narrow.png
- docs/qa/evidence/2026-08-01-loops-paper-adoption/loop-catalog-desktop.png
- docs/qa/evidence/2026-08-01-loops-paper-adoption/loop-detail-narrow-full.png
- docs/qa/evidence/2026-08-01-loops-paper-adoption/loop-detail-desktop.png
- docs/qa/evidence/2026-08-01-loops-paper-adoption/session-create-dialog-narrow.png
- docs/qa/evidence/2026-08-01-loops-paper-adoption/session-create-dialog-desktop.png

The final rebase exposed three integration regressions before delivery. Build metadata now ignores
module tags and derives releases only from root `v*` tags. Workspace-scoped extension development now
keys instances by the public workspace registration ID, while retaining the independent local identity
for workspace storage. Loop `gate_verdict` SSE criterion notes now project already-sanitized command
stdout/stderr from the canonical verdict record, so useful diagnostics survive without leaking claim
tokens. The focused regressions and both complete E2E lanes passed after these fixes.

Public QA then found a fourth integration gap: workspace-scoped extension reads existed in HTTP/UDS
but not in the CLI. `extension list/status --workspace` now resolve aliases and paths to the stable
registration ID before using that scoped contract. The canonical CLI command and transport tests,
generated CLI reference, development guide, and official Compozy skill shipped with the fix.

## Paper Cuts

| Persona | Where | Felt | Sharpness | Outcome |
|---|---|---|---|---|
| Cora | Direct run deep link in a different active workspace | The route truthfully said not found but did not offer a one-click workspace switch. | Friction | Documented; no data leak and not a blocker. |
| Bruno | First browser visit after isolated daemon restart | Setup had to be completed before the persisted run was visible. | Friction | Completed with Codex Terra/high defaults; no model invocation occurred. |

## Runtime Errors Observed

- Several exploratory real-model Loop runs returned action-schema or judge-shape failures. After the
  user's model constraint, all new real QA agents used Codex Terra/high. The exact ratchet walk used
  the repository's deterministic ACP fixture; it invoked no model.
- The Northstar kickoff used Sol/high before the user's later QA-model constraint. It was not
  restarted because the scenario contract allows exactly one kickoff.
- Graceful daemon stop timed out while old sessions were active. The exact verified daemon PID was
  terminated, the same persisted QA home restarted cleanly, and all run state remained readable.
- Browser-use CDP was unavailable. The documented agent-browser fallback drove and captured the
  highest-risk UI workflow; this is the only browser-policy deviation.

## Human Verifications Needed

- None for the five minted/reset Loop scenarios, config-file fix, public-surface parity, or CRUD
  canary.

## Decisions for a Human

- Decide separately whether to prioritize the runtime-to-observer projection required by
  BUG-20260719-autonomous-progress-unobservable. The defect predates this workstream and remains
  reproducible; it does not invalidate the independently persisted Task/Loop results.

## Experiential Lens Pass

| Journey | Usability | Accessibility | Perceived performance | Compatibility | Error recoverability | Production parity |
|---|---|---|---|---|---|---|
| J-improve-loop-with-feedback | Pass | Pass | Pass | Pass | Pass | Pass |
| J-extension-dev-lifecycle | Pass | Pass | Pass | Pass | Pass | Pass |

- Bruno could identify why the run stopped, the retained best result, elapsed time, round cap, and
  immutable run identity from one screen. Structured CLI/HTTP/SSE outputs retained stable names,
  bounded diagnostics, and a typed terminal cause without requiring database access.
- The extension loop used the shipped CLI and daemon contracts from link through unlink. Structured
  JSON remained readable, actions returned immediate typed results, failure after removal was a
  specific not-found response, and the same workspace identity survived CLI, HTTP/UDS, reload, logs,
  and invocation. No mock service or model was used.
- Compatibility here covers the CLI/HTTP/UDS contracts on the local release artifact. No Web layout
  changed in this post-rebase replay, so browser/viewport coverage remains owned by the already-passed
  Web E2E and deterministic captures above.

## Learnings

- A YAML decoder does not use JSON tags for KnownFields; public dual-format file contracts need
  explicit YAML names and a command-level regression.
- Same-run parity is strongest when one persisted run is compared byte-for-byte across owner
  surfaces, then probed from a neighboring workspace.
- Exact visual claims require a deterministic runtime fixture even when exploratory real-model runs
  are useful for finding prompt/schema weaknesses.
- The observer remains an external controller view, not a runtime-owned progress source.

## Compozy Impact Audit

- **Native tools:** compozy__loop_status and compozy__loop_runs descriptors, structured outputs,
  schema digests, availability, and read-only risk flags were checked. The owner-workspace output
  matched CLI/HTTP/UDS exactly; the foreign workspace could not invoke or read the run.
  compozy__extensions_list and compozy__extensions_info already used the workspace-scoped daemon
  contract and served as parity references; their IDs, descriptors, schemas, digests, risk flags,
  capability gates, and availability diagnostics did not change.
- **Extensibility and hooks:** loop.gate.post, loop.generation.pre, extension scorer contracts,
  generation origins, and SSE projections were exercised by the official runtime lane and manual
  replay. Workspace extension overlays were reverified against the registration ID across dev,
  CLI list/status, reload, logs, invoke, and remove. The CLI flags and official skill now expose the
  existing scoped read; extension hooks, bundles, bridge SDKs, MCP sidecars, and config lifecycle are
  unchanged. Post-review codegen carries the closed seven-value generation-origin vocabulary into
  both SDKs.
- **Workspace data isolation:** Loop definitions and runs are workspace-scoped. workspace_id was
  verified through CLI, HTTP, UDS, native-tool, SSE, and Web reads. Foreign HTTP/UDS/CLI reads
  returned not found and exposed no run body. Snapshot restore now validates the stored Loop
  identity against the requested workspace before reconstruction. Extension dev instances use the
  resolved registry ID as their ownership key and do not conflate it with the local workspace marker.
  CLI aliases and paths are resolved to that stable ID before the existing HTTP/UDS scoped list/status
  reads; manager and daemon integration suites retain the published/global instance independently.
- **Official Compozy skill:** the bundled Loop reference documents the closed generation-origin
  vocabulary and `gate_verdict.item_index`; the capability/bundle reference now documents scoped
  extension list/status. Config-file field names remain the documented public snake_case vocabulary.

## Final Status

- **Verdict:** PASS.
- **Official E2E lanes:** make test-e2e-runtime and make test-e2e-web passed on the final post-rebase
  working tree after the deep-review and integration remediation.
- **Targeted regressions:** CLI JSON/YAML config files and adjacent safety checks passed under race.
- **Strict QA evidence audit:** PASS — the scenario tracker materialized 717 valid rows, the matrix
  has no pending entry, both fixed rows link verified bug records and real SHAs, and teardown records
  are clean.
- **Teardown:** PASS — /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260801-135009-390014-lab/qa-artifacts/qa/teardown.json records clean: true with zero survivors.
- **Exit gate:** PASS — `make gate-full` completed the current-tree `make verify`; its fingerprint
  and log are owned by `.cache/gate/` and reported by `make gate-status`.
- **Issues:** 3 QA-discovered fixes and 21/21 deep-review defects fixed; 1 pre-existing open
  Blocks-Completion observer issue remains a separate human prioritization.
- **User-impact totals:** 2 Blocks-Completion fixed; 1 Friction fixed; 0 Data-Loss; 0 Trust-Damage;
  0 Cosmetic. The separate pre-existing queue retains 1 open Blocks-Completion observer bug.
- **Coverage:** 6/6 minted/reset Loop and SSE scenarios passed; 1/1 extension-dev scenario passed
  after its QA fix; 1/1 new config regression passed; 1/1 MCP timeout scenario passed; 4 adjacent
  safety/CRUD scenarios passed; 1 planned goal-toggle branch skipped in favor of the single required
  delete canary; Northstar runtime completed with the observer decision blocked.
- **Verdict:** QA behavior, post-review remediation, current Web/session re-walks, visual evidence,
  and cleanup are complete. Delivery uses the final current-tree full-gate evidence record.
