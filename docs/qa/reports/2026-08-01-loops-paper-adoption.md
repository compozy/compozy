# QA Run Report — 2026-08-01 — Loops Paper Adoption

- **Scope:** Metric ratchet, rejected-gate succession, repair context, generation provenance, breaker semantics, Loop config files, and persisted run-detail truth across CLI, HTTP, UDS, native tools, SSE, Web, docs, and the official skill.
- **Cadence tier:** targeted, with adjacent safety checks, one destructive Loop CRUD canary, and one rotated Northstar Pay real-scenario observation.
- **Build:** Task 07 committed at 746fd06; the post-task deep-review remediation is in the working tree.
- **Environment:** fresh isolated northstar-pay-20260801-135009-390014 lab; daemon target http://127.0.0.1:60717.
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
| 13 | CLI config-file regression | LP-loop-config-file-snake-case | Bruno | Feature Tour | Fixed | BUG-20260801-loop-config-file-snake-case | pending Task 07 commit |

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

Task 07 also hardened the shipped E2E fixtures and production paths for sandbox binding, exact tool
approvals, extension author identity, provider negotiation diagnostics, Dream route config, fixture
confidence, migration-34 harnesses, MCP timeout/default/editor behavior, clarification races, OAuth
callbacks, cost provenance, session focus suppression, workspace session windows, and provider
override envelopes. Both official E2E lanes passed after those fixes.

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

- Bruno could identify why the run stopped, the retained best result, elapsed time, round cap, and
  immutable run identity from one screen.
- Cora's deep link remained truthful across workspace isolation and did not silently switch context.
- Ada's structured views exposed typed origins, parent generations, scores, recovery reasons, and
  exact ownership outcomes without requiring database access.

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
- **Extensibility and hooks:** loop.gate.post, loop.generation.pre, extension scorer contracts,
  generation origins, and SSE projections were exercised by the official runtime lane and manual
  replay. Post-review codegen now carries the closed seven-value generation-origin vocabulary into
  both SDKs; bridge, MCP-sidecar, bundle, and config lifecycle contracts remain unchanged.
- **Workspace data isolation:** Loop definitions and runs are workspace-scoped. workspace_id was
  verified through CLI, HTTP, UDS, native-tool, SSE, and Web reads. Foreign HTTP/UDS/CLI reads
  returned not found and exposed no run body. Snapshot restore now validates the stored Loop
  identity against the requested workspace before reconstruction.
- **Official Compozy skill:** the bundled Loop reference now documents the closed generation-origin
  vocabulary and `gate_verdict.item_index`; the config-file field names remain the documented public
  snake_case vocabulary.

## Final Status

- **Verdict:** PASS.
- **Official E2E lanes:** make test-e2e-runtime passed; make test-e2e-web passed again after the
  deep-review remediation.
- **Targeted regressions:** CLI JSON/YAML config files and adjacent safety checks passed under race.
- **Strict QA evidence audit:** PASS after the full verification evidence was indexed.
- **Teardown:** PASS — /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260801-135009-390014-lab/qa-artifacts/qa/teardown.json records clean: true with zero survivors.
- **Final make verify evidence:** /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260801-135009-390014-lab/qa-artifacts/qa/final-make-verify.log.
- **Exit gate:** the current-tree full-gate fingerprint and log are owned by `make gate-status` after
  final source freeze; C14 indexes that same log in the QA evidence directory.
- **Issues:** 1 QA-discovered config fix and 21/21 deep-review defects fixed; 1 pre-existing open
  Blocks-Completion observer issue remains a separate human prioritization.
- **Coverage:** 5/5 minted/reset Loop scenarios passed; 1/1 new config regression passed; 4 adjacent
  safety/CRUD scenarios passed; 1 planned goal-toggle branch skipped in favor of the single required
  delete canary; Northstar runtime completed with the observer decision blocked.
- **Verdict:** QA behavior, post-review remediation, current Web/session re-walks, visual evidence,
  and cleanup are complete. Delivery uses the final current-tree full-gate and C14 evidence records.
