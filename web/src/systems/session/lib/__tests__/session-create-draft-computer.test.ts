// Suite: Session create draft computer immutability after send (UT-082)
// Invariant: Changing the target computer is allowed before send and becomes a no-op once sent.
// Boundary IN: session-create-draft helpers and sessionCreateStoreLogic.
// Boundary OUT: Network session creation and navigation.

import { describe, expect, it } from "vitest";

import {
  EMPTY_SESSION_CREATE_DRAFT,
  markSessionDraftSent,
  setSessionDraftComputer,
} from "../session-create-draft";
import { createSessionCreateStore } from "../../stores/session-create-store";

describe("session create draft computer immutability (UT-082)", () => {
  it("UT-082: Should allow changing computer before send and treat setComputer as a no-op after send", () => {
    const beforeSend = setSessionDraftComputer(EMPTY_SESSION_CREATE_DRAFT, "cmp_7f3k2q9x");
    expect(beforeSend.computerId).toBe("cmp_7f3k2q9x");
    expect(beforeSend.sent).toBe(false);

    const sent = markSessionDraftSent(beforeSend);
    expect(sent.sent).toBe(true);

    const afterAttempt = setSessionDraftComputer(sent, "local");
    expect(afterAttempt).toBe(sent);
    expect(afterAttempt.computerId).toBe("cmp_7f3k2q9x");
  });

  it("UT-082: Should ignore computerSelected store events once submissionRequested has fired", () => {
    const store = createSessionCreateStore();
    store.trigger.dialogOpened({ agentName: "general", workspaceId: "ws_1" });
    store.trigger.computerSelected({ computerId: "cmp_7f3k2q9x" });
    expect(store.getSnapshot().context.draft.computerId).toBe("cmp_7f3k2q9x");

    store.trigger.submissionRequested({
      agentName: "general",
      workspaceId: "ws_1",
      execute: () => new Promise(() => undefined),
      navigate: async () => undefined,
    });
    expect(store.getSnapshot().context.draft.sent).toBe(true);

    store.trigger.computerSelected({ computerId: "local" });
    expect(store.getSnapshot().context.draft.computerId).toBe("cmp_7f3k2q9x");
  });
});
