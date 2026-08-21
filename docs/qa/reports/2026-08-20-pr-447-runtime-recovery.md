# QA Run Report — 2026-08-20 — PR 447 runtime recovery

- **Scope:** Session follow-up transport, Loop action-result settlement, Loop task recovery, confirmed-crash ownership, and resource-agent observation.
- **Cadence tier:** targeted
- **Build:** `ed93a4b3` · **Environment:** fresh isolated local lab; CLI, HTTP, UDS, runtime/provider, and Web where applicable
- **Started:** 2026-08-20 23:10 BRT · **Status:** blocked-verify
- **Automated precondition:** focused Go suites passed 4,255 tests and strict Go lint reported zero issues; the diff gate later hit the unrelated `internal/store/globaldb` package timeout at 10 minutes.

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Théo | Returning User | desktop / wifi-fast / en-US | RT-018 |
| Bruno | Power User | desktop / wifi-fast / en-US | CH-loop-task-recovery-binding; CH-oversized-loop-result |
| Ada | Reliability Engineer | desktop / flaky or wifi-fast / en-US | CH-loop-managed-session-death; CH-extension-agent-observation |

## Flows in Scope

- `J-13` — Follow a live run (`../journeys/J-13-follow-a-live-run.md`).
- `J-recover-loop-node-failure` — Author, run, repair, and finish a Loop (`../journeys/J-recover-loop-node-failure.md`).
- `J-complete-partial-loop` — Author and complete a routed partial Loop (`../journeys/J-complete-partial-loop.md`).
- `J-extension-dev-lifecycle` — Operate a workspace-scoped extension generation safely (`../journeys/J-extension-dev-lifecycle.md`).

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | focused stopped-follow-up walk | J-13 / RT-018 | Théo | Feature Tour | Pass | | `75ce57f2` |
| 2 | CH-loop-task-recovery-binding | J-recover-loop-node-failure / TA-033 | Bruno | Feature Tour | Blocked (needs human verify) | Public surfaces cannot create a Loop-owned needs-attention precondition. | `75ce57f2`, `ed93a4b3` |
| 3 | CH-loop-managed-session-death | J-recover-loop-node-failure / RT-subprocess-health-escalation | Ada | Interrupt Tour | Blocked (needs human verify) | No public managed-session crash injector exists. | `75ce57f2`, `ed93a4b3` |
| 4 | CH-loop-managed-session-death | J-recover-loop-node-failure / LP-crash-death-resume | Ada | Interrupt Tour | Blocked (needs human verify) | No public managed-session crash injector exists. | `75ce57f2`, `ed93a4b3` |
| 5 | CH-oversized-loop-result | J-complete-partial-loop / LP-oversized-action-result-fails | Bruno | Garbage Tour | Blocked (needs human verify) | Builtin transform externalizes results before the repaired raw result boundary. | `75ce57f2` |
| 6 | CH-extension-agent-observation | J-extension-dev-lifecycle / ET-extension-agent-observer-resolution | Ada | Feature Tour | Blocked (needs human verify) | Live revision and isolation passed; no public late stopped-event injector exposes recovered auth state. | `75ce57f2`, `ed93a4b3` |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

- Théo stopped `sess-da661dc3b57e16ca`, prompted it again through CLI, stopped it again, and prompted it through Web. All three exact markers stayed in one durable transcript and one ACP session.
- Ada changed `qa_resource_coder` from GPT-5.4/approve-all to GPT-5.6 Sol/deny-all without restarting the daemon. A new live provider session used GPT-5.6 Sol and returned `REVISION_MARKER_447`.
- Ada compared observe over two workspaces. `pr447-qa` contained `qa_resource_coder` usage; `pedronauck` remained at zero tokens and zero agent share.
- Bruno ran a 70,000-byte builtin transform. It completed because transform externalizes output before settlement; the run was correctly excluded from the oversized raw action-result verdict.

Smoke evidence: /Users/pedronauck/dev/qa-labs/compozy-pr-447-runtime-recovery-20260821-020432-748658-lab/qa-artifacts/qa/evidence/observe-isolation.json

Behavioral evidence: /Users/pedronauck/dev/qa-labs/compozy-pr-447-runtime-recovery-20260821-020432-748658-lab/qa-artifacts/qa/evidence/stopped-session-transcript.json

## What Was Fixed

- Stopped-session prompting now sends only the latest user message to the provider while preserving the durable transcript.
- Oversized raw Loop action results fail settlement without leaving a running lease.
- Loop task recovery preserves exact generation, node, and item binding and projects intervention attention consistently.
- Confirmed Loop worker crashes have one recovery owner; generic subprocess escalation does not compete with Loop recovery.
- Observer resolution uses live global, workspace, and builtin catalogs, invalidates authorization when the catalog revision changes, and recovers stopped-session model, permissions, and auth owner from persisted metadata.

## Paper Cuts


- The builtin transform action is not a valid public probe for raw action-result size because it externalizes its payload before settlement. This is a test-path limitation, not a product defect.

## Runtime Errors Observed


- None in the successful stopped-session, provider, catalog-revision, HTTP, Web, or workspace-isolation walks.

## Human Verifications Needed


1. Create a Loop-owned task run in `needs_attention` with a fixture that preserves generation, node id, and item index. Recover it once through `compozy task run recover $RUN_ID`, then compare CLI and HTTP for one failed source and one queued continuation. A second recovery must create nothing.
2. Start a checkpointing Loop node with a killable provider fixture. Kill the managed process after progress and verify one continuation/new epoch, no generic needs-attention transition, and cancel precedence. Repeat without progress to `resume_exhausted`.
3. Install a resource extension action that returns more than 64 KiB directly. Execute it in a Loop and verify a failed task/run, no active lease, no success output, and one bounded validation diagnostic through CLI and HTTP.
4. Use an observer integration harness to emit a late event after the session is removed from the live registry. Verify the event retains the persisted model, effective permission mode, and authentication owner across a catalog revision and remains workspace-scoped.

## Decisions for a Human

None. The remaining boundary is missing public fault injection, not an unresolved product choice.

## Learnings


- Session continuity can be proven strongly through one cross-surface object: the same Compozy session id and ACP session id appeared in CLI, HTTP, Web, and persisted runtime events.
- Catalog cache invalidation is visible without internal reads by changing the resource agent, starting a new session without daemon restart, and inspecting the persisted prompt runtime.
- Fault-oriented Loop scenarios still need supported fixtures; using direct database writes would bypass the product contract and produce misleading evidence.

## Experiential Lens Pass


- Feature Tour: one stopped durable session survived CLI and Web follow-ups with continuous history.
- Interrupt Tour: blocked at the missing public provider-process death injector.
- Garbage Tour: the available builtin action used a different result path, so the raw oversized boundary remains blocked instead of receiving a false pass.

## Final Status

**Verdict: BLOCKED.** RT-018 passed end to end with a live Codex provider, and the resource-catalog revision plus workspace-isolation paths passed publicly. Four Loop/crash checks and the stopped-event authorization tail require fault-injection fixtures that the public product does not expose. The repaired branches remain covered by their canonical Go suites; final `make gate-full` evidence is recorded in the isolated QA journey log after source freeze.
