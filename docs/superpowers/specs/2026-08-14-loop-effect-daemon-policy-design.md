# Daemon-Owned Loop Effect Policy Design

## Status

Approved for implementation on 2026-08-14. Tracks [#403](https://github.com/compozy/compozy/issues/403).

## Problem

Loop terminal tool effects are committed to the outbox correctly, but the daemon cannot deliver native tools that require workspace authorization. On Compozy `v0.3.0-beta.16` (`c38ba0fac69aa657140fc578067d2c538b18ec10`), two independent terminal deliveries failed with:

```text
effect_tool_failed: daemon: resolve agent tool policy "loop-effect": agent not available in workspace: loop-effect
```

The relay labels its tool scope with `AgentName: "loop-effect"` and `ActorKind: "daemon"`. Policy normalization currently drops `ActorKind`, so the policy resolver mistakes the audit label for an authored agent and queries the workspace agent catalog. No such public agent should exist.

Fixing only that lookup is incomplete. Native tools with a `workspace` input also require a session for every non-operator scope. A daemon-owned outbox delivery deliberately has no caller session, so it would next fail with `native workspace access requires a session id`.

## Invariants

- `loop-effect` remains an attribution label; it does not become an authored, bundled, or public agent.
- A trusted daemon actor never acquires an authored agent's tool policy from its audit label.
- Global and workspace tool enablement, source trust, permission ceilings, and approval requirements remain enforced.
- A daemon-owned effect may target only the workspace recorded on its durable outbox entry.
- Cross-workspace native tool input remains denied for daemon actors.
- Unknown agents in agent-session scopes continue to fail closed.
- Tool IDs, schemas, descriptors, configuration, extension formats, and public catalog entries do not change.

## Design

### Preserve actor identity during policy resolution

`normalizeToolPolicyScope` preserves `ActorKind`. The native policy resolver applies authored-agent policy only when a non-empty agent name belongs to a non-daemon actor. A daemon actor still receives the global and resolved workspace policy inputs, bundled/development extension trust, and the normal approval availability setting.

This is actor-based rather than a special case for the string `loop-effect`. Public operator and hosted-MCP scopes cannot supply `ActorKind`; those scopes are constructed and bound server-side, so the daemon distinction remains a trusted internal classification.

### Authorize daemon-owned workspace inputs

The native workspace input binder recognizes a daemon scope only when it carries a non-empty trusted workspace ID. It constructs a `workspaceaccess.ActorRef` with `Kind: ActorDaemon`, the exact workspace ID, and the audit label.

The existing authorization path then supplies the fence:

- same workspace: accepted by exact workspace identity;
- different workspace: evaluated by `workspaceaccess.DefaultPolicy`, which denies daemon actors;
- missing workspace identity: rejected before tool execution.

No synthetic session is created, and no approval prompt can broaden daemon authority.

## Rejected alternatives

- **Register `loop-effect` as a builtin agent.** This would reserve a public authoring name, pollute the catalog, and invent provider/prompt semantics for an internal delivery worker.
- **Remove `AgentName` from the relay.** This avoids the first failure but loses useful tool-call attribution and still leaves the workspace-input session barrier.
- **Add a public actor-ID surface.** A separate actor identity may be a useful future model, but it is a much larger cross-surface change than this behavioral repair requires.

## Test strategy

The regression belongs in existing canonical suites:

1. `native_tools_test.go` proves a daemon scope with a synthetic label inherits workspace policy without resolving an authored agent; the same label in an agent-session scope still fails.
2. The native workspace binding suite proves daemon same-workspace access without a session, denial for a foreign workspace, and denial when daemon workspace identity is absent.
3. `loop_effect_relay_test.go` crosses the real runtime registry, policy resolver, and workspace binder while delivering a native tool effect. Existing fake-tool relay tests continue to own delivery isolation, approval, retry, and acknowledgement behavior.
4. The existing Loop integration suite proves a committed terminal tool effect is delivered through real daemon wiring.

Tests must fail for the beta.16 behavior before production changes are applied.

## Documentation and QA impact

- **Native tools:** policy interpretation changes only for trusted daemon-owned calls. No ToolID, schema, descriptor, or MCP/HTTP/UDS shape changes.
- **Extensibility and hooks:** any installed Loop may declare a permitted terminal tool effect; extension formats and hook contracts are unchanged.
- **Workspace isolation:** the existing same-workspace fence is made usable for daemon effects, while foreign workspace access remains denied.
- **Official skill and site:** current Loop documentation already promises daemon-delivered, at-least-once terminal effects. This restores that contract rather than adding a new one, so no public prose change is required.
- **QA:** update the existing `LP-terminal-outcome-notification` scenario and attach real evidence there; do not create a duplicate scenario.

## Out of scope

- Interactive approvals for background effects.
- A new public actor identity API.
- Changes to effect retry or at-least-once delivery semantics.
- A public `loop-effect` agent definition.
