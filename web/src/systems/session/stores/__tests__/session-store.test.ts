import { beforeEach, describe, expect, it, vi } from "vitest";

import { FIRST_PROMPT_SEND_FAILED, sendFirstPrompt } from "../../lib/session-first-prompt";
import { sessionStore } from "../session-store";

const { mockNotifyUser } = vi.hoisted(() => ({ mockNotifyUser: vi.fn() }));

vi.mock("@/lib/user-feedback", () => ({ notifyUser: mockNotifyUser }));

// Suite: session store transitions
// Invariant: Empty drafts are removed, session-scoped operations never affect another session, goal
// feedback keeps command, result, and error acknowledgement coherent, and a first prompt typed before
// its session existed is delivered at most once and never destroyed by a refusal.
// Boundary IN: Session-local drafts, first-prompt handoff (store transitions + `sendFirstPrompt`),
// and Goal feedback transitions.
// Boundary OUT: React subscriptions and prompt transport, owned by their consumer suites.
describe("session store", () => {
  it("removes an empty composer draft and reports discard availability", () => {
    const initial = sessionStore.getInitialSnapshot();

    expect(sessionStore.can.composerDraftDiscarded({ sessionId: "session-a" })).toBe(false);
    sessionStore.trigger.composerDraftChanged({ sessionId: "session-a", text: "Hello" });
    expect(sessionStore.can.composerDraftDiscarded({ sessionId: "session-a" })).toBe(true);
    sessionStore.trigger.composerDraftDiscarded({ sessionId: "session-a" });

    const [withDraft] = sessionStore.transition(initial, {
      type: "composerDraftChanged",
      sessionId: "session-a",
      text: "Hello",
    });
    const [withoutDraft] = sessionStore.transition(withDraft, {
      type: "composerDraftChanged",
      sessionId: "session-a",
      text: "",
    });

    expect(withoutDraft.context.drafts["session-a"]).toBeUndefined();
    expect(sessionStore.can.composerDraftDiscarded({ sessionId: "session-a" })).toBe(false);
  });

  it("removes one session interaction without touching another session", () => {
    const [withFirstDraft] = sessionStore.transition(sessionStore.getInitialSnapshot(), {
      type: "composerDraftChanged",
      sessionId: "session-a",
      text: "Alpha draft",
    });
    const [withBothDrafts] = sessionStore.transition(withFirstDraft, {
      type: "composerDraftChanged",
      sessionId: "session-b",
      text: "Bravo draft",
    });
    const [withFirstFeedback] = sessionStore.transition(withBothDrafts, {
      type: "goalCommandReported",
      sessionId: "session-a",
      command: "/goal alpha",
      result: { outcome: "status", reason_code: null, replaced_run_id: null, snapshot: null },
    });
    const [withBothFeedback] = sessionStore.transition(withFirstFeedback, {
      type: "goalCommandReported",
      sessionId: "session-b",
      command: "/goal bravo",
      result: { outcome: "status", reason_code: null, replaced_run_id: null, snapshot: null },
    });
    const [withFirstPrompts] = sessionStore.transition(withBothFeedback, {
      type: "firstPromptQueued",
      sessionId: "session-a",
      text: "Alpha first message",
    });
    const [withBothPrompts] = sessionStore.transition(withFirstPrompts, {
      type: "firstPromptQueued",
      sessionId: "session-b",
      text: "Bravo first message",
    });
    const [snapshot] = sessionStore.transition(withBothPrompts, {
      type: "sessionInteractionRemoved",
      sessionId: "session-a",
    });

    expect(snapshot.context.drafts["session-a"]).toBeUndefined();
    expect(snapshot.context.goalFeedback["session-a"]).toBeUndefined();
    expect(snapshot.context.firstPrompts["session-a"]).toBeUndefined();
    expect(snapshot.context.drafts["session-b"]).toBe("Bravo draft");
    expect(snapshot.context.goalFeedback["session-b"]?.command).toBe("/goal bravo");
    expect(snapshot.context.firstPrompts["session-b"]?.text).toBe("Bravo first message");
  });

  it("records Goal feedback atomically and acknowledges only visible errors", () => {
    const [reported] = sessionStore.transition(sessionStore.getInitialSnapshot(), {
      type: "goalCommandReported",
      sessionId: "session-a",
      command: "/goal replace run_1 Rewrite the plan",
      result: {
        outcome: "error",
        reason_code: "goal_replace_stale",
        replaced_run_id: null,
        snapshot: null,
      },
    });
    const [acknowledged] = sessionStore.transition(reported, {
      type: "goalErrorAcknowledged",
      sessionId: "session-a",
    });

    expect(reported.context.goalFeedback["session-a"]).toMatchObject({
      command: "/goal replace run_1 Rewrite the plan",
      errorVisible: true,
      result: { outcome: "error", reason_code: "goal_replace_stale" },
    });
    expect(acknowledged.context.goalFeedback["session-a"]?.errorVisible).toBe(false);
    expect(sessionStore.can.goalErrorAcknowledged({ sessionId: "session-a" })).toBe(false);
  });

  // These exercise `sendFirstPrompt` against the live store rather than a pure
  // transition, because the exactly-once guard is the read-then-claim pair
  // itself — a transition test would step over the very thing being verified.
  describe("first prompt handoff", () => {
    const sessionId = "session-first-prompt";

    beforeEach(() => {
      mockNotifyUser.mockReset();
      sessionStore.trigger.sessionInteractionRemoved({ sessionId });
    });

    it("Should deliver a queued first prompt once and forget it", () => {
      const send = vi.fn();
      sessionStore.trigger.firstPromptQueued({ sessionId, text: "Investigate the regression" });

      sendFirstPrompt(sessionId, send);
      sendFirstPrompt(sessionId, send);

      expect(send).toHaveBeenCalledTimes(1);
      expect(send).toHaveBeenCalledWith("Investigate the regression");
      expect(sessionStore.getSnapshot().context.firstPrompts[sessionId]).toBeUndefined();
      expect(sessionStore.getSnapshot().context.drafts[sessionId]).toBeUndefined();
    });

    it("Should not send again once the prompt is claimed by an earlier attempt", () => {
      sessionStore.trigger.firstPromptQueued({ sessionId, text: "Only once" });
      sessionStore.trigger.firstPromptClaimed({ sessionId });

      const send = vi.fn();
      sendFirstPrompt(sessionId, send);

      expect(send).not.toHaveBeenCalled();
    });

    it("Should keep a refused first prompt as a composer draft instead of losing it", () => {
      const send = vi.fn(() => {
        throw new Error("composer refused the message");
      });
      sessionStore.trigger.firstPromptQueued({ sessionId, text: "Words worth keeping" });

      sendFirstPrompt(sessionId, send);

      expect(sessionStore.getSnapshot().context.drafts[sessionId]).toBe("Words worth keeping");
      expect(sessionStore.getSnapshot().context.firstPrompts[sessionId]).toBeUndefined();
      expect(mockNotifyUser).toHaveBeenCalledWith({
        message: FIRST_PROMPT_SEND_FAILED,
        tone: "error",
      });
    });

    it("Should do nothing when no first prompt was queued", () => {
      const send = vi.fn();

      sendFirstPrompt(sessionId, send);

      expect(send).not.toHaveBeenCalled();
      expect(mockNotifyUser).not.toHaveBeenCalled();
    });

    it("Should refuse to queue a blank first prompt", () => {
      sessionStore.trigger.firstPromptQueued({ sessionId, text: "   " });

      expect(sessionStore.getSnapshot().context.firstPrompts[sessionId]).toBeUndefined();
    });
  });
});
