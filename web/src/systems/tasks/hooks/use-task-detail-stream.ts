import { useSelector } from "@xstate/store-react";

import { useStoreBinding } from "@/hooks/use-store-binding";

import { useTaskStream } from "./use-task-stream";
import { taskStreamStatusLogic } from "../lib/task-stream-state";

function streamFailureMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  if (typeof error === "string") return error;
  return "Stream connection failed";
}

/**
 * Keeps the task detail fresh from run-lifecycle SSE events. It waits for the
 * detail payload before connecting so it seeds from the real cursor instead of
 * after_sequence=0 (a full-history replay + immediate reconnect when
 * latest_event_seq resolves).
 */
function useTaskDetailStream(
  taskId: string,
  detailEventSeq: number | null | undefined,
  enabled: boolean
) {
  const hasEventSeq = typeof detailEventSeq === "number";
  const streamEnabled = enabled && hasEventSeq;
  const streamSeedSequence = hasEventSeq ? Math.max(0, detailEventSeq) : 0;
  const streamKey = streamEnabled ? `${taskId}:${detailEventSeq}` : "disabled";
  const { store: streamStatusStore } = useStoreBinding(streamKey, () =>
    taskStreamStatusLogic.createStore({ enabled: streamEnabled })
  );
  const currentStreamStatus = useSelector(streamStatusStore, snapshot => snapshot.context);
  useTaskStream(taskId, {
    enabled: streamEnabled,
    afterSequence: hasEventSeq ? streamSeedSequence : undefined,
    onEvent: () => streamStatusStore.trigger.frameReceived(),
    onError: error =>
      streamStatusStore.trigger.streamFailed({ error: streamFailureMessage(error) }),
  });

  return {
    streamErrorMessage: currentStreamStatus.error,
    streamSeedSequence,
    streamState: currentStreamStatus.state,
  };
}

export { useTaskDetailStream };
