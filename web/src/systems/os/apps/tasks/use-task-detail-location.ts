import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { toast } from "sonner";

import {
  computeElapsed,
  resolveTaskCommandState,
  TASK_RESULT_ANCHOR_ID,
  useProfileEditor,
  useDeleteTask,
  useLiveElapsed,
  useTaskDetailPage,
  useTaskOperatorLayer,
  useTaskPauseDialog,
  useTaskSetupRuntime,
  useUpdateTask,
  type ResolvedTaskDetailSearch,
  type TaskDetailTab,
  type TaskInspectTarget,
  type TaskPriority,
} from "@/systems/tasks";

import { useOsShell } from "../../hooks/use-os-shell";
import { copyTaskRecordId } from "./copy-record-id";

function scrollToResult() {
  document.getElementById(TASK_RESULT_ANCHOR_ID)?.scrollIntoView({ block: "start" });
}

/**
 * Controller for the task-detail window location: data (via
 * `useTaskDetailPage` + `useTaskOperatorLayer`), overlay state, WM-location
 * navigation, and the runtime-backed command state. The location component only renders.
 */
export function useTaskDetailLocation(taskId: string, search: ResolvedTaskDetailSearch) {
  const navigate = useNavigate();
  const { coordinator } = useOsShell();
  const page = useTaskDetailPage(taskId);
  const deleteMutation = useDeleteTask();
  const updateMutation = useUpdateTask();
  const [overlay, setOverlay] = useState<"setup" | "setup_clear" | "fan_out" | "delete" | null>(
    null
  );
  const inspectOpen = search.inspect !== undefined;
  const pauseDialog = useTaskPauseDialog(page.handlePauseTask);
  const latestEventSeq = page.detail?.task?.latest_event_seq ?? null;
  const operator = useTaskOperatorLayer(taskId, { enabled: inspectOpen, latestEventSeq });
  const setupRuntime = useTaskSetupRuntime(page.detail?.task.workspace_id);
  const setupEditor = useProfileEditor({
    onSetProfile: operator.handleSetProfile,
    profile: page.profile,
    taskId,
  });

  const detail = page.detail;
  const record = detail?.task ?? null;
  const activeElapsed = useLiveElapsed(page.activeRun?.started_at, page.isLive);
  const runDurations = new Map(
    page.runs.map(run => [
      run.id,
      run.id === page.activeRun?.id && page.isLive ? activeElapsed : computeElapsed(run),
    ])
  );
  const command = detail ? resolveTaskCommandState(detail, page.runs) : null;
  // Fan-out designations are supplied per invocation, so the entry stays
  // available for any non-terminal, published task.
  const showFanOut = Boolean(command?.canFanOut);

  const setTab = (tab: TaskDetailTab) => {
    void navigate({
      to: "/tasks/$id",
      params: { id: taskId },
      search: { tab, inspect: search.inspect },
      replace: true,
    });
  };

  const setInspectTarget = (inspect: TaskInspectTarget) => {
    void navigate({
      to: "/tasks/$id",
      params: { id: taskId },
      search: { tab: search.tab, inspect },
      replace: true,
    });
  };

  const setInspectOpen = (open: boolean) => {
    if (open) {
      setInspectTarget(search.inspect ?? "diagnostics");
      return;
    }
    void navigate({
      to: "/tasks/$id",
      params: { id: taskId },
      search: { tab: search.tab },
      replace: true,
    });
  };

  const openRun = (runId: string) => {
    void navigate({ to: "/tasks/$id/runs/$runId", params: { id: taskId, runId } });
  };

  const openTask = (id: string) => {
    void navigate({ to: "/tasks/$id", params: { id } });
  };

  // The editor layers over this location, so the active tab rides along and
  // dismissal returns to the tab the operator was reading.
  const openEdit = () => {
    void navigate({ to: "/tasks/$id/edit", params: { id: taskId }, search: { tab: search.tab } });
  };

  const backToTasks = () => {
    void navigate({ to: "/tasks" });
  };

  const handleDeleteTask = async (id: string) => {
    void coordinator.userOpen({ app: "tasks", route: { pathname: "/tasks", search: {} } });
    try {
      await deleteMutation.mutateAsync({ id });
      toast.success("Task deleted.");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to delete task");
    }
  };

  const handlePriorityChange = async (priority: TaskPriority) => {
    if (!record || record.priority === priority) return;
    try {
      await updateMutation.mutateAsync({ id: record.id, data: { priority } });
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Couldn't change priority");
    }
  };

  const handleAutoEnqueueChange = async (enabled: boolean) => {
    if (!record || Boolean(record.auto_enqueue_on_ready) === enabled) return;
    try {
      await updateMutation.mutateAsync({
        id: record.id,
        data: { auto_enqueue_on_ready: enabled },
      });
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Couldn't change auto-enqueue");
    }
  };

  const handleClearSetup = async () => {
    try {
      await operator.handleDeleteProfile();
      setOverlay("setup");
    } catch {
      // The mutation helper reports the failure; keep the confirmation open.
    }
  };

  const copyTaskId = () => {
    if (!record) return;
    void copyTaskRecordId(record.id, "Task");
  };

  return {
    backToTasks,
    command,
    copyTaskId,
    deleteMutation,
    deleteOpen: overlay === "delete",
    detail,
    fanOutOpen: overlay === "fan_out",
    handleDeleteTask,
    handleAutoEnqueueChange,
    handleClearSetup,
    handlePriorityChange,
    inspectOpen,
    latestEventSeq,
    openEdit,
    openRun,
    openTask,
    operator,
    page,
    pauseDialog,
    record,
    runDurations,
    scrollToResult,
    search,
    setDeleteOpen: (open: boolean) => setOverlay(open ? "delete" : null),
    setFanOutOpen: (open: boolean) => setOverlay(open ? "fan_out" : null),
    setInspectOpen,
    setInspectTarget,
    setSetupOpen: (open: boolean) => setOverlay(open ? "setup" : null),
    setSetupClearOpen: (open: boolean) => setOverlay(open ? "setup_clear" : "setup"),
    setTab,
    setupClearOpen: overlay === "setup_clear",
    setupEditor,
    setupOpen: overlay === "setup" || overlay === "setup_clear",
    setupRuntime,
    showFanOut,
    taskId,
    updatePending: updateMutation.isPending,
  };
}

export type TaskDetailLocationController = ReturnType<typeof useTaskDetailLocation>;
