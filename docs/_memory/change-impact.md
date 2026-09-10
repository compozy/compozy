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

## Issue 602 — Embedded private endpoint verification

- **Native tools:** Gateway status/audit, CLI and HTTP/UDS keep their IDs, DTOs and authorization. Existing provider causes gain safe failure classification; only a core-verified route becomes advertised.
- **Extensibility and hooks:** The connectivity endpoint wire contract adds optional `verification_address` for a bounded loopback TCP relay. Go/TypeScript SDKs co-ship through codegen. Omission retains existing behavior; public proof rejects the field. No config, hook or manifest permission changes.
- **Workspace data isolation:** Provider transports remain global gateway runtime resources, owned per tier and torn down before the node closes. No persisted schema or data migration. Public addresses exclude the relay; endpoint identity comparisons include it.
- **Official Compozy skill:** Runtime Gateway guidance explains embedded private proof and safe diagnostic classes.
- **Web/Docs:** Existing Gateway views consume provider causes without new UI state. Tailscale and extension-authoring guides describe transport, proof and recovery. `RT-connectivity-provider-route` owns the affected scenario. TLS, nonce, redirects, tier binding and public outbound policy retain their owning gateway suites; provider tests cover relay lifecycle.
