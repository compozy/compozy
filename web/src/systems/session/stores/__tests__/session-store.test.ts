import { describe, expect, it } from "vitest";

import { sessionStore } from "../session-store";

// Suite: session store transitions
// Invariant: Empty drafts are removed, session-scoped operations never affect another session, and goal feedback keeps command, result, and error acknowledgement coherent.
// Boundary IN: Session-local drafts and Goal feedback transitions.
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
    const [snapshot] = sessionStore.transition(withBothFeedback, {
      type: "sessionInteractionRemoved",
      sessionId: "session-a",
    });

    expect(snapshot.context.drafts["session-a"]).toBeUndefined();
    expect(snapshot.context.goalFeedback["session-a"]).toBeUndefined();
    expect(snapshot.context.drafts["session-b"]).toBe("Bravo draft");
    expect(snapshot.context.goalFeedback["session-b"]?.command).toBe("/goal bravo");
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
});
