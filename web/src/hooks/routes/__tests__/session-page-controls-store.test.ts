import { waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("sonner", () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

import { toast } from "sonner";

import { createSessionPageControlsLogic } from "../session-page-controls-store";

// Suite: Session page-controls workflow ownership.
// Invariant: one accepted busy-input intent owns the client operation until its
// acknowledgement settles, without manufacturing daemon queue state or success notices.
// Boundary IN: accepted control intents and async executor settlements.
// Boundary OUT: store phases and emitted failures.
describe("session page controls store", () => {
  it("Should serialize busy-input intents until the accepted request settles", () => {
    const store = createSessionPageControlsLogic().createStore();
    const pending = () => new Promise<never>(() => undefined);
    let snapshot = store.getInitialSnapshot();
    [snapshot] = store.transition(snapshot, {
      type: "busyInputRequested",
      execute: pending,
      kind: "queue",
      message: "Follow up",
    });
    const requestId = snapshot.context.busyInput.requestId;
    [snapshot] = store.transition(snapshot, {
      type: "busyInputRequested",
      execute: pending,
      kind: "interrupt",
      message: "Replace it",
    });

    expect(snapshot.context.busyInput).toMatchObject({ phase: "pending", requestId });
    expect(snapshot.context.nextRequestId).toBe(requestId);
  });

  it("Should return to idle without a success toast after acknowledgement", () => {
    const store = createSessionPageControlsLogic().createStore();
    let snapshot = store.getInitialSnapshot();
    [snapshot] = store.transition(snapshot, {
      type: "busyInputRequested",
      execute: vi.fn().mockResolvedValue(undefined),
      kind: "steer",
      message: "Use the new constraint",
    });
    [snapshot] = store.transition(snapshot, {
      type: "busyInputSucceeded",
      requestId: snapshot.context.busyInput.requestId,
    });

    expect(snapshot.context.busyInput.phase).toBe("idle");
    expect(toast.success).not.toHaveBeenCalled();
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
