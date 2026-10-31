import { toast } from "@compozy/ui";

import { useTaskDashboard } from "./use-task-dashboard";
import { schedulerBacklogScope, type ActiveTaskScopeFilter } from "../lib/workspace-scope";
import {
  useDrainScheduler,
  usePauseScheduler,
  useResumeScheduler,
  useSchedulerBacklog,
  useSchedulerStatus,
} from "@/systems/scheduler";

export function useTasksDashboardPage(scope: ActiveTaskScopeFilter, enabled: boolean) {
  const dashboardQuery = useTaskDashboard(scope, { enabled });
  const schedulerStatusQuery = useSchedulerStatus({ enabled });
  const schedulerBacklogFilters = {
    ...schedulerBacklogScope(scope),
    limit: 5,
    include_paused: true,
  };
  const schedulerBacklogQuery = useSchedulerBacklog(schedulerBacklogFilters, { enabled });
  const pauseMutation = usePauseScheduler();
  const resumeMutation = useResumeScheduler();
  const drainMutation = useDrainScheduler();

  const handlePauseScheduler = async (reason: string) => {
    try {
      const normalizedReason = reason.trim();
      await pauseMutation.mutateAsync(normalizedReason ? { reason: normalizedReason } : {});
      toast.success("Scheduler paused.");
    } catch (error) {
      console.error("Failed to pause the scheduler", error);
      toast.error("Couldn't drain the queue.");
      throw error;
    }
  };
  const handleResumeScheduler = async (reason?: string) => {
    try {
      const normalizedReason = reason?.trim();
      await resumeMutation.mutateAsync(normalizedReason ? { reason: normalizedReason } : {});
      toast.success("Scheduler resumed.");
    } catch (error) {
      console.error("Failed to resume the scheduler", error);
      toast.error("Couldn't resume the queue.");
      throw error;
    }
  };
  const handleDrainScheduler = async ({
    reason,
    timeoutSeconds,
  }: {
    reason?: string;
    timeoutSeconds?: number;
  }) => {
    try {
      const normalizedReason = reason?.trim();
      await drainMutation.mutateAsync({
        ...(normalizedReason ? { reason: normalizedReason } : {}),
        timeout_seconds: timeoutSeconds ?? 60,
      });
      toast.success("Scheduler drain requested.");
    } catch (error) {
      console.error("Failed to drain the scheduler", error);
      toast.error("Couldn't pause the queue.");
      throw error;
    }
  };

  return {
    dashboard: dashboardQuery.data ?? null,
    dashboardError: dashboardQuery.error ?? null,
    dashboardLoading: dashboardQuery.isPending && !dashboardQuery.data,
    handleDrainScheduler,
    handlePauseScheduler,
    handleResumeScheduler,
    isSchedulerDrainPending: drainMutation.isPending,
    isSchedulerPausePending: pauseMutation.isPending,
    isSchedulerResumePending: resumeMutation.isPending,
    schedulerBacklog: schedulerBacklogQuery.data ?? null,
    schedulerBacklogError: schedulerBacklogQuery.error ?? null,
    schedulerBacklogLoading: schedulerBacklogQuery.isPending && !schedulerBacklogQuery.data,
    schedulerStatus: schedulerStatusQuery.data ?? null,
    schedulerStatusError: schedulerStatusQuery.error ?? null,
    schedulerStatusLoading: schedulerStatusQuery.isPending && !schedulerStatusQuery.data,
  };
}
