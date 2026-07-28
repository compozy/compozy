import { useState } from "react";

import {
  useCreateTaskBridgeNotificationSubscription,
  useDeleteTaskBridgeNotificationSubscription,
  useTaskBridgeNotificationSubscriptions,
} from "./use-task-notifications";
import { useDeleteTaskExecutionProfile, useSetTaskExecutionProfile } from "./use-task-profile";
import { useTaskStream } from "./use-task-stream";
import { submitTaskMutation } from "../lib/task-mutation";
import type { TaskStreamState } from "../lib/task-stream-state";
import type {
  TaskBridgeNotificationSubscriptionCreateRequest,
  TaskExecutionProfileSetRequest,
} from "../types";

interface UseTaskOperatorLayerOptions {
  /** Gate the bridge-subscription query + status stream to the open drawer. */
  enabled?: boolean;
  latestEventSeq?: number | null;
}

/**
 * Operator-layer data for the Inspect drawer and Setup sheet:
 * bridge subscriptions, execution-profile writes, and a status-bearing SSE
 * probe whose connection state feeds the drawer's Stream pane.
 *
 * @see docs/design/opendesign/tasks/TASK-DETAILS-REDESIGN-PLAN.md §4.7–4.8
 */
function useTaskOperatorLayer(taskId: string, options: UseTaskOperatorLayerOptions = {}) {
  const enabled = options.enabled ?? true;
  const hasTaskId = taskId.trim() !== "";
  const hasLatestEventSeq =
    typeof options.latestEventSeq === "number" && Number.isFinite(options.latestEventSeq);
  const seedSequence = hasLatestEventSeq ? Math.max(0, options.latestEventSeq ?? 0) : 0;
  const layerEnabled = enabled && hasTaskId;

  const subscriptionsQuery = useTaskBridgeNotificationSubscriptions(
    taskId,
    {},
    { enabled: layerEnabled }
  );

  const setProfileMutation = useSetTaskExecutionProfile();
  const deleteProfileMutation = useDeleteTaskExecutionProfile();
  const createSubscriptionMutation = useCreateTaskBridgeNotificationSubscription();
  const deleteSubscriptionMutation = useDeleteTaskBridgeNotificationSubscription();

  const streamKey = layerEnabled ? `${taskId}:${seedSequence}` : "disabled";
  const [streamStatus, setStreamStatus] = useState<{
    error: string | null;
    key: string;
    state: TaskStreamState;
  }>(() => ({ error: null, key: streamKey, state: layerEnabled ? "idle" : "disabled" }));
  const currentStreamStatus =
    streamStatus.key === streamKey
      ? streamStatus
      : {
          error: null,
          key: streamKey,
          state: layerEnabled ? ("idle" as const) : ("disabled" as const),
        };

  useTaskStream(taskId, {
    enabled: layerEnabled && hasLatestEventSeq,
    afterSequence: seedSequence,
    onEvent: () => setStreamStatus({ error: null, key: streamKey, state: "receiving" }),
    onError: error =>
      setStreamStatus({
        error:
          error instanceof Error
            ? error.message
            : typeof error === "string"
              ? error
              : "Stream connection failed",
        key: streamKey,
        state: "error",
      }),
  });

  const handleSetProfile = async (data: TaskExecutionProfileSetRequest) => {
    await submitTaskMutation(
      () => setProfileMutation.mutateAsync({ id: taskId, data }),
      "Setup saved.",
      "Failed to save setup"
    );
  };

  const handleDeleteProfile = async () => {
    await submitTaskMutation(
      () => deleteProfileMutation.mutateAsync({ id: taskId }),
      "Setup cleared.",
      "Failed to clear setup"
    );
  };

  const handleCreateSubscription = async (
    request: TaskBridgeNotificationSubscriptionCreateRequest
  ) => {
    await submitTaskMutation(
      () => createSubscriptionMutation.mutateAsync({ taskId, data: request }),
      "Subscription added.",
      "Failed to add subscription"
    );
  };

  const handleDeleteSubscription = async (subscriptionId: string) => {
    await submitTaskMutation(
      () => deleteSubscriptionMutation.mutateAsync({ taskId, subscriptionId }),
      "Subscription removed.",
      "Failed to remove subscription"
    );
  };

  return {
    handleCreateSubscription,
    handleDeleteProfile,
    handleDeleteSubscription,
    handleSetProfile,
    isCreateSubscriptionPending: createSubscriptionMutation.isPending,
    isDeleteProfilePending: deleteProfileMutation.isPending,
    isDeleteSubscriptionPending: deleteSubscriptionMutation.isPending,
    isSetProfilePending: setProfileMutation.isPending,
    streamErrorMessage: currentStreamStatus.error,
    streamSeedSequence: seedSequence,
    streamState: currentStreamStatus.state,
    subscriptions: subscriptionsQuery.data ?? [],
    subscriptionsError: subscriptionsQuery.error ?? null,
    subscriptionsLoading: subscriptionsQuery.isLoading && !subscriptionsQuery.data,
  };
}

export { useTaskOperatorLayer };
export type { UseTaskOperatorLayerOptions };
