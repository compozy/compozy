import { createStoreLogic } from "@xstate/store";

import {
  applyLoopEventFrame,
  emptyLoopRunLiveState,
  type LoopRunEventFrame,
  type LoopRunLiveState,
} from "@/systems/loops";

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
    streamFrameReceived: (
      current,
      event: {
        frame: LoopRunEventFrame;
        subscription: { generation: number; runId: string; workspaceId: string };
      }
    ) => {
      if (event.subscription.generation < current.streamGeneration) return;
      if (
        event.subscription.workspaceId !== current.workspaceId ||
        event.subscription.runId !== current.runId ||
        event.frame.workspace_id !== current.workspaceId ||
        event.frame.loop_run_id !== current.runId
      ) {
        return;
      }
      return {
        ...current,
        live: applyLoopEventFrame(current.live, event.frame),
        streamGeneration: event.subscription.generation,
      };
    },
  },
});
