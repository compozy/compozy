# QA Run Report — 2026-07-13 — automation-features

- **Scope:** Full real-user pass over Loops (with and without goals), Goals, Jobs, Triggers, Tasks, session startup/background continuity, and missing-workspace reconciliation; named regressions AGH-47, AGH-71, and AGH-84.
- **Cadence tier:** full
- **Build:** base 08c1797b with the semantic QA remediation commits listed below · **Environment:** isolated live-provider labs; final lab daemon was at http://127.0.0.1:53199 with Web at http://localhost:3000; in-app browser; Cursor/Grok 4.5 for provider legs
- **Started:** 2026-07-13T04:48:03Z · **Completed:** 2026-07-14T09:19:48Z · **Status:** historical run complete; post-rebase revalidation required

## Post-rebase validity notice

This report preserves the real-user evidence produced by the original Automation branch, but its final verdict does not apply to the rebased Automation + Hermes worktree. The rebase changed prompt identity, judge routing, Task settlement publication, workspace/session removal, and Job Loop-target binding. Their canonical scenarios were reset below, the combined full gate is pending, and the tracker remains the authority for promotion. The missing session-latency, Cursor filesystem-artifact, disabled-Loop, unrelated-workspace, and cold message-reload controls must run from the final source state before a new PASS verdict.

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Lea | New User | laptop / wifi-fast / en-US | CH-001, CH-008, CH-046, CH-use-response-as-goal |
| Bruno | Power User | desktop / wifi-fast / en-US | CH-006, CH-007, CH-loop-goal-delete, CH-automation-crud-loop-target, CH-task-tree-loop-rollup, CH-task-template-draft, CH-038, CH-045, CH-047, CH-new-session-latency-title, CH-cursor-agent-mode, CH-prune-missing-workspace |
| Théo | Power User | desktop / wifi-fast / en-US | CH-background-session-switch |
| Ada | Power User (agent surfaces) | desktop / wifi-fast / en-US | CH-024 |
| Marina | Casual User | desktop / wifi-fast / en-US | CH-009 |

## Flows in Scope

- `J-01` — arrive and run a default Loop (`../journeys/J-01-arrive-and-use-run.md`)
- `J-02` — preview a Loop without side effects (`../journeys/J-02-dry-run-preview.md`)
- `J-05` — configure a Loop without forking (`../journeys/J-05-configure-no-fork.md`)
- `J-06` — fork, edit, run with/without goal, and delete a custom Loop (`../journeys/J-06-fork-and-edit.md`)
- `J-09` — bind schedules and triggers to a Loop (`../journeys/J-09-automation-start-bindings.md`)
- `J-complete-task-tree` — roll up a task tree and create its follow-up (`../journeys/J-complete-task-tree.md`)
- `J-24` — triage tasks and manage automations at scale (`../journeys/J-24-triage-work-at-scale.md`)
- `J-26/J-27` — start, control, author, and observe Goals (`../journeys/J-26-converge-and-control-goal.md`, `../journeys/J-27-observe-and-author-goal.md`)
- `J-17/J-11` — create a session quickly and keep it visible across workspaces (`../journeys/J-17-session-create-unified-selector.md`, `../journeys/J-11-return-to-running-session.md`)
- `J-prune-missing-workspace` — remove a missing local workspace (`../journeys/J-prune-missing-workspace.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-001 | J-01 / LP-001, LP-002, LP-action-failure-detail | Lea | Feature Tour | Fixed | BUG-20260713-loop-failure-hidden | `5b8ee98`, `667f4e9` |
| 2 | CH-008 | J-02 / LP-006, LP-007 | Lea | Garbage Tour | Pass | | |
| 3 | CH-006 | J-05 / LP-017..LP-020 | Bruno | Back-Button Tour | Fixed | BUG-20260713-loop-preview-ignores-config | `667f4e9` |
| 4 | CH-007 | J-06 / LP-021..LP-024 | Bruno | Multi-Tab Tour | Fixed | BUG-20260713-loop-fork-internal-error | `5b8ee98`, `45aa855`, `667f4e9` |
| 5 | CH-loop-goal-delete | J-06 / LP-toggle-loop-goal, LP-delete-custom-loop | Bruno | Feature Tour | Fixed | BUG-20260713-loop-contract-goal-not-editable; BUG-20260713-custom-loop-delete-missing | `5b8ee98`, `667f4e9` |
| 6 | CH-009 | J-09 / LP-033..LP-035 | Marina | Back-Button Tour | Skipped | broader schedule/filter/failure-policy matrix explicitly deferred after the integrated Job/Trigger/Loop lifecycle passed | |
| 7 | CH-automation-crud-loop-target | J-24 / TA-automation-crud-loop-target | Bruno | Garbage Tour | Fixed | Trigger fired exactly once; delegated generation-zero failure now terminalizes truthfully; Trigger safely deleted | `5b8ee98`, `667f4e9` |
| 8 | CH-task-tree-loop-rollup | J-complete-task-tree / TA-004, TA-033, TA-task-role-session-activation, TA-parent-rollup-completion, LP-task-rollup-wakes-loop | Bruno | Feature Tour | Fixed | fresh two-child Cursor tree settled parent exactly once; matching Loop woke exactly once and created one follow-up | `a18665e`, `667f4e9` |
| 9 | CH-024 | J-16 / LP-042 | Ada | Feature Tour | Skipped | agent-surface adjacency explicitly deferred after public UI/HTTP/UDS and native-tool Task/Loop proofs converged | |
| 10 | CH-038 | J-24 / TA-002, TA-040, TA-052, TA-054, TA-056, TA-065 | Bruno | Feature Tour | Skipped | broader triage matrix explicitly deferred; create/edit/pause/resume/approve/recover/delete critical path passed | |
| 11 | CH-046 | J-26 / GL-001..GL-013 | Lea | Feature Tour | Fixed | real Cursor/Grok Goal completed with strict approved verdict and zero judge tools; exact GL-004 two-rejection macro remains broader coverage | `43659b1`, `5b8ee98`, `667f4e9` |
| 12 | CH-047 | J-26 / GL-005..GL-008, GL-019 | Bruno | Interrupt Tour | Fixed | active-judge Clear fences successor work, joins cleanup, and renders the typed terminal cause; Stop restores the composer | `43659b1`, `5b8ee98`, `667f4e9` |
| 13 | CH-045 | J-27 / GL-022..GL-024 | Bruno | Feature Tour | Skipped | broader Goal authoring matrix explicitly deferred after start/approve/block/clear/stop and builder authoring critical paths passed | |
| 14 | CH-new-session-latency-title | J-17 / RT-new-session-fast-feedback, RT-session-auto-title | Bruno | Network Tour | Fixed | startup latency/modal, automatic-title, and AGH-84 identity branches pass | `852629d`, `43659b1`, `667f4e9` |
| 15 | CH-background-session-switch | J-11 / RT-workspace-active-session-badge, RT-041, RT-045 | Théo | Interrupt Tour | Fixed | onboarding is system-classified; exact badges, reciprocal Return, reconnect, stop decrement, and delete pass; AGH-84 | `43659b1`, `45aa855`, `667f4e9` |
| 16 | CH-prune-missing-workspace | J-prune-missing-workspace / RT-missing-workspace-pruned | Bruno | Interrupt Tour | Fixed | BUG-20260713-missing-workspace-persists; AGH-47 | `43659b1`, `667f4e9` |
| 17 | CH-use-response-as-goal | J-26 / GL-use-response-as-goal | Lea | Feature Tour | Fixed | BUG-20260713-use-as-goal-inert; invalid Goal copy also verified | `43659b1`, `45aa855`, `667f4e9` |
| 18 | CH-cursor-agent-mode | J-17 / RT-cursor-agent-mode | Bruno | Feature Tour | Fixed | real Cursor/Grok task-role and user sessions now run in Agent mode | `852629d`, `43659b1`, `667f4e9` |
| 19 | CH-task-template-draft | J-complete-task-tree / TA-task-template-preserves-draft | Bruno | Back-Button Tour | Pass | BUG-20260713-task-template-clears-draft fixed and browser-verified | |
| 20 | CH-onboarding-stale-workspace | J-19 / RT-004 | Lea | Back-Button Tour | Fixed | BUG-20260713-onboarding-stale-workspace-draft browser-verified | `43659b1`, `667f4e9` |
| 21 | CH-session-transcript-reload | J-11 / RT-session-message-reload | Théo | Interrupt Tour | Fixed | ordinary and `/goal` inputs remained exactly once/in order across both acceptance reloads | `852629d`, `43659b1`, `45aa855`, `667f4e9` |
| 22 | CH-task-tree-loop-rollup | J-complete-task-tree / TA-task-create-async-activation | Bruno | Network Tour | Fixed | BUG-20260714-task-create-waits-for-worker-session | `a18665e`, `667f4e9` |
| 23 | CH-session-transcript-reload | J-11 / RT-session-delete-owned-history | Bruno | Garbage Tour | Fixed | BUG-20260714-session-delete-history-fk | `43659b1`, `667f4e9` |
| 24 | CH-task-tree-loop-rollup | J-complete-task-tree / TA-016 | Bruno | Interrupt Tour | Fixed | BUG-20260714-task-named-events-stale | `667f4e9` |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Final Replay Corrections and Browser Acceptance

The final replay resumed in a fresh, non-reused lab at `/Users/pedronauck/dev/qa-labs/agh-automation-features-final-replay-20260713-20260713-194432-535561-lab`. Daemon PID `47809` was registered on manifest port `56381`; Web PID `51236` was registered on `3000`, and direct/proxy status agreed. Cursor Agent with `Grok 4.5 (High, Fast)` was selected through the onboarding UI. Step 2 restored an earlier daemon's workspace selection and initially failed Remove with `workspace not found`. A single serialized Codex GPT-5.6-SOL/high worker fixed the browser-draft/current-catalog authority boundary; no peer review ran. Controller replay then removed the stale entry with zero alerts, preserved registered DELETE semantics, added exactly `agh3` and `bench-ops`, and completed onboarding. `BUG-20260713-onboarding-stale-workspace-draft` and RT-004 are verified/pass. That pre-classification lab was torn down cleanly; AGH-84 was accepted in the fresh post-fix lab described below.

The list below retains the failed controls that drove remediation together with the final same-persona Browser verdicts:

- **AGH-84 accepted:** the fresh post-fix lab `/Users/pedronauck/dev/qa-labs/agh-automation-features-post-onboarding-fix-20260713-20260713-203513-816377-lab` proved the upstream classification and the formerly hanging route together. Onboarding session `sess-8101d12c9aaa4db0` persisted as `system`; the all-type catalog retained it while the user-only catalog and inactive-workspace badge counted only authored session `sess-84c5282a292e7f0f`. A second real Cursor/Grok 4.5 session `sess-0c73989a7e97390b` in `bench-ops` received a durable title. Direct reciprocal Return clicks reconciled workspace, permalink, title, and transcript without Loading; stopping it removed the bench badge and deleting it through the modal produced Sessions `0` plus HTTP 404. `BUG-20260713-background-session-indicator-title` and `BUG-20260713-cross-workspace-session-return-hangs` are verified; `RT-workspace-active-session-badge` is `pass` / `retest_status: pass`.
- **Goal judge and active clear accepted after re-found controls:** fresh Goal `looprun-e6830bc6fd4a086f` re-found unconstrained tool use and a Clear that admitted a successor. After the authority/fencing correction, `looprun-a6a4368bf1fc8c49` completed in one turn with a strict approved verdict and zero judge tools. Active-judge Clear on `looprun-0d9a6cc9afa3e92e` stopped the exact judge as `user_canceled`, left zero active system sessions, admitted no successor, and rendered the typed cause/recovery. `GL-judge-session-contract` and `GL-019` pass; GL-004's exact two-rejection macro remains explicit broader coverage.
- **AGH-71 / task hierarchy and matching wake accepted:** fresh parent `task-a2b46ce593b5e75b` naturally reached Needs Attention with no bound session. Child A completed once and left it nonterminal; child B completed once and atomically settled parent run `run-b0985a94beb209b9`. Reload plus Children/Events/Run views proved one parent run, one run/session per child, both children Completed, and no parent worker session. A second matching tree then woke `looprun-adc2f42e8e7c60d5` exactly once on parent completion and created one follow-up. `TA-parent-rollup-completion`, `TA-task-role-session-activation`, and `LP-task-rollup-wakes-loop` pass.
- **Transcript identity and reload accepted:** pre-fix session `sess-3fb644eedaea5ab9` duplicated and reordered ordinary user rows during reconciliation, then lost its `/goal` command on reload. Fresh post-fix Cursor/Grok session `sess-59296138935045ea` completed two ordinary turns and approved Goal `looprun-3724bede0e0e62f5`. A real permalink reload retained every authored input exactly once and in strict chronology before its assistant work. `BUG-20260713-session-user-message-reorders-or-disappears` is verified and `RT-session-message-reload` is pass/retest pass.
- **Task enqueue latency accepted:** the pre-fix run-enqueue request held its 201 response for 19.822 seconds while provisioning Cursor. Post-fix Task `task-6087b6cffe877fb7` and run `run-1056af4670c94d26` returned in 2 ms/3 ms; route navigation happened immediately, then the same run transitioned waiting → running → completed with real session `sess-12b2c865a27ecc72` without reload.
- **Session-history deletion accepted (historical pre-rebase lab):** cleanup of that real task-role session first returned HTTP 500 because five permission rows and its token statistics blocked the session foreign key. The then-current Automation migration v2 preserved existing rows and changed those owned relationships to cascade. The same UI modal then succeeded; fresh HTTP read returned 404 and all three target counts were zero. In the merged schema, Hermes owns immutable v2 and this cascade is v3 (`00003_schema.sql`).
- **Final ownership and live-event replay accepted:** consolidated peer review found four residual integration gaps after the broad Browser pass. Task-role activation now drains before the daemon's session-stop snapshot; workspace pruning stages stopped session directories before committing catalog cascades; individual session deletion uses the same pre-commit tombstone boundary; and the Web EventSource registers every persisted named Task event. Final Browser replay created/deleted real Cursor/Grok session `sess-6a4f5db74d195230`, pruned `workspace-prune-final` together with stopped session `sess-0f2fc3f71bf6b69e`, and observed Task `task-5a7465009a4f277a` pause/resume in 380 ms/322 ms without reload. The disposable Task was canceled and deleted through the UI.

The user reopened the in-app Browser, the controller rebound a clean tab, and the post-fix lab remained active through the final cleanup. Only scenarios with completed live evidence were promoted; four explicitly listed broad matrix charters are Skipped rather than inheriting a remediation verdict.

## Session Debriefs

### CH-001 — Lea

- **Ran:** 2026-07-13T05:00Z → 2026-07-13T05:03Z; retested 2026-07-13T06:16Z (boxes respected: yes)
- **Findings:** The original real `software-delivery` run reached truthful terminal `Stalled` after two failed-only `load_tasks` attempts but hid the deterministic cause. After the typed/redacted failure fix and isolated daemon restart, browser-created run `looprun-b165c15b174e3d40` preserved and rendered the exact safe missing-pattern cause plus a concrete recovery instruction beneath both failed nodes.
- **Bugs filed/updated:** BUG-20260713-loop-failure-hidden → verified.
- **Scenarios settled:** LP-action-failure-detail → pass after retest; LP-001 and LP-002 remain pending until a valid task-set run is replayed.
- **Paper cuts:** None remain on the deterministic missing-taskset failure path.
- **Surprises:** The Loop continues to avoid a false `Done`; failure transparency was restored without weakening the honest terminal outcome or leaking an absolute path.
- **Suggested next charter:** Execute `software-delivery` with a valid task set and live Cursor runtime after the Cursor Agent-mode blocker is fixed.

### CH-008 — Lea

- **Ran:** 2026-07-13T04:58Z → 2026-07-13T05:02Z (box respected: yes)
- **Findings:** Valid inputs rendered the generation-1 plan in 277 ms with five resolved inputs and eight nodes; the required-empty state disabled both actions before submission.
- **Bugs filed/updated:** None.
- **Scenarios settled:** LP-006 → pass; LP-007 → pass.
- **Paper cuts:** The empty required-input state is clear and immediate; no sharp friction.
- **Surprises:** The dry-run confirmation explicitly states that no run was created and no budget was spent, and a fresh detail read independently confirmed zero runs.
- **Suggested next charter:** CH-001, using the same Loop for a real execution.

### CH-006 — Bruno

- **Ran:** 2026-07-13T05:04Z → 2026-07-13T05:10Z; retest 2026-07-13T06:55Z → 2026-07-13T07:08Z (box respected: yes)
- **Findings:** Cancel discarded unsaved edits; all six numeric fields clamped at daemon ceilings; no cost-cap input exists; structural fields remained outside Configure; saved values round-tripped. The original run honored cap 3/full-body while detail and preview advertised 50/default. After the fix, detail rendered cap 3, escalate, no-progress 2, fan-out 4, and gate revisions 2; Run rendered 3/full-body, a temporary cap 4 updated badge and preview to 4, and Cancel/reopen restored 3.
- **Bugs filed/updated:** BUG-20260713-loop-preview-ignores-config → verified.
- **Scenarios settled:** LP-017 → pass after fix/retest; LP-018 → pass; LP-019 → pass; LP-020 → pass.
- **Paper cuts:** Configure labels its first group `Review gate` but reports no verification checks despite the definition containing review/verify gates; this remains under investigation rather than a separate bug until the intended configurability contract is confirmed.
- **Surprises:** Runtime effective-config resolution was already correct; the first candidate fix exposed a second stale projection only when a real user changed a per-run override, which the controller UI retest caught before acceptance.
- **Suggested next charter:** CH-007, then continue the complete custom Loop with/without Goal and delete lifecycle.

### CH-007 — Bruno

- **Ran:** 2026-07-13T05:10Z → 2026-07-13T05:12Z; retest 2026-07-13T08:29Z → 2026-07-13T09:00Z (box respected: yes)
- **Findings:** The entry action initially failed deterministically: `Fork & edit` posted to the workspace Loop collection, received HTTP 500, and exposed only a generic toast despite leaving a copied definition on disk. After the schema-aware atomic-fork fix, extension-backed `reviews-watch` opened in the builder, preserved ToolIDs, and correctly gated an intentional broken reference. The replay found and fixed a v0 CAS boundary residual; the final pass persisted a 45s Watch spec, published v0 → v1, passed dry-run, started a real run, and survived fresh reads.
- **Bugs filed/updated:** BUG-20260713-loop-fork-internal-error → verified.
- **Scenarios settled:** LP-021 → pass after fix; LP-024 → pass after fix. LP-022's `unknown_reference` branch behaved correctly but its exact fan-out-ceiling case remains pending; LP-023 remains pending.
- **Paper cuts:** The error toast carries neither a cause nor a retry/recovery action.
- **Surprises:** The original 500 happened after the filesystem copy, not before it; the first browser replay then found that zero is a valid published version in the builder but was treated as absent by the Publish contract.
- **Suggested next charter:** Continue the now-unblocked custom lifecycle into contract goal removal/addition and workspace-shadow deletion.

### CH-loop-goal-delete — Bruno

- **Ran:** 2026-07-13T08:36Z → 2026-07-13T09:18Z; retested through 2026-07-13T09:24Z (box respected: yes)
- **Findings:** The corrected fork published goal-bearing `reviews-watch` v1 and started `looprun-7e6dbcacdf292853`. The remediation added a writable Contract rail and workspace-only typed confirmation. Bruno cleared the goal, changed the definition of done, published, and confirmed fresh goal-less projections. A strict second pass recreated the shadow, published goal-less v1, started real `looprun-c0e322b615e43c12`, observed no stale Goal in the run, stopped it, deleted the shadow by exact-name confirmation, and recovered bundled read-only v0 from a fresh catalog read.
- **Bugs filed/updated:** BUG-20260713-loop-fork-internal-error → verified; BUG-20260713-loop-contract-goal-not-editable → verified; BUG-20260713-custom-loop-delete-missing → verified.
- **Scenarios settled:** LP-021 → pass after fix; LP-024 → pass after fix; LP-toggle-loop-goal → pass after fix; LP-delete-custom-loop → pass after fix. LP-022's exact fan-out-ceiling case and LP-023 position sidecar remain pending.
- **Paper cuts:** None remain on the contract authoring and workspace-shadow deletion path. Workspace-owned surfaces now say `Edit`; read-only sources retain `Fork & edit`.
- **Surprises:** Deleting a workspace shadow correctly revealed the bundled source without deleting historical runs; the strict replay retained both earlier run links while restoring read-only v0.
- **Suggested next charter:** Complete automation start-binding compatibility and destructive confirmation, then exercise the remaining exact fan-out and layout-sidecar edges.

### CH-automation-crud-loop-target — Bruno

- **Ran:** 2026-07-13T05:12Z → 2026-07-13T05:16Z; retests through 2026-07-13T10:34Z (boxes respected: yes)
- **Findings:** The target-aware correction passed create/edit/detail/Run-now for `software-delivery` and linked delegated history to `looprun-e39eb7e8d36ffa7b`. The residual pass filtered Job to schedule-capable `software-delivery` and Trigger to `reviews-watch` for ordinary and webhook starts. Job Delete passed dialog, Cancel, wrong-name, exact-name, and one-shot removal. The final Trigger repair passed create and detail read-back for workspace `ws_06366aad69887872`, `data.session_type=system`, `reviews-watch`, and typed `pr=2`. Re-enabling the Trigger and stopping the correlated real task-role session created exactly one additional delegated run, `looprun-4bc3d180d2edd5ba`. The corrected generation-zero watch failure automatically settled Failed with the safe source cause and recovery, zero attempts, and no operator Stop. Exact-name deletion then removed the Trigger; a fresh catalog rendered zero objects.
- **Fresh control:** On 2026-07-14, a newly created `session.stopped` Trigger filtered `data.session_type=user`, targeted `reviews-watch` with `pr=2`, and fired once from real Cursor/Grok session `sess-19c9b67078687781`. Delegated `looprun-a52b84b65ffba9b6` failed in 0s at generation/attempt zero with the same typed cause/recovery before and after reload. Exact-name deletion restored zero Triggers, and the disposable session was deleted.
- **Bugs filed/updated:** BUG-20260713-loop-automation-shown-as-agent → verified; BUG-20260713-loop-automation-start-mismatch-late → verified; BUG-20260713-automation-delete-no-confirmation → verified for Job and Trigger; BUG-20260713-workspace-trigger-loop-submit-inert → verified; BUG-20260713-loop-watch-poll-error-stuck → verified.
- **Scenarios settled:** TA-automation-crud-loop-target → pass across create/read/edit/disable/re-enable/exactly-once dispatch/history/delete; LP-035 proactive start-kind truth → pass for schedule, trigger, and webhook options. The downstream failure now proves the negative generation-zero path without a zombie Run.
- **Paper cuts:** None retained separately; the unsafe deletion and late compatibility failure are now tracked defects.
- **Surprises:** The shared history projection now made the runtime correlation directly navigable, exposing the delegated Loop's typed failure path without API inspection.
- **Suggested next charter:** None for this lifecycle invariant. The remaining CH-038 schedule/filter/failure-policy matrix is broader coverage, not a remediation dependency.

### CH-new-session-latency-title — Bruno

- **Ran:** 2026-07-13T05:20Z → 2026-07-13T05:34Z (box respected: yes)
- **Findings:** The selector/onboarding values `cursor-grok-4.5-high` and `grok-4.5` both negotiated only after a 14.7–19.5 second wait and persisted failed sessions, even though the failure listed `grok-4.5[effort=high,fast=true]` as valid. Manually entering that hidden descriptor started live session `sess-b1c980b86709053d`, but click-to-composer-ready was still approximately 18.4 seconds. Two substantive completed turns never changed the generic `general` title.
- **Bugs filed/updated:** BUG-20260713-cursor-model-startup-contract; BUG-20260713-background-session-indicator-title (AGH-84).
- **Scenarios settled:** RT-new-session-fast-feedback → fail; RT-session-auto-title → fail.
- **Paper cuts:** The failure alert lists every provider model in one unbounded block and can push modal actions below the viewport.
- **Surprises:** The failed session remains inspectable and truthfully reports `protocol_failure`; the successful Cursor/Grok turn itself completed normally once the exact descriptor was forced.
- **Suggested next charter:** Retest cold and warm startup after catalog canonicalization/preflight, then verify the title after first turn, refresh, and list navigation.

#### Live session-startup continuation

- **Ran:** 2026-07-13T11:18Z → 2026-07-13T11:22Z (box respected: yes)
- **Findings:** Two fresh sessions were created entirely through the modal with the visible canonical `Grok 4.5 (High, Fast)` choice. On the instrumented replay, pending feedback was still truthfully present at 6.027 seconds because the provider request had not completed. HTTP 201 arrived 6.452 seconds after click; destination loading began 7 ms later, and session `sess-c09b90c914321946` rendered a usable composer without the creation dialog or runtime selector. The old ~17.7-second post-success overlay delay did not recur.
- **Bugs filed/updated:** BUG-20260713-cursor-model-startup-contract → verified; BUG-20260713-new-session-modal-lingers → verified. AGH-84 automatic title remains open.
- **Scenarios settled:** RT-new-session-fast-feedback → pass; RT-session-auto-title remains fail pending its dedicated repair.
- **Paper cuts:** The model catalog remains open after option selection, although one click on `Start session` still submits correctly; this did not block completion and was not classified separately in this pass.
- **Surprises:** The 6-second checkpoint initially looked slow, but daemon correlation proved that it preceded ACP confirmation by about 425 ms. The corrected UI preserves truthful startup feedback rather than hiding real provider latency.
- **Suggested next charter:** Exercise automatic title and cross-workspace active-session discovery with simultaneous live sessions after the AGH-84 repair.

#### Post-fix first-prompt continuation

- **Ran:** 2026-07-13T20:21-03:00 → 2026-07-13T20:24-03:00 (box respected: yes)
- **Findings:** Fresh Cursor/Grok session `sess-e74df4386f8d5a77` returned HTTP 201 in 14.764 seconds: local sandbox synchronization consumed 8.885 seconds and ACP startup consumed 5.758 seconds. After the modal disappeared, the first `/goal` prompt rendered optimistically as `Working…` for more than 64 seconds, but the daemon remained healthy/idle with `active_prompt=false`, Goal `null`, a two-entry creation-only transcript, and no prompt POST.
- **Bugs filed/updated:** BUG-20260713-first-prompt-optimistic-stuck opened; RT-new-session-fast-feedback reopened.
- **Scenarios settled:** RT-new-session-fast-feedback → fail; GL-004 remains blocked pending the first-message handoff repair.
- **Paper cuts:** The selected model catalog still renders the active Cursor model twice and stays expanded after selection.
- **Surprises:** The UI presented a stop-generation state even though the authoritative runtime had no active prompt to stop.
- **Suggested next charter:** Fix the first-message handoff at its owning client/runtime boundary, then replay Goal success and active-judge Clear from a clean session.
- **Diagnostic continuation:** The serialized GPT-5.6-SOL/high investigation rejected the first navigation hypothesis using the authoritative 40.103-second timing. The canonical destination-runtime test then reproduced the exact hook-only epoch-0/generation-0 transcript and passed with exactly one `/prompt` fetch. All diagnostic edits were removed. The bug remains open pending a clean renderer replay because no faithful source-level red exists yet.
- **Second live reproduction:** Fresh session `sess-8eb726b62df96bb3` repeated the same zero-POST failure when its first `/goal` was sent 37.190 seconds after Start, after the destination composer was stable. Console evidence showed the normal StrictMode SSE open/cleanup/open sequence with no error. A faithful StrictMode provider replay still reached the prompt fetch exactly once, so the live-only pre-fetch boundary remains unconfirmed and no speculative transport fix was retained.
- **Coupled Stop defect:** The same live attempt proved `Stop generation` could not unwind assistant-ui when the daemon had admitted no prompt. `BUG-20260713-stop-generation-local-stuck` is source-fixed at the local run owner with a canonical StrictMode red/green, but remains pending Browser retest. This correction is explicitly separate from the still-open first-prompt bug.

#### Automatic-title continuation

- **Ran:** 2026-07-13T14:47Z → 2026-07-13T14:55Z (box respected: yes)
- **Findings:** Two sessions were created entirely through the UI with visible `Grok 4.5 (High, Fast)` and meaningful real-user prompts. Primary session `sess-5ec18f5f2a13fe16` persisted `Investigate why checkout webhook retries create duplicate autom…`; bench session `sess-40e90687024bfb24` persisted `Review bench operations alert routing and identify one…`. The primary title remained identical after direct permalink reload and appeared in the exact inactive-workspace return affordance.
- **Bugs filed/updated:** BUG-20260713-background-session-indicator-title → automatic-title branch verified; its background-return branch remains open through BUG-20260713-cross-workspace-session-return-hangs.
- **Scenarios settled:** RT-session-auto-title → pass; RT-workspace-active-session-badge remains fail pending reciprocal-return replay.
- **Paper cuts:** None on automatic identity; the title is concise and does not expose raw IDs or evaluator language.
- **Surprises:** The title was durable before the first workspace switch completed, proving daemon-owned persistence rather than a transient client label.
- **Suggested next charter:** Repeat the corrected reciprocal return, then stop/delete one user session and verify the exact inactive-workspace count decrements.

### CH-background-session-switch — Théo

- **Ran:** 2026-07-13T05:35Z → 2026-07-13T05:39Z (box respected: yes)
- **Findings:** A second real Grok turn continued while the operator switched to `bench-ops` and completed in 9 seconds; returning restored the full transcript. The switch initially emitted a useful `Session belongs to workspace ...` toast, but after it expired the owning workspace button carried no active-session count or state. Only after switching back did the scoped Agents surface show `1/4` and the live session.
- **Bugs filed/updated:** BUG-20260713-background-session-indicator-title (AGH-84).
- **Scenarios settled:** RT-workspace-active-session-badge → fail; RT-041 background execution/route isolation branch → pass; RT-045 transcript return branch → pass.
- **Paper cuts:** Reopening the running session from the agent overview used a raw id because no automatic title existed.
- **Surprises:** Runtime continuity and workspace data isolation were sound; the defect is discoverability/state aggregation, not session termination or transcript loss.
- **Suggested next charter:** Repeat with two simultaneous sessions in different workspaces after the badge/title fix.

#### Final-replay internal-session exclusion residual

- **Ran:** 2026-07-13T20:12Z → 2026-07-13T20:18Z (box respected: yes)
- **Findings:** After completing onboarding with Cursor/Grok 4.5, Théo created one meaningful user session in `agh3`, waited for its durable automatic title, and switched to `bench-ops`. The inactive workspace return link truthfully targeted that session but displayed an exact count of `2`. The public session catalog confirmed that the extra row was the onboarding agent session, persisted as `type=user` rather than an internal class.
- **Bugs filed/updated:** BUG-20260713-background-session-indicator-title re-found; no duplicate bug minted.
- **Scenarios settled:** RT-workspace-active-session-badge → fail; title and return-target identity remain correct.
- **Paper cuts:** The count is plausible enough to mislead operators into looking for a second user session that does not exist.
- **Surprises:** The new public `type=user` filter behaved exactly as implemented; the remaining defect is upstream classification at onboarding session creation.
- **Suggested next charter:** Re-run onboarding in a fresh post-fix lab, create one user session, and prove the inactive-workspace count is `1` before completing reciprocal return, stop/delete decrement, and reconnect.

#### Post-fix AGH-84 acceptance

- **Ran:** 2026-07-13T20:39Z → 2026-07-13T20:51Z (boxes respected: yes)
- **Findings:** Fresh onboarding created `sess-8101d12c9aaa4db0` as `system`. One real authored Cursor/Grok 4.5 session in `agh3` produced an automatic title and exact inactive badge `1`; the all-type active catalog contained both sessions while `type=user` contained only the authored one. A second real session in `bench-ops` produced its own title and transcript. Repeated direct Return clicks in both directions opened the exact target session without stale identity or Loading. Stopping the bench session removed its badge; deleting it through the confirmation modal reset Sessions to `0`, returned HTTP 404 for its id, and reduced the user-only active catalog to total `1`.
- **Bugs filed/updated:** BUG-20260713-background-session-indicator-title → verified; BUG-20260713-cross-workspace-session-return-hangs → verified.
- **Scenarios settled:** RT-workspace-active-session-badge → pass/retest pass; RT-session-auto-title remains pass.
- **Paper cuts:** One non-reproduced Return click settled on the destination agent overview rather than its session, without stale content or a hang; immediate direct repeats in both directions reached the exact sessions.
- **Surprises:** Internal onboarding remains inspectable through the all-type operational catalog without leaking into user activity indicators, proving classification rather than suppression.
- **Suggested next charter:** None for AGH-84; task-role and Goal-judge internal-session exclusions remain covered by their owning workflows.

### CH-use-response-as-goal — Lea

- **Ran:** 2026-07-13T05:34Z → 2026-07-13T05:35Z (box respected: yes)
- **Findings:** `Use as Goal` on a substantive real-provider response was inert through both pointer and keyboard activation; no draft, Goal chip, route, toast, or error appeared.
- **Bugs filed/updated:** BUG-20260713-use-as-goal-inert.
- **Scenarios settled:** GL-use-response-as-goal → fail.
- **Paper cuts:** None separate from the blocking action.
- **Surprises:** The same response action was rendered on each assistant turn, but neither produced any observable state.
- **Suggested next charter:** After the fix, replay response → Goal draft/start/cancel and then continue the full CH-046 Goal lifecycle with the same live session.

#### Use-as-Goal remediation replay

- **Ran:** 2026-07-13T14:31Z → 2026-07-13T14:44Z (box respected: yes)
- **Findings:** On live Cursor/Grok session `sess-7842125cce618d86`, a real pointer click prefixed the selected response exactly once, focused the composer, and rendered `Goal command draft` without submitting a prompt or creating a Goal. A pre-existing authored draft was preserved with actionable warning, and Discard returned the composer to empty with no transcript side effect. Bare and oversized `/goal` requests now render human guidance while retaining a reusable composer and hiding both machine reason codes.
- **Bugs filed/updated:** BUG-20260713-use-as-goal-inert → verified; BUG-20260713-goal-errors-expose-reason-code → verified.
- **Scenarios settled:** GL-use-response-as-goal → pass. Invalid-objective branches of GL-003 → pass for side-effect-free rejection and actionable recovery.
- **Paper cuts:** The in-app browser driver could not focus this virtualized response action with its direct keyboard API; keyboard behavior is therefore supported by the production Storybook/runtime integration evidence rather than a second live-provider keystroke.
- **Surprises:** The visible action could be invoked reliably with the browser's real DOM pointer path even though the higher-level locator click missed the virtualized row; the resulting production state was exact and deterministic.
- **Suggested next charter:** Retest the constrained Goal judge and typed active-clear failure after the daemon rebuild.

### CH-cursor-agent-mode — Bruno

- **Ran:** 2026-07-13T05:33Z → 2026-07-13T05:34Z (box respected: yes)
- **Findings:** Cursor/Grok read the isolated workspace and produced a detailed release-risk draft, but refused the requested file creation because `Ask mode is on`. AGH exposed no mode control, so the provider's own recovery instruction was impossible to follow.
- **Bugs filed/updated:** BUG-20260713-cursor-agent-mode-unavailable.
- **Scenarios settled:** RT-cursor-agent-mode → fail.
- **Paper cuts:** None separate from the blocked write path.
- **Surprises:** Read-only reasoning was high quality and grounded in real files, isolating the failure to operating-mode configuration rather than provider connectivity.
- **Suggested next charter:** Select a writable Cursor mode in the fixed runtime selector and independently verify the requested report on disk.

### CH-046 — Lea

- **Ran:** 2026-07-13T05:42Z → 2026-07-13T05:43Z (box respected: yes; stopped at blocking entry prerequisite)
- **Findings:** A valid plain `/goal` command submitted from the established live Cursor/Grok session returned only `goal_judge_unavailable`. No Goal chip, Run, draft, or recovery guidance appeared, so convergence/control cases could not begin.
- **Bugs filed/updated:** BUG-20260713-goal-judge-unavailable.
- **Scenarios settled:** GL-001 → fail; GL-003 unavailable-judge branch → fail because the code is deterministic but not actionable; GL-002 and GL-004..GL-013 remain blocked behind start.
- **Paper cuts:** The raw underscore code is rendered as the entire error message.
- **Surprises:** Goal interception itself was immediate and did not send the slash command to Cursor, suggesting the failure occurs during judge resolution before provider work.
- **Suggested next charter:** Retest the same command after judge availability/config recovery, then continue the original CH-046 convergence/replace/draft matrix.

#### Live Goal lifecycle continuation

- **Ran:** 2026-07-13T10:26Z → 2026-07-13T10:48Z (box respected: yes)
- **Findings:** Canonical Cursor/Grok session `sess-7842125cce618d86` created durable Goal Run `looprun-5a1acf5934fef596` in 474 ms. Turn 1 used `agh__goal_get`, `agh__loop_turns`, and `agh__goal_report`, then paused at a clean boundary. Resume created one continuation. Two completion reports were rejected only because their command-judge responses contained no JSON; Turn 3 correctly reported an evidenced external block rather than looping forever. The session and Run exposed the same objective, 3/20 turns, Blocked state, `goal_reported_blocked` cause, last verdict, and evidence. Clearing the settled Goal removed its live status without deleting the transcript or historical Run.
- **Control edge:** A second Goal, `looprun-1667f72b7cdb7128`, was cleared while a continuation was in flight. The session correctly revoked control and removed the Goal status; the turn became Ambiguous with cause `goal_control_revoked_in_flight`. The Run nevertheless projected a generic action failure and generic recovery, losing the durable revocation cause at the top level.
- **Bugs filed/updated:** BUG-20260713-goal-judge-unavailable → start path verified after model fallback repair; BUG-20260713-goal-judge-unconstrained-leaks-session → open; BUG-20260713-goal-clear-generic-failure → open.
- **Scenarios settled:** GL-001, GL-002, pause/resume, blocked settlement, settled clear, and active control revocation were exercised with a live provider. Goal completion remains blocked on the command-judge contract; active-clear terminal projection remains blocked on truthful cause/recovery.
- **Surprises:** Three judge sessions (`sess-37f86bd295697c71`, `sess-f49398af9db4c77d`, and `sess-14af1951acb1bcae`) ran as unrestricted `general` Cursor system sessions and remained ACTIVE after their criteria. The first two made 90 and 68 unrelated tool calls while ignoring the exact-JSON instruction.
- **Suggested next charter:** Retest with a constrained verdict-only judge and mandatory temporary-session cleanup, then prove one successful `Done` settlement and one clear-in-flight terminal outcome without duplicate turns or leaked sessions.

#### Post-fix Goal judge and Clear rejection

- **Ran:** 2026-07-13T20:55Z → 2026-07-13T21:00Z (box respected: yes)
- **Findings:** Real Cursor/Grok 4.5 Goal `looprun-e6830bc6fd4a086f` produced two concise work responses satisfying the textual criterion, but both durable turns were rejected as `judge_output_malformed`. The second temporary judge was `system`, `goal_judge`, and eventually stopped, yet its audit contains 60 tool calls, 30 tool results, 105 thoughts, and malformed output. Clicking Clear during that judge left the control Loading and admitted turn 3, which used Goal/Loop tools before `Stop generation` contained it; the connected chip stayed active and permalink reconnection timed out.
- **Bugs filed/updated:** BUG-20260713-goal-judge-unconstrained-leaks-session → reopened; BUG-20260713-goal-clear-generic-failure → reopened.
- **Scenarios settled:** GL-004 → fail; GL-judge-session-contract → fail; GL-019 → fail.
- **Paper cuts:** The chip's static verification sentence looked successful while the authoritative verdict was rejected, increasing ambiguity during the stuck control.
- **Surprises:** Temporary-session cleanup now occurs, but Ask mode plus the intended empty policy did not constrain Cursor tools; cleanup alone is insufficient.
- **Suggested next charter:** Repair the real provider capability/output boundary and fence Clear before successor admission, then replay one approved Goal and one active Clear from a fresh session.

#### Final catalog, Goal judge, Clear, and Stop acceptance

- **Ran:** 2026-07-13T22:40-03:00 → 2026-07-13T23:13-03:00 (box respected: yes)
- **Findings:** The browser-proven first-prompt stall came from exhausting six HTTP/1.1 connections with Vite console, workspace logs, three per-workspace session catalogs, and the transcript. The rebuilt app kept Vite and logs separate while replacing only the catalog trio with one global workspace-tagged stream, leaving four active connections on the session route. Fresh Cursor/Grok session `sess-2a768148b6106dc3` submitted its first `/goal` exactly once four milliseconds after click; `looprun-a6a4368bf1fc8c49` completed in one turn with a strict approved verdict and no judge tools. Stop generation then canceled both daemon and local runtime and restored an editable composer.
- **Clear projection:** Initial active-judge Clear `looprun-d8466636e525f1e5` proved join/fencing/cleanup but exposed the remaining generic `loop_action_failed`. After the executor authority correction and rebuild, one-process Browser Use replay `looprun-0d9a6cc9afa3e92e` observed judge `sess-4afdada5589b5fed` active at click, cleared the Goal, stopped the judge as `user_canceled`, left zero active system sessions, admitted no successor generation, and rendered the typed cause and recovery on Run detail.
- **Bugs filed/updated:** BUG-20260713-first-prompt-optimistic-stuck → verified; BUG-20260713-stop-generation-local-stuck → verified; BUG-20260713-goal-judge-unconstrained-leaks-session → verified; BUG-20260713-goal-clear-generic-failure → verified.
- **Scenarios settled:** GL-judge-session-contract → pass; GL-019 → pass. RT-new-session-fast-feedback remains failed only for its separate startup-latency budget; GL-004's exact two-rejection live walk remains outstanding even though its judge authority defect is verified.
- **Evidence:** `qa/network/catalog-global-goal-acceptance.json`, `qa/screenshots/catalog-global-goal-approved.png`, `qa/screenshots/stop-generation-composer-ready.png`, `qa/screenshots/goal-clear-typed-third-during-judge-before.png`, `qa/screenshots/goal-clear-typed-third-after.png`, and `qa/screenshots/goal-clear-typed-run-detail.png` under the active post-onboarding-fix lab.
- **Suggested next charter:** Rewalk only the exact GL-004 two-rejection convergence sequence; no further remediation is required for the four defects closed by this batch.

### CH-task-tree-loop-rollup — Bruno

- **Ran:** 2026-07-13T05:45Z → 2026-07-13T05:55Z; partial retests through 2026-07-13T10:35Z (box respected: yes)
- **Findings:** The original UI tree exposed the direct AGH-71 failure after three real child completions. The faithful post-fix tree removed historical contamination: unavailable exact ownership kept the parent run unbound; two new Cursor/Grok children each claimed and completed one run; the first child left the parent nonterminal and the final child settled the parent run and Task exactly once. Two approval-gated controls also retained one pre-enqueued run through Inbox approval and real execution.
- **Bugs filed/updated:** BUG-20260713-task-owner-cannot-clear, BUG-20260713-task-role-session-never-starts, BUG-20260713-task-role-dispatch-repeats, BUG-20260713-needs-attention-recovery-hidden, BUG-20260713-parent-task-rollup-missing, BUG-20260713-task-approval-duplicates-open-run, and BUG-20260714-terminal-task-run-reported-orphan are fixed/browser-verified.
- **Scenarios settled:** TA-004, TA-033, TA-041, TA-task-role-session-activation, TA-parent-rollup-completion, TA-terminal-run-inspect, LP-task-rollup-wakes-loop, and TA-task-create-async-activation pass.
- **Paper cuts:** The Task list and Inbox use the same top-level navigation button; the first Inbox click from detail returns to List before a second click activates Inbox.
- **Surprises:** Approval of the disposable task caused its existing task-role Cursor session to claim and complete in nine seconds without any workspace write, proving the earlier zero-iteration sessions can recover when the contract becomes claimable.
- **Suggested next charter:** None for the hierarchy, wake, or async-enqueue invariants; retain them as one integrated regression macro.

#### Matching Loop wake, live state, and asynchronous enqueue acceptance

- **Ran:** 2026-07-14T01:53-03:00 → 2026-07-14T03:49-03:00 (boxes respected: yes)
- **Findings:** Browser-authored `reviews-watch` preserved its `watch_events.events` DTO. Run `looprun-adc2f42e8e7c60d5` stayed Watching after child A, woke exactly once only when child B completed parent `task-85c91299de1c8af4`, executed `watch_events → create_followup → fetch_issues`, reached Done, and created exactly one follow-up Task. A separate live-state Task moved Pending → Running → Completed without reload after dot-name SSE invalidation was corrected. The first ready pool Task replay then isolated a 19.822-second synchronous enqueue response; after remediation, Task/run creation returned in 2 ms/3 ms while the same run provisioned and completed asynchronously.
- **Bugs filed/updated:** BUG-20260714-task-create-waits-for-worker-session → verified; BUG-20260714-task-named-events-stale → verified; Agent visible-session population was corrected at its owning Web data boundary during the same replay.
- **Scenarios settled:** LP-task-rollup-wakes-loop → pass; TA-task-create-async-activation → pass; TA-016 live-state branch → pass.
- **Cleanup edge (historical pre-rebase lab):** Deleting the accepted real task-role session exposed BUG-20260714-session-delete-history-fk; the then-current Automation v2 ownership migration was applied to the live v1 database and the original modal then passed. The merged implementation carries that ownership change in v3.
- **Suggested next charter:** Retain this integrated macro when task-role provisioning, watch sources, or Task SSE vocabulary changes.

### CH-task-template-draft — Bruno

- **Ran:** 2026-07-13T05:44Z → 2026-07-13T05:45Z; fixed retest 2026-07-13T10:30Z → 2026-07-13T10:32Z (box respected: yes)
- **Findings:** The original replay lost both authored fields on `Break into steps`. After the repair, `QA retained template draft` and its description survived the preset switch plus Simple → Advanced → Simple, and the modal was cancelled without creating a task.
- **Bugs filed/updated:** BUG-20260713-task-template-clears-draft fixed and verified.
- **Scenarios settled:** TA-task-template-preserves-draft → pass.
- **Paper cuts:** The modal gives no dirty-state/reset warning.
- **Surprises:** Parent placement entered by exact task id in Advanced mode worked without an additional suggestion selection.
- **Suggested next charter:** None for this invariant; automated canonical coverage retains the all-preset matrix.

### CH-prune-missing-workspace — Bruno

- **Ran:** 2026-07-13T06:22Z → 2026-07-13T06:25Z; fixed retest 2026-07-13T10:38Z → 2026-07-13T10:40Z (box respected: yes)
- **Findings:** Registered lab-owned folder `ghost-prune-probe` through the modal, confirmed it was active and counted, removed only that folder, and reproduced the ghost across Web, HTTP, and UDS CLI. After remediation, the first Web catalog read pruned the missing registration, all three public catalogs agreed, direct read returned 404, and a second daemon restart preserved the deletion.
- **Bugs filed/updated:** BUG-20260713-missing-workspace-persists (AGH-47) fixed and verified.
- **Scenarios settled:** RT-missing-workspace-pruned → pass.
- **Paper cuts:** The error page contains no action to switch to or prune a valid fallback; recovery depends on noticing the workspace rail.
- **Surprises:** The read path already distinguishes a missing root with HTTP 410, but list/reconciliation intentionally or accidentally preserves the same invalid registration.
- **Suggested next charter:** None for this invariant; canonical resolver suites retain transient-error and concurrent-list controls.

### CH-session-transcript-reload — Théo

- **Ran:** 2026-07-13T23:38-03:00 → 2026-07-14T00:52-03:00 (box respected: yes)
- **Findings:** The pre-fix thread duplicated and reordered both ordinary user messages during live reconciliation; reload removed those duplicates but erased the structured `/goal` input entirely. After remediation and rebuild, fresh Cursor/Grok session `sess-59296138935045ea` rendered `QA-RELOAD-ONE-0714-0049`, `QA-RELOAD-TWO-0714-0050`, and the exact `/goal` command once each before their responses. Goal `looprun-3724bede0e0e62f5` reached Approved after two real turns. Reloading the exact permalink kept all three authored inputs exactly once, with DOM offsets proving strict chronology.
- **Explicit second retest (historical pre-rebase lab):** After that branch's final daemon rebuild and then-current global v2 migration, the user-requested reload completed in 874 ms. The two ordinary prompts and exact `/goal` command each remained present once and retained their order (`orderPreserved=true`, `allExactlyOnce=true`).
- **Bugs filed/updated:** BUG-20260713-session-user-message-reorders-or-disappears → verified.
- **Scenarios settled:** RT-session-message-reload → pass/retest pass.
- **Paper cuts:** None remained after the identity-based promotion and durable Goal ingress correction.
- **Surprises:** The ordinary and structured-command symptoms shared one visible failure but had two persistence boundaries: missing client identity caused optimistic duplication/reordering, while pre-record dispatch caused `/goal` loss.
- **Suggested next charter:** Retain the same three-message live/reload macro when transcript pagination or assistant-ui runtime ownership changes.

## What Was Fixed

- **BUG-20260713-background-session-indicator-title / onboarding residual:** The manager now reconciles the final class after resolving the managed agent and before lineage/metadata/catalog persistence. The red regression observed onboarding runtime and metadata as `user` and a filtered total of two; green proof shows onboarding `system`, the normal session `user`, exact filtered total one, and both rows in the all-type operational view. Controller verification passed the directed case, the complete `internal/session -race` package in 38.068s, the four-test Web onboarding hook suite, `gofmt -d`, `git diff --check`, and scoped lint with only the existing `query_store.go` baseline finding. Fresh Browser retest then proved exact badge `1`, reciprocal Return, reconnect, stop decrement, durable delete, and public-catalog convergence with real Cursor/Grok 4.5 sessions.

- **BUG-20260713-loop-failure-hidden:** GPT-5.6-SOL/high worker propagated a typed operator-safe action failure from the bundled extension through JSON-RPC, daemon redaction, globaldb persistence, and the Web timeline. Same-persona UI retest passed in `looprun-b165c15b174e3d40`; focused daemon/globaldb race suites passed.
- **BUG-20260713-loop-preview-ignores-config:** GPT-5.6-SOL/high worker aligned Loop detail and Run form with saved workspace-scoped configuration, then reused the exact per-run request projection in the live preview after controller dogfooding found a stale override edge. Same-persona UI retest passed for baseline 3, temporary override 4, and Cancel/reopen restoration; 3,326 Web tests passed.
- **BUG-20260713-loop-contract-goal-not-editable:** GPT-5.6-SOL/high worker added contract goal/definition-of-done authoring to the canonical Loop draft and preserved CAS publishing. Same-persona UI retest passed for both goal-bearing and goal-less real runs, including strict fresh-version evidence in `looprun-c0e322b615e43c12`.
- **BUG-20260713-custom-loop-delete-missing:** The same worker connected the existing delete mutation to a workspace-only typed-name confirmation with success-only cache/navigation updates. Two browser replays preserved Cancel/wrong-name safety and restored bundled read-only v0 after deletion.
- **BUG-20260713-missing-workspace-persists / AGH-47:** GPT-5.6-SOL/high worker moved missing-root reconciliation into the authoritative resolver list path. Same-persona Web-first retest pruned the ghost, HTTP and UDS CLI lists converged, the old ID returned 404, and a second daemon restart proved persistent removal.
- **BUG-20260713-needs-attention-recovery-hidden:** GPT-5.6-SOL/high worker wired the existing run-recovery contract into Task/run detail with shared attempt-budget eligibility and complete cache invalidation. Browser retest proved the exhausted negative control and the successful attempt-2 continuation `run-be2c1d6592e2c043`.
- **BUG-20260713-task-role-session-never-starts:** GPT-5.6-SOL/high worker added correlated synthetic first-turn dispatch plus coalescing and failed-start cleanup. Live Cursor/Grok session `sess-1e9a13013651c8b0` responded to the exact Task/run notification in 21 seconds; claim remains blocked only by the separate Cursor Ask-mode defect.
- **BUG-20260713-parent-task-rollup-missing / AGH-71:** The shared transactional hierarchy settlement is live-verified. A fresh parent's unbound run reached Needs Attention, child A left it nonterminal, and child B's one completion settled it exactly once. The Browser retained the same terminal state after reload.
- **BUG-20260713-task-approval-duplicates-open-run:** Approval now reuses the validated sole nonterminal run and never aliases idempotency across operation origins. Two Browser controls retained `Runs 1`, used the original run ID, and completed through real Cursor/Grok sessions with no error.
- **BUG-20260713-session-user-message-reorders-or-disappears:** The controller preserved the AI SDK user-message ID through HTTP/UDS, core, canonical event storage, and Web reconciliation; Goal commands now record exact authored ingress before dispatch; hook-transformed provider input remains auditable separately. A real two-ordinary-plus-Goal Cursor/Grok thread survived live reconciliation and cold reload with each authored input present exactly once in server chronology.
- **BUG-20260714-task-create-waits-for-worker-session:** Durable enqueue now transfers task-role activation to a daemon-owned goroutine after commit. Lifecycle admission is serialized against shutdown, the drain context cancels provisioning, and shutdown joins every activation. Browser acceptance proved immediate navigation followed by one real Cursor/Grok completion of the original run.
- **BUG-20260714-session-delete-history-fk:** `permission_log` and `token_stats` now declare session ownership with `ON DELETE CASCADE` through append-only global migration v3 (`00003_schema.sql`); immutable v2 belongs to Hermes Bridge. Session directories are renamed to tombstones before the catalog commit, restored on commit failure, and retried after a post-commit cleanup failure. The original five-permission session and a fresh real Cursor/Grok user session both deleted through the historical pre-rebase UI replay with HTTP/catalog/filesystem convergence.
- **BUG-20260714-task-named-events-stale:** The Web Task stream now registers every named event persisted by transactional Task hooks. Browser acceptance kept one detail route mounted while UDS pause/resume rendered the exact reason and inverse action in 380 ms/322 ms without reload; the focused root-Turbo hook suite passed 16/16.
- **BUG-20260713-missing-workspace-persists / final ownership residual:** Workspace unregister now stages every stopped session directory through the session manager before deleting the workspace/catalog rows. The final UI replay removed a live-registered folder with one stopped real Cursor session; the rail recovered to `agh3`, UDS retained only healthy workspaces, and the session directory was removed.

## Paper Cuts

| Persona | Where (journey/step) | Felt | Sharpness | Outcome |
|---|---|---|---|---|

## Runtime Errors Observed

- `looprun-2cf0340ae8091bbe`: `load_tasks` failed twice with `call action tool \"ext__dev_cycle__import_tasks\": tool \"ext__dev_cycle__import_tasks\" backend failed`; the Web/API projection retained only `loop_action_failed` (BUG-20260713-loop-failure-hidden).
- `looprun-b165c15b174e3d40`: the same deterministic missing-taskset failure now persists and renders a typed `action_failure` with bounded cause and recovery guidance; BUG-20260713-loop-failure-hidden verified.
- `looprun-acb65149c8fc91a5`: no runtime error; it proved the daemon already used saved cap 3/full-body. The formerly stale pre-run projection is now fixed and verified through the same-persona UI replay (BUG-20260713-loop-preview-ignores-config).
- `POST /api/workspaces/:workspace_id/loops` at 2026-07-13T05:11:03Z returned 500 when `Fork & edit` was clicked; no fork was created (BUG-20260713-loop-fork-internal-error).
- The first-fix replay successfully forked extension-backed `reviews-watch`, then its first valid Publish failed with `expected_version is required` while the builder displayed `Published v0` (same bug, residual CAS branch).
- Automation Job `job-6a0a00830d60c1c0` delegated `run-f4489762ac431856` to Loop run `looprun-aeb24d4f17cf1feb`; UI previews/details omitted that target and correlation (BUG-20260713-loop-automation-shown-as-agent).
- Sessions `sess-d5879464f13e2350` and `sess-8cdde6e564e0ac5c` failed only after long model negotiation because visible/custom Cursor values did not preserve the canonical descriptor (BUG-20260713-cursor-model-startup-contract).
- Live Cursor session `sess-b1c980b86709053d` completed real turns but remained in Ask mode, retained a generic title, and disappeared from persistent cross-workspace indicators (BUG-20260713-cursor-agent-mode-unavailable; BUG-20260713-background-session-indicator-title).
- Final-replay onboarding session `sess-cdd8a43c9902d4be` was persisted as `type=user`; after one real user session, both the public catalog and inactive-workspace badge reported two user sessions (BUG-20260713-background-session-indicator-title re-found).
- Plain session-origin Goal start returned only `goal_judge_unavailable` and created no visible Goal state (BUG-20260713-goal-judge-unavailable).
- Task-role session `sess-fbc0f0f9edf012ea` originally stopped at the prompt overlay and expired by TTL; fixed session `sess-1e9a13013651c8b0` received and answered the correlated synthetic turn, isolating the remaining Ask-mode claim blocker (BUG-20260713-task-role-session-never-starts verified; AGH-71 pending).
- That fixed session later accumulated 14 responses for the same unclaimed run without user input or explicit recovery, exposing BUG-20260713-task-role-dispatch-repeats.
- The original needs-attention run now has truthful exhausted/recoverable UI states, and one Web recovery created exactly one continuation (BUG-20260713-needs-attention-recovery-hidden verified).
- Switching Create task to `Break into steps` originally erased the entered title and description; authored fields now survive template and mode transitions in the browser retest (BUG-20260713-task-template-clears-draft verified).
- Workspace-scoped Create Trigger originally projected `loop_target.workspace_id: ""` despite an outer workspace selection and then exposed a backend normalization residual. The final UI replay created and read back the fully typed filtered Loop Trigger; its matching system-session event dispatched exactly once, correlated history to the delegated Loop, and exact-name deletion removed the Trigger (BUG-20260713-workspace-trigger-loop-submit-inert verified).
- The fixed Trigger later fired exactly once from the real system-session stop, but its delegated `reviews-watch` coordinator logged a deterministic watch-poll error while the run stayed Running at generation 0 for more than two minutes with no cause (BUG-20260713-loop-watch-poll-error-stuck).
- Replaying the same binding after the coordinator repair produced only `looprun-4bc3d180d2edd5ba`, which terminalized automatically at generation zero with the bounded watch-source cause/recovery and no operator Stop; the Trigger was then deleted by exact-name confirmation (BUG-20260713-loop-watch-poll-error-stuck verified).
- Goal Run `looprun-5a1acf5934fef596` completed three real turns and settled Blocked only after two unrestricted command-judge sessions ignored the verdict JSON contract. Both temporary system sessions remained ACTIVE (BUG-20260713-goal-judge-unconstrained-leaks-session).
- Clearing active Goal `looprun-1667f72b7cdb7128` revoked the in-flight turn with durable `goal_control_revoked_in_flight`, but its Run-level alert degraded to a generic action failure (BUG-20260713-goal-clear-generic-failure).
- Workspace `ws_73db983811b21119` originally returned HTTP 410 while remaining in the catalogs; the fixed resolver pruned it on the first Web list and persisted that removal across a second daemon restart (BUG-20260713-missing-workspace-persists / AGH-47 verified).
- Session `sess-3fb644eedaea5ab9` duplicated/reordered ordinary user rows and lost its `/goal` input after reload. Fresh session `sess-59296138935045ea` preserved two ordinary inputs plus the Goal command exactly once after live reconciliation and permalink reload (BUG-20260713-session-user-message-reorders-or-disappears verified).
- Human-in-the-loop approval originally attempted a second run after Task creation had already enqueued one, then an intermediate correction tried to alias idempotency across different origins. Tasks `task-e316e5733fb4feb0` and `task-d702248d032de117` now retained one original run through approval, real claim, completion, and reload (BUG-20260713-task-approval-duplicates-open-run verified).
- Completed `run-df8c1dd9a1b8b5f8` originally rendered `task_run_orphan` solely because its former session was terminal. After the status-aware correction, Task and run detail retain that stopped-session history but render zero diagnostics and no release command (BUG-20260714-terminal-task-run-reported-orphan verified).
- Pre-fix Task `task-1bfc42bfafb6659a` persisted immediately, but `POST /api/tasks/:id/runs` held its 201 response for 19.822 seconds while task-role provisioning consumed 15.529 seconds in sandbox synchronization and 4.108 seconds in Cursor startup. Post-fix Task/run create returned in 2 ms/3 ms and provisioning completed after the response (BUG-20260714-task-create-waits-for-worker-session verified).
- In the historical pre-rebase lab, `DELETE /api/workspaces/ws_30f28bfa2ef7ac98/sessions/sess-12b2c865a27ecc72` returned 500 before the then-current Automation migration v2 because session-owned permission/token rows were restrictive foreign keys. The same UI deletion passed after migration; fresh read returned 404 and dependent counts were zero. The merged stream assigns that cascade to v3.
- The Task EventSource omitted persisted `task.paused`, `task.resumed`, block, auto-enqueue, hallucination, and wake event names. Final replay observed pause sequence 385 and resume sequence 386 update the mounted detail without reload after the listener inventory correction (BUG-20260714-task-named-events-stale verified).

## Human Verifications Needed

None identified before execution.

## Decisions for a Human

None identified before execution.

## Learnings

- Existing Loop and Goal tracker rows are intentionally reset/blocked from earlier provider-limited passes; this run must settle them with live Cursor/Grok 4.5 evidence rather than inherit prior claims.

## AGH Impact Audit

- **Native tools:** `agh__session_list` retains its ID, toolset, and risk semantics and now exposes the exact public `type` filter; its descriptor/schema digest and generated native-tool catalog were regenerated and canonical CLI/native-tool tests passed. Goal and Task tool IDs remain unchanged, while their existing structured settlement/recovery contracts now receive the corrected runtime behavior. Loop action failures preserve a bounded typed operator-safe envelope through extension RPC, daemon/store, and Web instead of degrading to a generic code. Checked built-in descriptors, CLI, HTTP/UDS fallbacks, generated OpenAPI/TypeScript, and native catalog fixtures.
- **Extensibility and hooks:** Loop start bindings now keep typed workspace Loop targets and validated inputs across Job/Trigger registration, clone, dispatch, and correlated history. The dev-cycle extension preserves safe action-failure metadata. Session catalog lifecycle wakes are exposed through HTTP and UDS for browser integrations; reconnect remains snapshot reconciliation rather than incremental client authority. Task completion now settles parent hierarchy and dependent Loop wakes atomically, generation-zero coordinator failures settle the correlated Loop exactly once, and post-commit Task activation remains behind the same observer contract while moving provider latency out of the request lifetime. No extension registry, hook event ID, capability/bundle/resource, MCP sidecar, bridge SDK, or `config.toml` key/default changed; those surfaces were explicitly checked.
- **Workspace data isolation:** Loops, Goals, Tasks, Jobs, Triggers, sessions, run history, and catalog wakes remain workspace-scoped. `workspace_id` propagation was exercised through UI, HTTP, UDS/CLI, core/store, query keys, SSE invalidation, and event correlation in two simultaneous workspaces. Session detail/transcript cache keys remain `(workspace_id, session_id)` while globally unique `byId` resolves identity only; exact `type=user` inactive-workspace counts exclude system/task-role/judge sessions. Missing-root reconciliation removes only canonically absent registrations through the authoritative resolver and persists pruning without deleting transiently unavailable workspaces. Global v3 changes only session-owned foreign keys: deletion cascades by exact globally unique session ID, and the canonical regression proves a foreign session's permission/token rows remain untouched. Immutable v2 remains Hermes Bridge's migration.
- **Official AGH skill:** `skills/agh/references/native-tools.md`, `runtime-operations.md`, and `tasks-and-orchestration.md` document exact session-type filtering, automatic titles/catalog reconciliation, durable session removal, Task recovery, and parent settlement. Loop/automation UI-only affordances did not introduce new public tool IDs or CLI paths.

## Web/Docs Impact

- **Web:** User-visible changes span Loop configuration/run/editor/failure/delete surfaces; target-aware Job/Trigger create/edit/detail/history/delete; Task creation, ownership clearing, recovery, and parent settlement projections; Goal composer/error/control projection; Cursor session creation; automatic titles; exact inactive-workspace session activity; and cross-workspace route reconciliation. Canonical hooks, route integration suites, Storybook states, root Turbo typecheck/test lanes, and deterministic screenshot captures cover the touched systems.
- **Docs:** Runtime session lifecycle/catalog documentation, CLI session-list reference, Task run/lease documentation, the official AGH skill, and the living `docs/qa` journeys/charters/scenarios/bugs/report are co-shipped. No site-only marketing claim or configuration lifecycle changed.

## Environment Pause

- The in-app Browser remained on Browser Use's native connection-refused `data:` interstitial for three consecutive Goal continuations. Browser security policy prohibits reading, controlling, or navigating from that page, so no substitute surface was used and no pending real-user verdict was weakened.
- The pre-final architecture audit found three aggregate-diff hard-cap violations: `internal/task/lease_manager.go` was 871 lines after growing by 48 net lines, `internal/api/core/interfaces.go` was 527 lines after growing by eight, and `internal/store/globaldb/global_db_task_coordinator.go` was 507 lines after growing by 24. Herdr briefly became unavailable, then recovered before the Goal was marked blocked. One serialized GPT-5.6-SOL/high worker moved only the added responsibilities into focused 70-line, 15-line, and 31-line files. The originals are now 812 lines versus 829 in HEAD, 519 versus 519, and 483 versus 483. Controller verification passed 467 Task, 1,124 API core, and 668 GlobalDB race tests; scoped GolangCI reported zero issues; `gofmt -d`, `git diff --check`, and the complete over-cap-growth scan are clean. No peer review ran.
- Because this became a terminal blocked path, L-029 teardown ran against the canonical bootstrap manifest. It killed registered Web PID `25259` and daemon PID `31100`; `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/teardown.json` records `clean: true` with zero survivors.
- A fresh isolated continuation lab was bootstrapped from the current source at `http://127.0.0.1:52792`, with Web at `http://localhost:3000`. After the final mechanical split, the source rebuilt and the daemon restarted as PID `37428`; Web PID `24887` remained healthy and the proxy reported zero Jobs, zero Triggers, and zero queued Tasks. The in-app Browser still could not be restored without user action. Because this was the third consecutive blocked continuation, L-029 teardown stopped both registered processes; the continuation lab's `teardown.json` records `clean: true` and zero survivors. Existing QA tracker, report, and prior evidence remain durable.

## Final Status

- **Behavioral QA verdict:** REVALIDATION REQUIRED — the historical Automation-only evidence remains useful, but it cannot promote scenarios changed by the Automation + Hermes rebase.
- **Exit gate (full automated suite):** PENDING for the combined worktree. The 3,410-Web/13,828-Go `make verify` log belongs to the pre-rebase source state and is historical evidence only.
- **Historical make verify evidence:** `/Users/pedronauck/dev/qa-labs/agh-automation-features-post-onboarding-fix-20260713-20260713-203513-816377-lab/qa-artifacts/qa/final-make-verify-post-remediation.log`.
- **Issues by user impact:** 37 verified this cycle — 20 Blocks-Completion, 1 Data-Loss, 11 Trust-Damage, and 5 Friction
- **Coverage:** not settled for the rebased source. Canonical open controls include session-start latency, Cursor artifact creation, disabled/unrelated-workspace Loop wake negatives, and message reconciliation/reload.
- **Peer review:** rebase review round 1 is `FIX_BEFORE_SHIP`; its selected blockers are being remediated and require a fresh review.
- **Historical teardown:** PASS — `/Users/pedronauck/dev/qa-labs/agh-automation-features-post-onboarding-fix-20260713-20260713-203513-816377-lab/qa-artifacts/qa/teardown.json` records `clean: true` and zero survivors for the historical lab.
- **Verdict:** PENDING — promote only canonical rows backed by fresh final-worktree evidence and the combined full gate.

```yaml qa-bootstrap
manifest_path: /Users/pedronauck/dev/qa-labs/agh-automation-features-post-onboarding-fix-20260713-20260713-203513-816377-lab/qa-artifacts/qa/bootstrap-manifest.json
lab_root: /Users/pedronauck/dev/qa-labs/agh-automation-features-post-onboarding-fix-20260713-20260713-203513-816377-lab
runtime_home: /var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/aghqa-b971946d505c/runtime
base_url: http://127.0.0.1:53199
reused_lab: false
```
