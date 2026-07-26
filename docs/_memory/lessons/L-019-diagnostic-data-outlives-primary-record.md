# L-019 — Diagnostic data must outlive its primary record when audit/replay matters

**Class:** Architecture / Persistence
**Date discovered:** 2026-05-05
**Evidence sources:** Bridge subscription store tests and workflow memory.

## Context

The orchestration-improvements workstream introduced two related but separately-owned concepts:

- A bridge task subscription describes a delivery _target_: which bridge instance, which task,
  which delivery mode and routing fields. Stored in `bridge_task_subscriptions`. Primary key is
  the subscription id.
- A notification cursor describes confirmed _delivery progress_: last sequence delivered, last
  delivery id, last delivered timestamp, last error, updated timestamp. Stored in
  `notification_cursors`. Primary key is `(consumer_id, stream_name, subject_id)`, with the bridge
  subscription using `consumer_id = "bridge_task_subscription:<subscription_id>"`.

A naive design would collapse both into one row keyed by subscription id. That model breaks the
moment an operator deletes a stale subscription: the only audit/replay surface — last error,
last delivery id, last sequence — disappears with it, and any same-id recreation either restarts
from sequence zero (re-deliver every terminal event) or skips ahead with no proof of where
delivery actually stopped.

The implemented model is the opposite of that naive design. `DeleteBridgeTaskSubscription`
(`internal/store/globaldb/global_db_bridge.go:985-1014`) deletes only the
`bridge_task_subscriptions` row. The `notification_cursors` row keeps living. The dedicated test
`Should remove active subscriptions while preserving stale cursor diagnostics`
(`internal/store/globaldb/global_db_bridge_task_subscription_test.go:147-210`) asserts that
the cursor row survives a delete and that a same-id recreation resumes from the preserved cursor.

## Root cause

Audit, replay, and recovery semantics decay silently when their data lives inside the row whose
deletion is a normal operator/agent action. Operators delete subscriptions for reasons that have
nothing to do with delivery progress (typo, retired bridge instance, scope change). If audit data
piggybacks on that row, deletion is no longer a routine action — it becomes a destructive one
that erases the only proof of what was delivered, what failed, and where to resume.

Splitting the lifetimes preserves the operator's intuition that delete-then-recreate is safe,
while still giving operators and agents a stable diagnostic surface keyed by the cursor identity
they can recreate.

## Rule

> When a primary record's deletion would lose audit, replay, or recovery context that operators or
> agents still need, model the diagnostic data as a separate row keyed by something stable (a
> cursor identity, a delivery id, a workspace path, an idempotency key) with its own lifecycle.
> Do not hide audit/replay state inside the same row whose deletion is a routine operator action.

## Operationalization

- Identify the audit/replay questions the operator or agent needs to answer after the primary row
  is gone. If "what was the last delivered sequence?", "which delivery id last advanced?",
  "what was the last error?", or "where would a recreated record resume?" still matter,
  the diagnostic data must outlive the primary.
- Pick a key for the diagnostic that is stable across primary recreation. Cursor identity
  `(consumer_id, stream_name, subject_id)` survives a `bridge_task_subscriptions` delete by
  construction.
  - Make the primary's delete path remove only the primary. Document that diagnostic rows survive
    and can be inspected through the cursor read model.
- Surface the cursor zero-state (no delivery yet) and the post-delete stale-cursor state through
  the same diagnostic projection. UI must handle both branches truthfully (see
  `web/src/systems/tasks/components/tasks-bridge-notifications-card.tsx`).
- Write a test that deletes the primary, fetches the diagnostic by its stable key, and asserts
  the diagnostic survived with its last-known fields intact.

## Anti-pattern

- A `DELETE CASCADE` from primary to cursor/diagnostic table that erases delivery history on
  routine subscription teardown.
- Rebuilding stale diagnostics into a write-time snapshot column that is silently overwritten by
  every successful delivery.
- Telling operators "recreate the subscription to retry" while quietly resetting the sequence to
  zero and re-delivering everything in the durable event log.

## Source

- `internal/store/globaldb/global_db_bridge.go:985-1014` (`DeleteBridgeTaskSubscription`)
- `internal/store/globaldb/global_db_bridge_task_subscription_test.go:147-210` (Should remove
  active subscriptions while preserving stale cursor diagnostics)
- `internal/notifications/` (cursor primitive: identity, monotonic advance, idempotent replay,
  reset semantics)
- `web/src/systems/tasks/components/tasks-bridge-notifications-card.tsx` (zero-state /
  populated-cursor branches)
