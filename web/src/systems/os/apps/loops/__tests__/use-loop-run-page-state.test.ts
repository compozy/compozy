import { describe, expect, it } from "vitest";

import type { LoopRunEventFrame, LoopRunEventKind } from "@/systems/loops";

import { loopRunPageLogic } from "../use-loop-run-page-state";

function frame(
  kind: LoopRunEventKind,
  seq: number,
  scope: { runId: string; workspaceId: string } = { runId: "run-a", workspaceId: "workspace-a" }
): LoopRunEventFrame {
  const event = {
    at: "2026-07-25T12:00:00Z",
    id: `event-${seq}`,
    kind,
    loop_run_id: scope.runId,
    payload: { status: "running" },
    seq,
    workspace_id: scope.workspaceId,
  };
  return event as LoopRunEventFrame;
}

describe("loopRunPageLogic", () => {
  // Invariant: stream generations are monotonic while live state accumulates for
  // the current scope. Owning layer: store transition. Canonical suite: this file.
  it("Should preserve live state when the same run receives a newer stream generation", () => {
    const store = loopRunPageLogic.createStore({ runId: "run-a", workspaceId: "workspace-a" });
    const firstSubscription = {
      generation: 1,
      runId: "run-a",
      workspaceId: "workspace-a",
    };
    const secondSubscription = { ...firstSubscription, generation: 2 };

    store.trigger.streamFrameReceived({
      frame: frame("node_running", 1),
      subscription: firstSubscription,
    });
    store.trigger.streamFrameReceived({
      frame: frame("node_running", 2),
      subscription: secondSubscription,
    });
    expect(store.getSnapshot().context.live.frames).toHaveLength(2);
    expect(store.getSnapshot().context.streamGeneration).toBe(2);

    store.trigger.streamFrameReceived({
      frame: frame("node_running", 3),
      subscription: firstSubscription,
    });
    expect(store.getSnapshot().context.live.frames).toHaveLength(2);
  });

  // Invariant: a frame belongs only to the page's workspace/run scope.
  // Owning layer: unit transition logic. Canonical suite: this file, because no
  // other run-page state suite owns stream-scope validation.
  it("Should reject frames from a different run subscription or payload scope", () => {
    const store = loopRunPageLogic.createStore({ runId: "run-a", workspaceId: "workspace-a" });
    const currentSubscription = {
      generation: 1,
      runId: "run-a",
      workspaceId: "workspace-a",
    };

    store.trigger.streamFrameReceived({
      frame: frame("node_running", 1),
      subscription: currentSubscription,
    });
    expect(store.getSnapshot().context.live.frames).toHaveLength(1);

    store.trigger.streamFrameReceived({
      frame: frame("node_running", 2),
      subscription: {
        generation: 2,
        runId: "run-b",
        workspaceId: "workspace-b",
      },
    });
    expect(store.getSnapshot().context.live.frames).toHaveLength(1);

    store.trigger.streamFrameReceived({
      frame: frame("node_running", 3, { runId: "run-b", workspaceId: "workspace-b" }),
      subscription: { ...currentSubscription, generation: 2 },
    });
    expect(store.getSnapshot().context.live.frames).toHaveLength(1);
  });
});
