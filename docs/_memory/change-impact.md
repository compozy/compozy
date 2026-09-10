# Compozy Change Impact

Use for changed runtime behavior, public contracts, config, or feature documentation. Record this analysis once in the owning spec/task/PR and link it from implementation slices; update only the affected entries. A small change needs only concise findings. Editorial changes with no runtime contract may state `not applicable — editorial only`.

- **Native tools:** changed `compozy__*` IDs, toolsets, descriptors, schemas/digests, risk flags, capability gates, diagnostics, and CLI/API fallbacks.
- **Extensibility and hooks:** extensions, hooks, skills/capabilities, resources, registries, bridge SDKs, MCP sidecars, and config lifecycle. Config changes co-ship defaults, loader/overlay behavior, validation, docs, and compatibility under SD-013.
- **Workspace data isolation:** classify changed data as global/workspace/session/agent-scoped. Follow `workspace_id` through affected CLI/HTTP/UDS/core/store/web/cache/SSE/event paths and verify the owning boundary prevents cross-workspace leakage.
- **Official Compozy skill:** update `skills/compozy/` when public behavior, tools, CLI paths, hooks, capabilities, resources, or memory/network/task semantics change.
- **Web/Docs impact:** name affected `web/` routes/components/hooks and `packages/site` docs, plus their verification owner. Backend changes carry this analysis with the feature.

For an unaffected entry, name the checked surfaces and why the change cannot affect them. Do not create separate artifacts or re-audit unchanged surfaces for every checkpoint. Breaking changes also name delete targets and the user-state/public/internal compatibility regime; apply SD-013 before deletion.

## Issue 595 — Goal lifecycle and attention

Owning delivery: [PR #601](https://github.com/compozy/compozy/pull/601). Detailed regression and runtime evidence: [focused QA report](../qa/reports/2026-09-10-issue-595-goal-lifecycle.md).

- **Native tools:** Existing Goal control/read, session stop/removal, and Loop cancellation surfaces keep their IDs, schemas, and authorization. Run terminal state and quarantine govern Goal status. Stopping a session cancels its session-origin Goals; failed cancellation remains retryable in the existing stop settlement receipt.
- **Extensibility and hooks:** No hook, SDK, configuration, or registry shape changes. Cancellation uses the existing aggregate and durable cleanup outbox. Catalog Loop lineage and window dismissal retain their semantics.
- **Workspace data isolation:** Binding adoption validates workspace, task, control, phase, session, handle, and epoch. Session Goal cancellation reads immutable profile/workspace scope. Existing receipts preserve session/runtime identity across restart. No schema migration or historical record deletion.
- **Official Compozy skill:** `skills/compozy/references/loops.md` documents cancellation, orphan recovery, context ownership, and bounded supervision freshness.
- **Web/Docs:** Session badges, Goal strip, Tasks, and Loop detail consume corrected backend projections; no client-side badge clearing. The Goals guide and GL/LP/RT QA slices accompany the change. Canonical lifecycle suites and focused CLI/API/Web runtime walks own validation; remaining delivery gates run in CI.

## Merged PRs 596, 597, 599, 600, and 601

The [main-branch remediation report](../qa/reports/2026-09-10-merged-pr-ci-review-remediation.md)
owns the follow-up audit: captured terminal creation scope, stale-navigation
rejection, explicit title-disclosure action names, public session imports, and
review/CI disposition. Existing native, persisted, configuration, and official
skill contracts remain unchanged.


## Issue 606 — Durable notification acknowledgement

- **Native tools:** no native IDs, tool descriptors or CLI verbs change. Additive operator HTTP/UDS
  `GET /notifications/attention` and `POST /notifications/attention/acknowledge` routes expose the
  notification read/acknowledgement contract. Existing task decisions, task triage, presence and
  session attention-summary surfaces keep their meanings.
- **Extensibility and hooks:** no hook, bridge preset, delivery cursor, SDK or configuration changes.
  Acknowledgement does not emit source completion or approval events.
- **Workspace data isolation:** exact occurrence receipts belong to a profile and actor. Home
  snapshots bind workspace/global scope plus profile lens; bell snapshots contain all workspaces and
  source profiles, with receipts belonging to the selected destination profile. Bulk writes only
  accept IDs captured in that snapshot, commit atomically and are idempotent. New occurrences race
  safely outside old snapshots. Task inbox triage filters the task candidates; raw escalations keep
  their existing lifecycle semantics.
- **Compatibility:** migration 109 adds receipt and snapshot tables without rewriting source data.
  Snapshots expire after 24 hours; receipts persist. Old databases migrate through the canonical
  schema generator. New overview fields are additive; the Home attention count now means unread
  occurrences. Existing runtime/session/task status and decision APIs are unchanged.
- **Official Compozy skill:** the native-tools reference documents the operator-only API, exact
  snapshot acknowledgement and separation from source actions.
- **Web/Docs:** bell, title count and Home use server receipts, show mutation errors, and invalidate
  both corresponding caches after settlement. Individual controls do not activate their row.
  Notification preset docs distinguish inbox acknowledgement from delivery configuration. Updated
  bell/Home/title QA scenarios record the new contract. SQLite, overview and existing API/component/
  browser suites own coverage; broad local gates and rendered labs are deferred to CI by explicit
  user instruction for this delivery.
