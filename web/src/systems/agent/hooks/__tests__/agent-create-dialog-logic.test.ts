import { describe, expect, it, vi } from "vitest";

import { createDefaultAgentCreateDraft } from "../../lib/agent-create-draft";
import { agentCreateDialogLogic } from "../agent-create-dialog-logic";

describe("agentCreateDialogLogic", () => {
  it("fences a stale submit completion after a new draft opens", () => {
    const store = agentCreateDialogLogic.createStore({
      initialDraft: createDefaultAgentCreateDraft(true),
    });
    const execute = vi.fn(() => new Promise<never>(() => {}));
    const firstDraft = { ...createDefaultAgentCreateDraft(true), name: "first" };
    const secondDraft = { ...createDefaultAgentCreateDraft(true), name: "second" };

    store.trigger.createOpened({ draft: firstDraft });
    store.trigger.submissionRequested({
      execute,
      failureMessage: "Failed to create agent.",
    });
    store.trigger.submissionRequested({
      execute,
      failureMessage: "Failed to create agent.",
    });
    const firstAttempt = store.getSnapshot().context;
    expect(firstAttempt.status).toBe("submitting");
    expect(execute).toHaveBeenCalledOnce();
    if (firstAttempt.status !== "submitting") {
      throw new Error("The first submission did not start.");
    }

    store.trigger.dialogDismissed({ draft: createDefaultAgentCreateDraft(true) });
    store.trigger.createOpened({ draft: secondDraft });
    store.trigger.submissionSucceeded({
      agentName: "first",
      attempt: firstAttempt.attempt,
    });

    expect(store.getSnapshot().context).toMatchObject({
      status: "editing",
      draft: secondDraft,
    });
  });
});
