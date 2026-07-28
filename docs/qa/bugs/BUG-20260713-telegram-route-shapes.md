# BUG-20260713-telegram-route-shapes: One Telegram bridge cannot accept all route shapes with exact isolation

- **Status:** open
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Tessa and Maya while connecting Telegram through the supported setup flow
- **Journey Step:** J-connect-bridge-provider, connect one supported provider with correct secrets and routing
- **Scenarios:** NB-bridge-provider-setup; NB-025; NB-029
- **Found:** 2026-07-13 · **Report:** docs/qa/reports/2026-07-12-hermes-bridge.md
- **Origin:** n/a
- **Scope updated:** 2026-07-13 Phase D remediation fixed one-shape setup expressibility; this record now owns only the remaining single-instance alternative-shape contract.

## Summary

Guided and strict-JSON Telegram setup can create a bridge for exactly one explicit routing shape: private chat (`peer`), ordinary group (`group`), or forum topic (`group + thread`). The guided default remains forum-topic routing.

The remaining gap is structural. `RoutingPolicy` is one conjunction of required dimensions, so one Telegram bridge cannot select the route identity from the actual inbound shape. Operators who need private chats, ordinary groups, and isolated forum topics must provision separate bridge instances. A group-only instance can accept a forum event only by omitting the thread from its routing identity, which collapses topics in the same group.

## Remaining reproduction

- **Charter:** CH-guided-setup-credentials / CH-structured-telegram-setup · **Tour:** Task Tour and Integration Tour
- **Environment:** isolated local daemon, current-source CLI, rebuilt Telegram extension, deterministic fake Telegram Bot API

1. Create and enable a Telegram bridge with the `private` shape.
2. Confirm a `peer_id` target works.
3. Send to a group-only target and observe rejection because that instance requires `peer_id`.
4. Create a `group` instance and send from two forum topics in the same group. Both events use the group-only route identity, so topic isolation is not preserved.
5. Create a `forum` instance. A forum target works, while an ordinary group without `thread_id` and a private chat without `group_id` or `thread_id` are rejected.
6. Observe that no single current `routing_policy` accepts all three shapes while preserving each shape's actual identity.

**Expected:** One Telegram instance can represent alternative route shapes and build the routing key from the dimensions present in the actual event: peer for private chats, group for ordinary groups, and group plus thread for forum topics.

**Actual:** Setup can select any one shape, but the stored policy remains a single conjunction. Covering all shapes with exact isolation requires multiple bridge instances.

## Evidence

- One-shape setup owner: `internal/cli/bridge_setup_platforms.go` maps `private`, `group`, and `forum` to three closed policies.
- Strict-JSON and guided owner: `internal/cli/bridge_setup_config.go` accepts typed `routing_policy` and wizard `routing_shape`.
- Focused regression: `internal/cli/bridge_setup_test.go::TestBridgeSetupTelegramRoutingShapes`.
- Remaining core owner: `internal/bridges/dimensions.go::validateRoutingDimensions` requires every enabled dimension on every route.
- Routing-key owner: `internal/bridges/routing.go::BuildRoutingKey` includes only the dimensions selected by the single stored policy.
- Public truth: `packages/site/content/runtime/core/bridges/setup-telegram.mdx` states that one instance selects one shape.
- Historical QA artifacts remain attached as evidence of the original fixed-policy behavior.

## Fix

- **Partial remediation:** Phase D setup now accepts one explicit private, ordinary-group, or forum shape through guided and strict-JSON paths.
- **Remaining root cause:** `RoutingPolicy` cannot express alternative shape-specific routing keys for one instance.
- **Remaining fix:** deferred pending a structural routing-contract TechSpec. The design must preserve direct-chat, ordinary-group, and forum-topic isolation without fabricating missing dimensions or silently collapsing topics.
- **Future regression:** prove one Telegram instance accepts all three actual event shapes and generates distinct canonical route identities for private chats, ordinary groups, and individual forum topics.

## Verification

- **Retested:** not through persona QA.
- **Focused implementation evidence:** `TestBridgeSetupTelegramRoutingShapes` proves that each single shape can be selected and persisted.
- **Result:** the original fixed `group + thread` setup defect is remediated; BUG-20260713-telegram-route-shapes remains open only for the broader single-instance alternative-shape contract.
