import { useTaskDetailActions } from "./use-task-detail-actions";
import { useTaskDetailReads, type TaskDetailReadOptions } from "./use-task-detail-reads";
import { useTaskDetailStream } from "./use-task-detail-stream";

interface UseTaskDetailPageOptions extends TaskDetailReadOptions {
  enableStream?: boolean;
}

/**
 * View model for the 3-tab task detail page. Tab state lives in the window
 * location (URL-addressable); this hook owns queries, the SSE stream, and
 * every task-level verb handler.
 */
function useTaskDetailPage(taskId: string, options: UseTaskDetailPageOptions = {}) {
  const reads = useTaskDetailReads(taskId, options);
  const streamEnabled =
    Boolean(taskId) && (options.enableStream ?? true) && (options.liveDataEnabled ?? true);
  const stream = useTaskDetailStream(taskId, reads.detail?.task?.latest_event_seq, streamEnabled);
  const actions = useTaskDetailActions(taskId, reads.activeRun, reads.detail?.task.max_attempts);

  return { ...reads, ...stream, ...actions, taskId };
}

export { useTaskDetailPage };
export type { UseTaskDetailPageOptions };
