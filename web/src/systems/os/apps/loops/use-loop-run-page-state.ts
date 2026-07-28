import { createStoreLogic } from "@xstate/store";

import { applyLoopEventFrame, emptyLoopRunLiveState, type LoopRunLiveState } from "@/systems/loops";
import type { LoopRunEventFrame } from "@/systems/loops";

interface LoopRunPageState {
  live: LoopRunLiveState;
  runId: string;
  streamGeneration: number;
  workspaceId: string;
}

export const loopRunPageLogic = createStoreLogic({
  context: (input: { runId: string; workspaceId: string }): LoopRunPageState => ({
    live: emptyLoopRunLiveState(),
    runId: input.runId,
    streamGeneration: 0,
    workspaceId: input.workspaceId,
  }),
  on: {
    streamSubscriptionOpened: (
      current,
      event: { generation: number; runId: string; workspaceId: string }
    ) => {
      const sameScope = event.workspaceId === current.workspaceId && event.runId === current.runId;
      if (sameScope && event.generation === current.streamGeneration) return;
      if (sameScope) {
        return { ...current, streamGeneration: event.generation };
      }
      return {
        live: emptyLoopRunLiveState(),
        runId: event.runId,
        streamGeneration: event.generation,
        workspaceId: event.workspaceId,
      };
    },
    streamFrameReceived: (current, event: { frame: LoopRunEventFrame; generation: number }) => {
      if (event.generation !== current.streamGeneration) return;
      if (
        event.frame.workspace_id !== current.workspaceId ||
        event.frame.loop_run_id !== current.runId
      ) {
        return;
      }
      return { ...current, live: applyLoopEventFrame(current.live, event.frame) };
    },
  },
});
