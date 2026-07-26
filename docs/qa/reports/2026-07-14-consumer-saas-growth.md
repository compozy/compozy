# QA Run Report — 2026-07-14 — Automation + Hermes rebase

- **Scope:** Final-worktree validation of the Automation Features rebase onto Hermes Bridge / ACP diagnostic attribution, including session durability/deletion, Task parent settlement and Loop wake ordering, workspace pruning, Goal judge attribution and clear behavior, Job Loop target persistence, and one provider-backed Lumen Notes canary.
- **Cadence tier:** targeted
- **Build:** `e9e5eb18792653310a9b65ffb33c07b72fb94f94` plus the reviewed uncommitted remediation · **Environment:** fresh isolated lab at `http://127.0.0.1:63966`; browser policy `browser-use`; playbook `consumer-saas-growth`
- **Started:** 2026-07-14T19:51:23Z · **Ended:** 2026-07-14T20:17:03Z · **Status:** BLOCKED (live provider rate limit)
- **Bootstrap manifest:** `/Users/pedronauck/dev/qa-labs/agh-consumer-saas-growth-20260714-194637-422214-lab/qa-artifacts/qa/bootstrap-manifest.json`

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Théo | Returning operator | desktop / wifi-fast / en-US | CH-session-message-delete-integrity |
| Bruno | Power user | desktop / wifi-fast / en-US | CH-task-tree-loop-rollup, CH-prune-missing-workspace, CH-automation-crud-loop-target, CH-041 |
| Lea | New user | laptop / wifi-fast / en-US | CH-judge-session-attribution |
| Priya Joshi | Head of Growth | desktop / wifi-fast / en-US | CH-consumer-saas-growth-runtime |

## Flows in Scope

- `J-11` — Return to a running session with durable, truthful history (`../journeys/J-11-return-to-running-session.md`)
- `J-24` — Triage work and manage automation at scale (`../journeys/J-24-triage-work-at-scale.md`)
- `J-26` — Start, converge, and control a conversational Goal (`../journeys/J-26-converge-and-control-goal.md`)
- `J-28` — Recover from context pressure and budget boundaries truthfully (`../journeys/J-28-recover-context-and-budget.md`)
- `J-complete-task-tree` — Complete a task tree and fire its follow-up Loop (`../journeys/J-complete-task-tree.md`)
- `J-prune-missing-workspace` — Remove a missing local workspace (`../journeys/J-prune-missing-workspace.md`)
- `consumer-saas-growth` — Sustain the Lumen Notes activation sprint from one operator kickoff.

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---:|---|---|---|---|---|---|---|
| 1 | CH-session-message-delete-integrity | J-11 / RT-session-message-reload, RT-session-delete-owned-history | Théo | Interrupt Tour | Pass | | |
| 2 | CH-task-tree-loop-rollup | J-complete-task-tree / TA-parent-rollup-completion, LP-task-rollup-wakes-loop, LP-042 | Bruno | State Tour | Pass | | |
| 3 | CH-prune-missing-workspace | J-prune-missing-workspace / RT-missing-workspace-pruned | Bruno | Navigation Tour | Pass | | |
| 4 | CH-automation-crud-loop-target | J-24 / TA-automation-crud-loop-target, LP-034, LP-035 | Bruno | Feature Tour | Pass | | |
| 5 | CH-judge-session-attribution | J-26 / GL-judge-session-contract | Lea | Feature Tour | Blocked (needs human verify) | Claude session limit | |
| 6 | CH-041 | J-28 / GL-019 | Bruno | Interrupt Tour | Blocked (needs human verify) | Claude session limit | |
| 7 | CH-consumer-saas-growth-runtime | consumer-saas-growth | Priya Joshi | Task Tour | Blocked (needs human verify) | Claude session limit | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### Session durability and deletion — Pass

The final worktree preserved the authored message through fresh transcript/history reads, then removed the selected stopped session through the public delete surface. Fresh detail, history, transcript, usage, event, and catalog reads no longer exposed the deleted session. A neighboring session retained its history and usage. Evidence lives under `qa/session-delete/` plus `qa/screenshots/session-delete-*.png` in the bootstrap artifact root.

### Parent settlement and Loop wake — Pass

The parent stayed nonterminal before the final child settlement and completed after child C committed. Fresh reads showed the parent transition exactly once. The complete runtime E2E gate independently passed the matching Loop wake plus disabled-Loop and unrelated-workspace negative controls. Evidence lives under `qa/task-rollup/` and `qa/screenshots/task-parent-rollup-completed.png`.

### Missing-workspace pruning — Pass

Removing the registered root and refreshing the public catalog pruned the missing workspace without removing the healthy home workspace. Web and structured catalog reads converged and the stale selection did not survive navigation.

### Automation Loop targets — Pass

Global and workspace Jobs preserved their authoritative Loop target and typed inputs across create, read, update, list, mismatch validation, and delete. The mismatched workspace target was rejected, while both selected Jobs returned 404 after deletion. Evidence lives under `qa/automation-loop-target/` and `qa/screenshots/automation-workspace-loop-target.png`.

### Judge attribution, active Clear, and provider-backed growth scenario — Blocked

Both Claude `native_cli` sessions reached the provider, but every prompt returned the typed `rate_limit` boundary (`session limit`). The scenario contract forbids a fallback provider, so no live verdict, active-Clear decision, autonomous Task run, peer exchange, review cycle, disruption recovery, or deliverable may be claimed. Deterministic ACP/Goal E2E controls passed separately, but they do not replace this provider-backed acceptance. Evidence: `qa/provider-attempt.json`, `qa/judge-attribution/`, `qa/observation-summary.json`, and `qa/qa-audit-report.md`.

## What Was Fixed

The manual controls found no new production defect. Subsequent automated gates found and fixed three integration defects in the final worktree: explicit preferred-model negotiation was skipped when ACP advertised the same current model; daemon restart recovered task roles before session identity reconciliation; and the Goal pause/resume fixture over-constrained a legitimate attempt number. Web E2E also exposed three stale UI interactions and one six-SSE HTTP/1.1 test-pool saturation; those canonical specs now exercise the explicit workspace switcher, typed Automation delete confirmation, and an observer with an independent network context.

## Paper Cuts

| Persona | Where (journey/step) | Felt | Sharpness | Outcome |
|---|---|---|---|---|
| Lea / Bruno | Goal provider canary | The product reached the provider but could not spend a turn because the account session limit was exhausted. | Blocking external boundary | Reported as BLOCKED; no fallback or synthetic promotion. |
| Théo | Two simultaneous task-detail UIs | Three long-lived SSEs per page can exhaust Chromium's six HTTP/1.1 connections when two pages share one network context. | Test/environment architecture | The cross-page SSE proof uses independent browser contexts; shared-stream or multiplexing remains a product follow-up. |

## Runtime Errors Observed

- Claude returned a typed `rate_limit` error for both live provider prompts.
- The strict `consumer-saas-growth` audit correctly failed because the provider boundary prevented all required collaborative runtime evidence.

## Human Verifications Needed

- Repeat the real-provider judge lifecycle and active-Clear controls after the Claude session limit resets.
- Repeat the full `consumer-saas-growth` playbook with the declared provider; synthetic or fallback output is not acceptable.

## Decisions for a Human

None. The provider boundary determines when the blocked controls can be rerun, not a product-contract decision.

## Learnings

- Public-surface session deletion preserved the neighboring-session boundary while removing every selected-session read surface.
- Parent settlement and Automation Loop target persistence agree across direct QA artifacts and the complete runtime/Web E2E gates.
- Provider reachability is not provider acceptance: a typed rate limit is useful evidence, but it cannot support judge or autonomous-collaboration claims.
- Multi-page live views should not depend indefinitely on three independent HTTP/1.1 SSE connections per page; the E2E now isolates its observer pool while leaving multiplexing as a follow-up.

## Final Status

- **Exit gates:** `make test-e2e-runtime` passed; `make test-e2e-web` passed 75/75 with zero flaky/skipped/unexpected results; the final `make verify` passed after the post-fix peer review.
- **Issues by user impact:** zero new confirmed product regressions in the completed controls; one external provider blocker; one multi-page SSE architecture follow-up.
- **Coverage:** 7/7 planned sessions settled: 4 Pass, 3 Blocked by the same live-provider boundary.
- **Teardown:** `qa/teardown.json` records `clean: true`, no survivors, and the isolated daemon stopped.
- **Verdict:** **BLOCKED** for provider-backed release/scenario acceptance. The independent session, workspace, Task, Loop-target, runtime E2E, and Web E2E controls pass, but they do not promote the blocked judge or `consumer-saas-growth` claims.

## AGH Impact Audit

- **Native tools:** `agh__session_list` adds the exact public session `type` filter and its generated descriptor/catalog digest; the session and workspace list/remove paths were checked across native tools, CLI, HTTP, UDS, and the official references.
- **Extensibility and hooks:** typed ACP judge metadata now crosses subprocess fixtures without substring routing; Task recovery/settlement hooks preserve post-commit publication ordering; Automation Loop targets keep their registry-backed workspace identity. Hermes bridge SDK/config lifecycle remains unchanged by the remediation after checking bridge boot and delivery surfaces.
- **Workspace data isolation:** sessions remain workspace-scoped through catalog/SSE/delete paths; missing-workspace removal is serialized against session starts; Job Loop targets resolve global versus workspace identity once and reject mismatches; Task/Loop settlement propagates the owning `workspace_id`. Manual controls plus runtime/Web E2E cover list/read/event/cache boundaries without cross-workspace leakage.
- **Official AGH skill:** `skills/agh/references/native-tools.md`, `runtime-operations.md`, and `tasks-and-orchestration.md` document the session type/list/remove, workspace reconciliation, and parent-rollup contracts shipped by the branch.
