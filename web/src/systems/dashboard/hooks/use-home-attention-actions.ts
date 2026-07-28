import { useSelector, useStore } from "@xstate/store-react";
import { useQueryClient } from "@tanstack/react-query";

import { useApproveTask, useRejectTask, useRetryTaskRun } from "@/systems/tasks";

import {
  createHomeAttentionActionsLogic,
  type HomeAttentionResolvedKind,
} from "./home-attention-actions-store";
import { dashboardKeys } from "../lib/query-keys";

export type { HomeAttentionResolvedKind } from "./home-attention-actions-store";

const homeAttentionActionsLogic = createHomeAttentionActionsLogic();

export interface HomeAttentionActions {
  resolvedById: Record<string, HomeAttentionResolvedKind>;
  /** Every task_id/run_id with a mutation in flight; each row stays disabled until it settles. */
  pendingIds: ReadonlySet<string>;
  onApprove: (taskId: string) => void;
  onReject: (taskId: string) => void;
  onRetry: (runId: string) => void;
}

/** Owns Needs-you mutations and reconciles overview counters after success. */
export function useHomeAttentionActions(): HomeAttentionActions {
  const queryClient = useQueryClient();
  const approveTask = useApproveTask();
  const rejectTask = useRejectTask();
  const retryTaskRun = useRetryTaskRun();
  const store = useStore(homeAttentionActionsLogic);
  const operations = useSelector(store, snapshot => snapshot.context.operations);

  const pendingIds = new Set<string>();
  const resolvedById: Record<string, HomeAttentionResolvedKind> = {};
  for (const [id, operation] of Object.entries(operations)) {
    if (operation.status === "pending") pendingIds.add(id);
    if (operation.status === "resolved") resolvedById[id] = operation.resolved;
  }

  const refreshOverview = () =>
    queryClient.invalidateQueries({ queryKey: dashboardKeys.overviewRoot() });

  return {
    resolvedById,
    pendingIds,
    onApprove: taskId =>
      store.trigger.approveRequested({
        id: taskId,
        execute: id => approveTask.mutateAsync({ id }),
        refreshOverview,
      }),
    onReject: taskId =>
      store.trigger.rejectRequested({
        id: taskId,
        execute: id => rejectTask.mutateAsync({ id }),
        refreshOverview,
      }),
    onRetry: runId =>
      store.trigger.retryRequested({
        id: runId,
        execute: id => retryTaskRun.mutateAsync({ runId: id }),
        refreshOverview,
      }),
  };
}
