// Suite: Runtime apply notice
// Invariants: The composer claims "now" only for an applied change on a provider that honors
// live changes mid-turn; deferred or refused changes name the next message.
// Boundary IN: The daemon's session runtime answer and the provider live-apply capability.
// Boundary OUT: Rendering and mutation wiring, owned by the session prompt runtime hook suite.

import { describe, expect, it } from "vitest";

import { currentRuntimeApplyNotice, runtimeApplyNotice } from "../runtime-apply-notice";

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

  it("Should treat a refused ACP option as not applied even when the other fields match", () => {
    const notice = runtimeApplyNotice(
      {
        badge: "running",
        runtime: {
          status: "ready",
          selection_revision: 2,
          failure: "acp: thinking refused",
          selected: { ...selected, acp_options: [{ id: "thinking", bool_value: true }] },
          effective: { ...selected, acp_options: [{ id: "thinking", bool_value: false }] },
        },
      },
      "next_step"
    );
    expect(notice?.tone).toBe("danger");
  });
});

describe("currentRuntimeApplyNotice", () => {
  const saved = {
    badge: "running" as const,
    runtime: { status: "ready" as const, selection_revision: 3, selected, effective: selected },
  };

  it("Should clear the running-turn note once the live session reports the turn ended", () => {
    const target = { saved, liveApply: "next_step" as const };
    expect(currentRuntimeApplyNotice(target, { badge: undefined, runtime: undefined })?.text).toBe(
      "Applies at the agent's next step."
    );
    expect(currentRuntimeApplyNotice(target, { badge: "idle", runtime: saved.runtime })).toBeNull();
  });

  it("Should clear a refusal once a later prompt applies the stored selection", () => {
    const refused = {
      badge: "running" as const,
      runtime: {
        ...saved.runtime,
        failure: "acp: refused",
        effective: { ...selected, reasoning_effort: "xhigh" as const },
      },
    };
    const target = { saved: refused, liveApply: "next_step" as const };
    expect(currentRuntimeApplyNotice(target, refused)?.tone).toBe("danger");
    expect(
      currentRuntimeApplyNotice(target, {
        badge: "idle",
        runtime: { ...saved.runtime, failure: undefined },
      })
    ).toBeNull();
  });

  it("Should drop the note when a newer selection supersedes the save", () => {
    const target = { saved, liveApply: "next_turn" as const };
    expect(
      currentRuntimeApplyNotice(target, {
        badge: "running",
        runtime: { ...saved.runtime, selection_revision: 4 },
      })
    ).toBeNull();
  });
});
