# QA Run Report — 2026-09-10 — Issue 595 Goal lifecycle

- **Scope:** Initial Goal context ownership, supervision freshness, and explicit session stop/removal.
- **Cadence:** Focused runtime and public API/CLI/Web journeys with real Codex / gpt-6-astra / high.
- **Build:** `fix/issue-595-goal-lifecycle`, based on `470c7c8f`.
- **Status:** in-progress; the explicit clarification-answer step is blocked by the execution approval boundary.
- **Persona:** Ada, a developer using self-directed Goals for a small billing utility; Bruno, an operator stopping unfinished work.

## Session matrix

| Scenario slice | Result | Evidence |
| --- | --- | --- |
| Self-activation during an ordinary prompt | Pass | Two real Codex sessions activated their own Goals; the corrected flow had known context, current Loop work, and no stale attention while ordinary work continued. |
| Known-context public reads | Fail, repaired, re-walk passed | The first live walk exposed incompatible event-query cursors. A canonical real-store regression failed before the fix and passed after it; the persisted Goal became readable after the isolated daemon upgrade. |
| Goal completion | Pass | Billing utility completed in two turns; a later session reused it to add exact JSON totals, passed twelve real CLI tests, and received an approved one-turn Goal verdict. |
| Genuine attention | Pass | A structured refund-scope question produced `waiting-for-input` while Loop supervision remained current. Web showed one waiting item. |
| Answer and continue | Blocked | Execution approval review rejected the operator's synthetic answer. No answer was injected through another surface. The agent did not choose a refund scope or implement it; it paused its Goal after the question expired. |
| Explicit stop | Pass | A real running Goal reached a stopped session and canceled Run before manual cancellation. Session supervision emptied and all associated task records were canceled. Repeated Run cancellation succeeded. |
| Session removal | Pass | Removing a separate real running Goal canceled the Run and its Tasks. The session returned 404; Run history remained readable, and cancellation by Run still succeeded with original daemon cancellation provenance. |
| Window-only dismissal | Pass | Closing the refund session window left its paused Goal live. No session stop or Goal cancellation occurred. |
| Browser Goal projection | Pass | The expanded Goal strip showed `Done`, `settled`, known context, and the approved verdict with zero blocking issues. |
| Final restart/reconnect | Pass | After restarting the isolated daemon with the final binary, stopped/removed Runs remained canceled, the approved Run remained done, and the paused Run remained paused. The browser reloaded the retained Goal and transcript. |

Raw lab evidence is retained locally. This report deliberately omits private runtime identities and paths.

## Root causes and regression evidence

The real SQLite/managed binding regression rejected the initial context observation with `BindingEpoch=0`. The active binding already existed; checkpoint adoption previously occurred only during prompt preparation, after the initial context observation. The fixed store adopts the exact active binding under the checkpoint's control/phase/task fence before observing context. Re-adoption preserves the usage sequence floor; stale owners, wrong bindings, and cross-workspace requests remain rejected.

The first live walk exposed a second defect: a known-context read combined `AfterSequence` and `BeforeSequence`, which the session event store rejects. The read now uses one forward cursor and one result, while retaining the exact-sequence check. Missing or non-usage events remain unknown.

Loop supervision previously inferred freshness only from completed coordinator tasks. Initial in-flight action leases and admitted wait deadlines now contribute bounded evidence. Reads do not extend deadlines. Quarantine, approval, and intervention carry explicit attention even when agent progress is fresh; real expired evidence still becomes stale. Goal projections follow terminal Run state and quarantine rather than a stale active checkpoint.

Explicit operator stop/removal now uses canonical Loop cancellation for session-origin Goals. A cancellation failure retains the existing durable stop receipt and pending settlement; retry and daemon boot recovery repeat cancellation before releasing that receipt. Its existing durable outbox handles session cleanup. Catalog Loop origin lineage remains informational, and daemon shutdown or window-only dismissal does not acquire cancellation semantics. The existing adapter regression suite exposed an early missing-session return that could suppress stop failures; production was corrected without weakening the assertions.

## Verification

- Canonical Goal executor tests and context/compaction telemetry tests passed.
- Real SQLite checkpoint ownership, Goal projection, Loop work, quarantine/parking, and requeue suites passed.
- Canonical session work-signal and Goal command tests passed.
- Existing public daemon subprocess E2E passed controls, disconnect/restart, and new stop/removal followed by repeated Run cancellation.
- `bunx turbo run typecheck build --filter=@compozy/site` passed all seven tasks.
- React Doctor changed-source scan found no changed React source files. Its PR check and all bot findings remain part of delivery.
- Local gate attempts passed codegen and lint and exposed the adapter regression described above. After correction, the canonical adapter suite and full daemon race suite passed. At the author’s request, remaining delivery gates run in CI; the local global database race suite was interrupted and is not claimed as passed. Current-head CI results are recorded in the PR.

## Cross-surface impact

Owning audit: [Issue 595 change-impact record](../../_memory/change-impact.md#issue-595--goal-lifecycle-and-attention).

- **Native tools:** Existing `compozy__goal_control`, Goal reads, session stop, and `compozy__loop_cancel` retain their schemas and authorization. Goal status follows terminal Run truth and exposes quarantine as a blocked state.
- **Extensibility/hooks/config:** No configuration, hook, extension SDK, or tool ID change. Session-origin Goals use canonical cancellation and its durable cleanup outbox. Catalog Loop origin lineage remains informational.
- **Isolation/data:** Internal queries remain scoped by immutable profile and workspace. Checkpoint adoption validates task/control/binding/phase identity in one transaction. No schema change or destructive migration. Historical failed generations remain available.
- **Official skill:** Loop guidance documents stop/removal, window-only dismissal, context ownership, bounded freshness, and cancellation by Run for a missing session.
- **Web/Docs:** Existing session badges, Goal strip, Tasks, and Loop detail consume backend state. No client-side badge clearing. The operator guide and affected scenario slices accompany the fix.

## Limits

The explicit clarification-answer step remains unverified because of the execution approval boundary. Teardown completed with `clean: true`. The PR records final delivery-gate, current-head CI, and complete bot-review remediation status. No live-provider Linux or Electron dogfooding is claimed. This focused issue walk does not claim a full release or multi-agent Network scenario sweep.
