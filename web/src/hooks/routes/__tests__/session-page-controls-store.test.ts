import { waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("sonner", () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

import { toast } from "sonner";

import { createSessionPageControlsLogic } from "../session-page-controls-store";

// Suite: Session page-controls workflow ownership.
// Invariant: only live queue entries may start edits, stop failures remain
// observable, and an obsolete queue scope cannot be restored by a late settle.
// Boundary IN: accepted control intents and async executor settlements.
// Boundary OUT: store phases, optimistic queue state, and emitted notices.
describe("session page controls store", () => {
  it("Should fence optimistic queue restores after the owning turn changes", () => {
    const store = createSessionPageControlsLogic().createStore();
    const execute = () => new Promise<never>(() => undefined);
    const [pendingQueue] = store.transition(store.getInitialSnapshot(), {
      type: "busyInputRequested",
      execute,
      kind: "queue",
      message: "Follow up",
    });
    const [queued] = store.transition(pendingQueue, {
      type: "busyInputSucceeded",
      requestId: pendingQueue.context.busyInput.requestId,
      result: { queued: true, queue_entry_id: "queue-1" },
    });
    const [removing] = store.transition(queued, {
      type: "queuedPromptRemovalRequested",
      execute,
      queueEntryId: "queue-1",
    });
    const requestId = removing.context.nextRequestId;
    const [nextTurn] = store.transition(removing, {
      type: "queueScopeChanged",
      queueScope: "running:turn-b",
    });
    const [afterLateRestore] = store.transition(nextTurn, {
      type: "queuedPromptEditFailed",
      error: new Error("late failure"),
      requestId,
      stage: "cancel",
    });

    expect(afterLateRestore.context.queuedPrompts).toEqual([]);
  });

  it("Should reject a queued steer whose prompt is no longer queued", () => {
    const store = createSessionPageControlsLogic().createStore();
    const steer = vi.fn().mockResolvedValue(undefined);
    const cancel = vi.fn().mockResolvedValue(undefined);

    store.trigger.queuedPromptSteerRequested({
      cancel,
      prompt: { id: "missing", text: "No longer queued" },
      steer,
    });

    expect(steer).not.toHaveBeenCalled();
    expect(cancel).not.toHaveBeenCalled();
    expect(store.getSnapshot().context.pendingQueueEdits).toEqual({});
  });

  it("Should emit the real error when a direct stop executor fails", async () => {
    const store = createSessionPageControlsLogic().createStore();

    store.trigger.stopRequested({
      execute: vi.fn().mockRejectedValue(new Error("daemon disconnected")),
      failureMessage: null,
    });

    await waitFor(() => expect(store.getSnapshot().context.stop.phase).toBe("idle"));
    expect(toast.error).toHaveBeenCalledWith("daemon disconnected");
  });
});
