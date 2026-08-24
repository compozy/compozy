// Suite: attention notifier activation target
// Invariant: an agent notification keeps its session and workspace identity
// even before the attention catalog resolves the sending session.
// Owning layer: OS notifier. No prior suite owned this delivery-to-jump mapping.
import { describe, expect, it } from "vitest";

import { attentionJumpTargetFor, planAttentionToasts } from "../use-attention-notifier";

function needsYouDelivery(id: string) {
  return {
    kind: "needs-you" as const,
    key: `needs-you:${id}`,
    target: {
      id,
      title: id,
      agentName: "claude",
      workspaceId: "ws-alpha",
      workspaceLabel: "alpha",
      badge: "waiting-for-input" as const,
      reason: "Question pending",
    },
  };
}

describe("attentionJumpTargetFor", () => {
  it("Should preserve an unresolved agent notification's session identity", () => {
    expect(
      attentionJumpTargetFor({
        kind: "agent",
        key: "agent:ntf-1",
        notification: {
          profile_id: "00000000000000000000000000",
          profile_name: "default",
          notification_id: "ntf-1",
          session_id: "sess-alpha",
          workspace_id: "ws-alpha",
          title: "Review ready",
          body: "",
          at: "2026-07-13T12:05:00Z",
        },
        target: null,
      })
    ).toEqual({ sessionId: "sess-alpha", workspaceId: "ws-alpha" });
  });

  it("Should keep the four newest needs-you toasts and fold older ones into a ledge", () => {
    const plan = planAttentionToasts(
      ["one", "two", "three", "four", "five", "six"].map(needsYouDelivery)
    );

    expect(plan.deliveries.map(delivery => delivery.key)).toEqual([
      "needs-you:three",
      "needs-you:four",
      "needs-you:five",
      "needs-you:six",
    ]);
    expect(plan.overflowNeedsYou).toBe(2);
  });
});
