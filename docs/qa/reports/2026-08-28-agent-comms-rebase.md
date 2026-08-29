# QA Run Report — 2026-08-28 — agent-comms-rebase

- **Scope:** Rebased agent communications runtime and web conflict repairs: Activity child lifecycle, session-catalog liveness, durable operator wake turns, serialized child revival, and large-tree keyboard behavior.
- **Cadence tier:** targeted
- **Build:** working tree after `agent-comms` rebase · **Environment:** isolated daemon at `http://127.0.0.1:64046`; deterministic ACP provider boundary; desktop, wifi-fast, en-US
- **Started:** 2026-08-28T22:29:00-03:00 · **Status:** passed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | Operator | desktop / wifi-fast / en-US | CH-agent-comms-operator-fence |
| Théo | Returning session user | desktop / wifi-fast / en-US | CH-agent-comms-in-session-truth |

## Flows in Scope

- `J-supervise-delegation-trees` — supervise delegated work from Activity and the conversation without trusting browser-invented state (`../journeys/J-supervise-delegation-trees.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-agent-comms-operator-fence | J-supervise-delegation-trees / RT-delegation-activity-tree | Ada | Interrupt Tour | Pass | | |
| 2 | CH-agent-comms-in-session-truth | J-supervise-delegation-trees / RT-in-context-call-messages | Théo | Feature Tour | Pass | | |
| 3 | CH-agent-comms-in-session-truth | J-supervise-delegation-trees / RT-session-calls-inspector-panel | Théo | Feature Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### Ada — delegation activity

Ada created a real three-level delegation through the public CLI, then opened Dock → Agents → Activity. The tree rendered the operator root plus `delegator-l1`, `delegator-l2`, and `leaf-l3` at their daemon-owned levels. A root-filtered page with `limit=1` reported total `6` through both CLI and HTTP, matching the Activity header. Keyboard `ArrowLeft` folded the tree while retaining focus; browser E2E independently proved `ArrowRight` unfolding and traversal across the 150-call window.

### Théo — call truth in context

Théo opened the completed `reviewer` row, saw prompt `golden path`, result `{ "answer": 42 }`, caller and child identities, and the terminal timeline. Sending `one more thing` from the call record produced durable receipt `msg-d7add48b2e076ea6` with delivery `woke`. The child session stored a workspace-scoped `synthetic_reentry` turn with the exact message inside the inert untrusted frame. Reloading `/agents/activity` and selecting the workspace rebuilt the same daemon-backed tree.

## What Was Fixed

No additional production fix was required during the manual walk. The current-code browser E2E suite had already exercised the fold/unfold, deep-link, stale-state, cancel, wake, and 150-call paths.

## Paper Cuts

The browser automation process exited while repeating the already-passing `ArrowRight` probe. The independent Playwright browser E2E passed the same fold/unfold contract on the current source, so this was treated as a QA harness interruption rather than a product finding.

## Runtime Errors Observed

The deterministic `reviewer` fixture returned a terminal tool after the original call was already settled. The runtime correctly rejected it as `call_return_unbound`; this happened after the message was durably accepted and woke the child, and does not reproduce in the production contract tests.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- The Activity count remained truthful when only one of six rows was loaded.
- Child lifecycle came from the session catalog: running work remained distinct from parked completed children.
- Operator messages persisted as synthetic turns before provider execution, so the receipt survived the controlled fixture error.

## Final Status

- **Exit gate (full automated suite):** PASS — `make gate`; all seven `make gate-status` records are `CURRENT-PASS` for the final tree.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 1/1 journeys walked; 3/3 in-scope scenarios passed
- **Verdict:** PASS — manual public-surface evidence agrees with the daemon, CLI/API cross-checks, and current-code browser E2E.
