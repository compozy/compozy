import type { QueryClient } from "@tanstack/react-query";

import { settingsKeys } from "../lib/query-keys";
import { settingsRestartStore } from "../stores/settings-restart-store";
import type { SettingsMutationResult } from "../types";

export function recordSettingsMutation(result: SettingsMutationResult) {
  settingsRestartStore.trigger.settingsMutationRecorded({
    mutation: {
      section: result.section,
      restartRequired: Boolean(result.restart_required),
      restartScope: result.restart_scope,
      warnings: result.warnings ?? [],
      lifecycle: result.lifecycle,
      nextAction: result.next_action,
      applyRecordId: result.apply_record_id,
      activeGeneration: result.active_generation,
      completedAt: new Date().toISOString(),
    },
  });
}

export function invalidateSettingsApplyRecords(queryClient: QueryClient) {
  return queryClient.invalidateQueries({ queryKey: settingsKeys.applyRoot() });
}
