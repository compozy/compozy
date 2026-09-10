import { useSelector, useStore } from "@xstate/store-react";
import { useMutation, useQueryClient } from "@tanstack/react-query";

import {
  createHomeAttentionActionsLogic,
  type HomeAttentionResolvedKind,
} from "./home-attention-actions-store";
import {
  acknowledgeAttentionNotifications,
  notificationKeys,
  type AttentionNotificationScope,
} from "@/systems/notifications";
import { dashboardKeys } from "../lib/query-keys";
import { useApproveTask, useRejectTask, useRetryTaskRun } from "@/systems/tasks";

export type { HomeAttentionResolvedKind } from "./home-attention-actions-store";

const homeAttentionActionsLogic = createHomeAttentionActionsLogic();

export interface HomeAttentionActions {
  resolvedById: Record<string, HomeAttentionResolvedKind>;
  acknowledgementPending: boolean;
  acknowledgementError: string | null;
  onAcknowledge: (id?: string) => void;
  /** Every task_id/run_id with a mutation in flight; each row stays disabled until it settles. */
  pendingIds: ReadonlySet<string>;
  onApprove: (taskId: string) => void;
  onReject: (taskId: string) => void;
  onRetry: (runId: string) => void;
}

/** Owns Needs-you mutations and reconciles overview counters after success. */
export function useHomeAttentionActions(acknowledgement?: {
  snapshot?: string;
  scope?: AttentionNotificationScope;
}): HomeAttentionActions {
  const queryClient = useQueryClient();
  const acknowledgementMutation = useMutation({
    mutationFn: ({
      snapshot,
      scope,
      id,
    }: {
      snapshot: string;
      scope: AttentionNotificationScope;
      id?: string;
    }) => acknowledgeAttentionNotifications(scope, { snapshot, id }),
    onSettled: () =>
      Promise.all([
        queryClient.invalidateQueries({ queryKey: dashboardKeys.overviewRoot() }),
        queryClient.invalidateQueries({ queryKey: notificationKeys.attentionRoot() }),
      ]),
  });
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
    acknowledgementPending: acknowledgementMutation.isPending,
    acknowledgementError: acknowledgementMutation.error?.message ?? null,
    onAcknowledge: id => {
      if (!acknowledgement?.snapshot || !acknowledgement.scope || acknowledgementMutation.isPending)
        return;
      acknowledgementMutation.mutate({
        snapshot: acknowledgement.snapshot,
        scope: acknowledgement.scope,
        id,
      });
    },
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
