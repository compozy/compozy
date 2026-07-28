import { describe, expect, it } from "vitest";

import type { LoopRunEventFrame, LoopRunEventKind } from "@/systems/loops";

import { loopRunPageLogic } from "../use-loop-run-page-state";

function frame(
  kind: LoopRunEventKind,
  seq: number,
  scope: { runId: string; workspaceId: string } = { runId: "run-a", workspaceId: "workspace-a" }
): LoopRunEventFrame {
  return {
    at: "2026-07-25T12:00:00Z",
    id: `event-${seq}`,
    kind,
    loop_run_id: scope.runId,
    payload: { status: "running" },
    seq,
    workspace_id: scope.workspaceId,
  };
}

describe("loopRunPageLogic", () => {
  it("Should preserve live state when the same run opens a newer stream generation", () => {
    const store = loopRunPageLogic.createStore({ runId: "run-a", workspaceId: "workspace-a" });

    store.trigger.streamSubscriptionOpened({
      generation: 1,
      runId: "run-a",
      workspaceId: "workspace-a",
    });
    store.trigger.streamFrameReceived({ frame: frame("node_running", 1), generation: 1 });

    store.trigger.streamSubscriptionOpened({
      generation: 2,
      runId: "run-a",
      workspaceId: "workspace-a",
    });

    expect(store.getSnapshot().context.live.frames).toHaveLength(1);
    expect(store.getSnapshot().context.streamGeneration).toBe(2);
    store.trigger.streamFrameReceived({ frame: frame("node_running", 2), generation: 1 });
    expect(store.getSnapshot().context.live.frames).toHaveLength(1);
  });

  // Invariant: a frame belongs only to the active run-stream generation; changing
  // the workspace/run scope clears live projection and rejects late frames.
  // Owning layer: unit transition logic. Canonical suite: this file, because no
  // existing run-page state suite owns generation-fenced stream transitions.
  it("Should reject late stream frames after the run subscription changes", () => {
    const store = loopRunPageLogic.createStore({ runId: "run-a", workspaceId: "workspace-a" });

    store.trigger.streamSubscriptionOpened({
      generation: 1,
      runId: "run-a",
      workspaceId: "workspace-a",
    });
    store.trigger.streamFrameReceived({ frame: frame("node_running", 1), generation: 1 });
    expect(store.getSnapshot().context.live.frames).toHaveLength(1);

    store.trigger.streamSubscriptionOpened({
      generation: 2,
      runId: "run-b",
      workspaceId: "workspace-b",
    });
    expect(store.getSnapshot().context.live.frames).toHaveLength(0);
    expect(store.getSnapshot().context.streamGeneration).toBe(2);

    store.trigger.streamFrameReceived({ frame: frame("node_running", 2), generation: 1 });
    expect(store.getSnapshot().context.live.frames).toHaveLength(0);

    store.trigger.streamFrameReceived({ frame: frame("node_running", 3), generation: 2 });
    expect(store.getSnapshot().context.live.frames).toHaveLength(0);

    store.trigger.streamFrameReceived({
      frame: frame("node_running", 4, { runId: "run-b", workspaceId: "workspace-b" }),
      generation: 2,
    });
    expect(store.getSnapshot().context.live.frames).toHaveLength(1);
  });
});
