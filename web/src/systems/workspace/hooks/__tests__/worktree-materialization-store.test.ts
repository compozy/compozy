import { describe, expect, it, vi } from "vitest";

import { buildWorktreeFixture } from "../../mocks/worktree-fixtures";
import type { WorktreePayload } from "../../types";
import { worktreeMaterializationLogic } from "../worktree-materialization-store";

function pendingWorktree(id: string): WorktreePayload {
  return buildWorktreeFixture({
    id,
    name: "payments-retry",
    state: "pending",
    pending_phase: "setup",
    setup_state: "none",
    dirty: null,
    ahead: null,
    behind: null,
  });
}

describe("worktreeMaterializationLogic", () => {
  it("Should allow a failed materialization to retry with the same name", async () => {
    const store = worktreeMaterializationLogic.createStore();
    const first = vi.fn().mockResolvedValue(pendingWorktree("wt-failed"));

    store.trigger.startRequested({ name: "payments-retry", execute: first, cancel: vi.fn() });
    await vi.waitFor(() => expect(store.getSnapshot().context.status).toBe("active"));
    store.trigger.failureObserved({ worktreeId: "wt-failed", error: "setup failed" });
    expect(store.getSnapshot().context.status).toBe("failed");

    const retry = vi.fn().mockResolvedValue(pendingWorktree("wt-retry"));
    store.trigger.retryRequested({ execute: retry, cancel: vi.fn() });

    await vi.waitFor(() => expect(store.getSnapshot().context.status).toBe("active"));
    expect(retry).toHaveBeenCalledWith("payments-retry");
    expect(store.getSnapshot().context.worktreeId).toBe("wt-retry");
  });

  it("Should cancel a create whose id arrives after the operator canceled", async () => {
    const store = worktreeMaterializationLogic.createStore();
    let resolveCreate: ((worktree: WorktreePayload) => void) | undefined;
    const create = vi.fn(
      () =>
        new Promise<WorktreePayload>(resolve => {
          resolveCreate = resolve;
        })
    );
    const cancel = vi.fn().mockResolvedValue(undefined);

    store.trigger.startRequested({ name: "payments-retry", execute: create, cancel });
    store.trigger.cancelRequested({ execute: cancel });
    store.trigger.reset({});
    resolveCreate?.(pendingWorktree("wt-late"));

    await vi.waitFor(() => expect(cancel).toHaveBeenCalledWith("wt-late"));
    expect(store.getSnapshot().context.status).toBe("idle");
  });
});
