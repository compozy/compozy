import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { useCurrentWindowLiveDataEnabled } from "../../hooks/use-window-live-data-enabled";
import { copyTaskRecordId } from "./copy-record-id";
import {
  computeElapsed,
  taskRunCanRecover,
  useForceFailDialog,
  useLiveElapsed,
  useTaskRunPage,
  useTaskRuns,
  useTaskStream,
  useTaskTimeline,
} from "@/systems/tasks";

/**
 * Controller for the run-detail window location: run page data, sibling runs
 * for lineage, run-scoped timeline slice, live stream attachment, and
 * WM-location navigation.
 */
export function useTaskRunLocation(taskId: string, runId: string) {
  const liveDataEnabled = useCurrentWindowLiveDataEnabled();
  const page = useTaskRunPage(taskId, runId, { liveDataEnabled });
  const [inspectOpen, setInspectOpen] = useState(false);
  const forceFailDialog = useForceFailDialog(page.handleForceFailRun);

  const authoritativeTaskId = page.run?.run.task_id ?? page.run?.task?.id ?? "";
  const related = useTaskRunRelatedData(authoritativeTaskId, page.isLive, liveDataEnabled);
  const navigation = useTaskRunNavigation({ authoritativeTaskId, page, taskId });
  const presentation = useTaskRunPresentation(page, liveDataEnabled);

  return {
    authoritativeTaskId,
    ...navigation,
    ...presentation,
    forceFailDialog,
    inspectOpen,
    page,
    runId,
    setInspectOpen,
    taskId,
    ...related,
  };
}

function useTaskRunRelatedData(
  authoritativeTaskId: string,
  isLive: boolean,
  liveDataEnabled: boolean
) {
  const refetchIntervalMs = isLive && liveDataEnabled ? undefined : false;
  const timelineQuery = useTaskTimeline(
    authoritativeTaskId,
    {},
    { enabled: Boolean(authoritativeTaskId) && liveDataEnabled, refetchIntervalMs }
  );
  const runsQuery = useTaskRuns(
    authoritativeTaskId,
    {},
    {
      enabled: Boolean(authoritativeTaskId) && liveDataEnabled,
      refetchIntervalMs,
    }
  );

  return {
    taskRuns: runsQuery.data ?? [],
    taskRunsError: runsQuery.error ?? null,
    taskRunsLoading: runsQuery.isLoading && !runsQuery.data,
    timelineItems: timelineQuery.data ?? [],
    timelineError: timelineQuery.error ?? null,
    timelineLoading: timelineQuery.isLoading && !timelineQuery.data,
  };
}

function useTaskRunNavigation({
  authoritativeTaskId,
  page,
  taskId,
}: {
  authoritativeTaskId: string;
  page: ReturnType<typeof useTaskRunPage>;
  taskId: string;
}) {
  const navigate = useNavigate();
  return {
    backToTask: () => {
      void navigate({ to: "/tasks/$id", params: { id: authoritativeTaskId || taskId } });
    },
    backToTasks: () => {
      void navigate({ to: "/tasks" });
    },
    openSession: (sessionId: string) => {
      const agentName =
        page.session?.session_id === sessionId ? page.session.agent_name?.trim() : undefined;
      if (agentName) {
        void navigate({
          to: "/agents/$name/sessions/$id",
          params: { name: agentName, id: sessionId },
        });
        return;
      }
      void navigate({ to: "/session/$id", params: { id: sessionId } });
    },
  };
}

function useTaskRunPresentation(page: ReturnType<typeof useTaskRunPage>, liveDataEnabled: boolean) {
  // Attach the task stream before the page advertises live state.
  const authoritativeTaskId = page.run?.run.task_id ?? page.run?.task?.id ?? "";
  const latestEventSeq = page.task?.task?.latest_event_seq;
  const hasEventSeq = typeof latestEventSeq === "number";
  useTaskStream(authoritativeTaskId, {
    enabled: liveDataEnabled && Boolean(authoritativeTaskId) && hasEventSeq,
    afterSequence: hasEventSeq ? Math.max(0, latestEventSeq) : undefined,
  });

  const record = page.run?.run ?? null;
  const runActive = record?.status === "running" || record?.status === "starting";
  const liveElapsed = useLiveElapsed(record?.started_at, runActive && liveDataEnabled);
  const runDuration = record ? (runActive ? liveElapsed : computeElapsed(record)) : undefined;
  const canRecover = Boolean(
    record &&
    page.task &&
    record.status === "needs_attention" &&
    taskRunCanRecover(record, page.task.task.max_attempts)
  );

  return {
    canRecover,
    copyRunId: () => {
      if (!record) return;
      void copyTaskRecordId(record.id, "Run");
    },
    record,
    runDuration,
  };
}

export type TaskRunLocationController = ReturnType<typeof useTaskRunLocation>;
