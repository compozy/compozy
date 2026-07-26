import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";

import { triggerSettingsRestart } from "../adapters/settings-api";
import { settingsKeys } from "../lib/query-keys";
import {
  isFailedRestart,
  isSuccessfulRestart,
  isTerminalRestartStatus,
} from "../lib/restart-status";
import { settingsRestartStatusOptions } from "../lib/query-options";
import { useSettingsRestartStore } from "../stores/use-settings-restart-store";

export function useSettingsRestart() {
  const queryClient = useQueryClient();
  const operationId = useSettingsRestartStore(state => state.operationId);
  const status = useSettingsRestartStore(state => state.status);
  const activeSessionCount = useSettingsRestartStore(state => state.activeSessionCount);
  const failureReason = useSettingsRestartStore(state => state.failureReason);
  const lastMutation = useSettingsRestartStore(state => state.lastMutation);
  const mutationGeneration = useSettingsRestartStore(state => state.mutationGeneration);
  const snoozedMutationGeneration = useSettingsRestartStore(
    state => state.snoozedMutationGeneration
  );
  const startRestart = useSettingsRestartStore(state => state.startRestart);
  const updateRestart = useSettingsRestartStore(state => state.updateRestart);
  const dismissRestartNotice = useSettingsRestartStore(state => state.dismissRestartNotice);

  const statusQuery = useQuery(
    settingsRestartStatusOptions(
      operationId,
      Boolean(operationId) && !isTerminalRestartStatus(status)
    )
  );

  useEffect(() => {
    if (!statusQuery.data) {
      return;
    }

    updateRestart({
      status: statusQuery.data.status,
      activeSessionCount: statusQuery.data.active_session_count,
      failureReason: statusQuery.data.failure_reason,
    });
  }, [statusQuery.data, updateRestart]);

  const triggerMutation = useMutation({
    mutationFn: () => triggerSettingsRestart(),
    onSuccess: response => {
      startRestart({
        operationId: response.operation_id,
        status: response.status,
        activeSessionCount: response.active_session_count,
      });
      void queryClient.invalidateQueries({
        queryKey: settingsKeys.restartStatus(response.operation_id),
      });
    },
  });

  const isPolling = Boolean(operationId) && !isTerminalRestartStatus(status);
  const isRestartRequired = Boolean(lastMutation?.restartRequired);
  const isNoticeSnoozed = isRestartRequired && snoozedMutationGeneration === mutationGeneration;
  const isSuccessful = isSuccessfulRestart(status);
  const isFailed = isFailedRestart(status);

  return {
    operationId,
    status,
    activeSessionCount,
    failureReason,
    lastMutation,
    trigger: triggerMutation.mutate,
    triggerAsync: triggerMutation.mutateAsync,
    isTriggerPending: triggerMutation.isPending,
    triggerError: triggerMutation.error,
    isPolling,
    isRestartRequired,
    isNoticeSnoozed,
    isSuccessful,
    isFailed,
    dismiss: dismissRestartNotice,
    statusQueryError: statusQuery.error,
    statusQueryLoading: statusQuery.isLoading,
  };
}
