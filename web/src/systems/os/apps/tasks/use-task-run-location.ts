import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import {
  computeElapsed,
  taskRunCanRecover,
  useLiveElapsed,
  useForceFailDialog,
  useTaskRunPage,
  useTaskRuns,
  useTaskStream,
  useTaskTimeline,
} from "@/systems/tasks";

import { copyTaskRecordId } from "./copy-record-id";

/**
 * Controller for the run-detail window location: run page data, sibling runs
 * for lineage, run-scoped timeline slice, live stream attachment, and
 * WM-location navigation.
 */
export function useTaskRunLocation(taskId: string, runId: string) {
  const navigate = useNavigate();
  const page = useTaskRunPage(taskId, runId);
  const [inspectOpen, setInspectOpen] = useState(false);
  const forceFailDialog = useForceFailDialog(page.handleForceFailRun);

  const authoritativeTaskId = page.run?.run.task_id ?? page.run?.task?.id ?? "";
  const timelineQuery = useTaskTimeline(
    authoritativeTaskId,
    {},
    { enabled: Boolean(authoritativeTaskId) }
  );
  const runsQuery = useTaskRuns(authoritativeTaskId, {}, { enabled: Boolean(authoritativeTaskId) });

  // Attach the task stream before the page advertises live state.
  const latestEventSeq = page.task?.task?.latest_event_seq;
  const hasEventSeq = typeof latestEventSeq === "number";
  useTaskStream(authoritativeTaskId, {
    enabled: Boolean(authoritativeTaskId) && hasEventSeq,
    afterSequence: hasEventSeq ? Math.max(0, latestEventSeq) : undefined,
  });

  const record = page.run?.run ?? null;
  const runActive = record?.status === "running" || record?.status === "starting";
  const liveElapsed = useLiveElapsed(record?.started_at, runActive);
  const runDuration = record ? (runActive ? liveElapsed : computeElapsed(record)) : undefined;

  const backToTask = () => {
    void navigate({ to: "/tasks/$id", params: { id: authoritativeTaskId || taskId } });
  };

  const backToTasks = () => {
    void navigate({ to: "/tasks" });
  };

  const openSession = (sessionId: string) => {
    void navigate({ to: "/session/$id", params: { id: sessionId } });
  };

  const copyRunId = () => {
    if (!record) return;
    void copyTaskRecordId(record.id, "Run");
  };

  const canRecover =
    record && page.task
      ? record.status === "needs_attention" &&
        taskRunCanRecover(record, page.task.task.max_attempts)
      : false;

  return {
    authoritativeTaskId,
    backToTask,
    backToTasks,
    canRecover,
    copyRunId,
    forceFailDialog,
    inspectOpen,
    openSession,
    page,
    record,
    runId,
    runDuration,
    setInspectOpen,
    taskId,
    taskRuns: runsQuery.data ?? [],
    taskRunsError: runsQuery.error ?? null,
    taskRunsLoading: runsQuery.isLoading && !runsQuery.data,
    timelineItems: timelineQuery.data ?? [],
    timelineError: timelineQuery.error ?? null,
    timelineLoading: timelineQuery.isLoading && !timelineQuery.data,
  };
}

export type TaskRunLocationController = ReturnType<typeof useTaskRunLocation>;
