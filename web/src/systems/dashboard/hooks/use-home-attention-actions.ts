import { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";

import { useApproveTask, useRejectTask, useRetryTaskRun } from "@/systems/tasks";

import { dashboardKeys } from "../lib/query-keys";

export type HomeAttentionResolvedKind = "approved" | "rejected";

export interface HomeAttentionActions {
  resolvedById: Record<string, HomeAttentionResolvedKind>;
  /** Every task_id/run_id with a mutation in flight; each row stays disabled until it settles. */
  pendingIds: ReadonlySet<string>;
  onApprove: (taskId: string) => void;
  onReject: (taskId: string) => void;
  onRetry: (runId: string) => void;
}

/**
 * Owns Needs-you mutations and reconciles overview counters after success.
 * Per-id reference counts keep concurrent rows disabled until their own promise settles.
 */
export function useHomeAttentionActions(): HomeAttentionActions {
  const queryClient = useQueryClient();
  const approveTask = useApproveTask();
  const rejectTask = useRejectTask();
  const retryTaskRun = useRetryTaskRun();
  const [resolvedById, setResolvedById] = useState<Record<string, HomeAttentionResolvedKind>>({});
  const [pendingCounts, setPendingCounts] = useState<Record<string, number>>({});

  const startPending = (id: string) =>
    setPendingCounts(current => ({ ...current, [id]: (current[id] ?? 0) + 1 }));
  const endPending = (id: string) =>
    setPendingCounts(current => {
      const next = { ...current };
      const count = (next[id] ?? 0) - 1;
      if (count > 0) {
        next[id] = count;
      } else {
        delete next[id];
      }
      return next;
    });

  const refreshOverview = () => {
    void queryClient.invalidateQueries({ queryKey: dashboardKeys.overviewRoot() }).catch(error => {
      toast.error(error instanceof Error ? error.message : "Failed to refresh dashboard");
    });
  };

  const resolveRow = (taskId: string, kind: HomeAttentionResolvedKind) => {
    setResolvedById(current => ({ ...current, [taskId]: kind }));
    refreshOverview();
  };

  const onApprove = (taskId: string) => {
    startPending(taskId);
    void approveTask
      .mutateAsync({ id: taskId })
      .then(() => resolveRow(taskId, "approved"))
      .catch(error => {
        toast.error(error instanceof Error ? error.message : "Failed to approve task");
      })
      .finally(() => endPending(taskId));
  };

  const onReject = (taskId: string) => {
    startPending(taskId);
    void rejectTask
      .mutateAsync({ id: taskId })
      .then(() => resolveRow(taskId, "rejected"))
      .catch(error => {
        toast.error(error instanceof Error ? error.message : "Failed to reject task");
      })
      .finally(() => endPending(taskId));
  };

  const onRetry = (runId: string) => {
    startPending(runId);
    void retryTaskRun
      .mutateAsync({ runId })
      .then(() => refreshOverview())
      .catch(error => {
        toast.error(error instanceof Error ? error.message : "Failed to retry run");
      })
      .finally(() => endPending(runId));
  };

  const pendingIds: ReadonlySet<string> = new Set(Object.keys(pendingCounts));
  return { resolvedById, pendingIds, onApprove, onReject, onRetry };
}
