import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { triggerSettingsRestart } from "../adapters/settings-api";
import { settingsKeys } from "../lib/query-keys";
import { settingsRestartStatusOptions } from "../lib/query-options";
import { useSettingsRestartState } from "../stores/use-settings-restart-store";
import { settingsRestartStore } from "../stores/settings-restart-store";

export function useSettingsRestart() {
  const queryClient = useQueryClient();
  const operationId = useSettingsRestartState(state => state.operationId);
  const lastMutation = useSettingsRestartState(state => state.lastMutation);
  const mutationGeneration = useSettingsRestartState(state => state.mutationGeneration);
  const snoozedMutationGeneration = useSettingsRestartState(
    state => state.snoozedMutationGeneration
  );
  const triggerMutation = useMutation({
    mutationFn: () => triggerSettingsRestart(),
    onSuccess: response => {
      settingsRestartStore.trigger.restartOperationStarted({
        operationId: response.operation_id,
      });
      void queryClient.invalidateQueries({
        queryKey: settingsKeys.restartStatus(response.operation_id),
      });
    },
  });

  const statusQuery = useQuery(settingsRestartStatusOptions(operationId));
  const resolvedOperationId = statusQuery.isError ? null : operationId;
  const triggerStatus =
    triggerMutation.data?.operation_id === operationId ? triggerMutation.data : null;
  const status = statusQuery.data?.status ?? triggerStatus?.status ?? null;
  const activeSessionCount =
    statusQuery.data?.active_session_count ?? triggerStatus?.active_session_count ?? 0;
  const failureReason = statusQuery.data?.failure_reason;

  const isRestartRequired = Boolean(lastMutation?.restartRequired);
  const isNoticeSnoozed = isRestartRequired && snoozedMutationGeneration === mutationGeneration;

  return {
    operationId: resolvedOperationId,
    status,
    activeSessionCount,
    failureReason,
    lastMutation,
    trigger: triggerMutation.mutate,
    triggerAsync: triggerMutation.mutateAsync,
    isTriggerPending: triggerMutation.isPending,
    triggerError: triggerMutation.error,
    isRestartRequired,
    isNoticeSnoozed,
    dismiss: () => settingsRestartStore.trigger.restartNoticeDismissed(),
    statusQueryError: statusQuery.error,
    statusQueryLoading: statusQuery.isLoading,
  };
}
