import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";

import { statusOptions } from "@/systems/status";

import { triggerSettingsRestart } from "../adapters/settings-api";
import { settingsKeys } from "../lib/query-keys";
import { settingsApplyRecordsOptions, settingsRestartStatusOptions } from "../lib/query-options";
import { isTerminalRestartStatus } from "../lib/restart-status";
import { useSettingsRestartState } from "../stores/use-settings-restart-store";
import { settingsRestartStore } from "../stores/settings-restart-store";
import { invalidateSettingsApplyRecords } from "./settings-mutation-helpers";

export function useSettingsRestart() {
  const queryClient = useQueryClient();
  const operationId = useSettingsRestartState(state => state.operationId);
  const lastMutation = useSettingsRestartState(state => state.lastMutation);
  const mutationGeneration = useSettingsRestartState(state => state.mutationGeneration);
  const snoozedMutationGeneration = useSettingsRestartState(
    state => state.snoozedMutationGeneration
  );
  const snoozedApplyRecordId = useSettingsRestartState(state => state.snoozedApplyRecordId);
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
  const daemonStatusQuery = useQuery(statusOptions());
  const applyRecordsQuery = useQuery(settingsApplyRecordsOptions({ limit: 1 }));
  const resolvedOperationId = statusQuery.isError ? null : operationId;
  const triggerStatus =
    triggerMutation.data?.operation_id === operationId ? triggerMutation.data : null;
  const status = statusQuery.data?.status ?? triggerStatus?.status ?? null;
  const activeSessionCount =
    daemonStatusQuery.data?.daemon.active_sessions ??
    statusQuery.data?.active_session_count ??
    triggerStatus?.active_session_count ??
    0;
  const failureReason = statusQuery.data?.failure_reason;

  useEffect(() => {
    if (operationId !== null && isTerminalRestartStatus(status)) {
      void invalidateSettingsApplyRecords(queryClient);
    }
  }, [operationId, status, queryClient]);

  const isRestartRequired =
    daemonStatusQuery.data?.config.restart_required ?? Boolean(lastMutation?.restartRequired);
  const latestApplyRecordId = applyRecordsQuery.data?.entries[0]?.id;
  const isNoticeSnoozed =
    isRestartRequired &&
    (latestApplyRecordId
      ? snoozedApplyRecordId === latestApplyRecordId
      : snoozedMutationGeneration === mutationGeneration);

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
    dismiss: () =>
      settingsRestartStore.trigger.restartNoticeDismissed({ applyRecordId: latestApplyRecordId }),
    statusQueryError: statusQuery.error,
    statusQueryLoading: statusQuery.isLoading,
  };
}
