// Suite: Runtime apply notice
// Invariants: The composer claims "now" only for an applied change on a provider that honors
// live changes mid-turn; deferred or refused changes name the next message.
// Boundary IN: The daemon's session runtime answer and the provider live-apply capability.
// Boundary OUT: Rendering and mutation wiring, owned by the session prompt runtime hook suite.

import { describe, expect, it } from "vitest";

import { runtimeApplyNotice } from "../runtime-apply-notice";

const selected = {
  provider: "claude",
  model: "claude-opus-5-5",
  reasoning_effort: "medium" as const,
};

describe("runtimeApplyNotice", () => {
  it("Should name the next message when the daemon deferred the change to a new process", () => {
    const notice = runtimeApplyNotice(
      {
        badge: "running",
        runtime: {
          status: "ready",
          selection_revision: 1,
          selected: { provider: "codex", model: "gpt-6-astra" },
          effective: { provider: "claude", model: "claude-opus-5-5", reasoning_effort: "xhigh" },
        },
      },
      "next_step"
    );
    expect(notice).toEqual({ text: "Applies from the next message.", tone: "neutral" });
  });

  it("Should surface a refused live change as a retry on the next message", () => {
    const notice = runtimeApplyNotice(
      {
        badge: "running",
        runtime: {
          status: "ready",
          selection_revision: 1,
          failure: 'acp: "max" is unavailable',
          selected: { ...selected, reasoning_effort: "max" },
          effective: { ...selected, reasoning_effort: "xhigh" },
        },
      },
      "next_step"
    );
    expect(notice).toEqual({
      text: "Couldn't apply this now. It retries with the next message.",
      tone: "danger",
    });
  });

  it("Should stay quiet for a change applied while the session is idle", () => {
    const notice = runtimeApplyNotice(
      {
        badge: "idle",
        runtime: { status: "ready", selection_revision: 1, selected, effective: selected },
      },
      "next_turn"
    );
    expect(notice).toBeNull();
  });
});
