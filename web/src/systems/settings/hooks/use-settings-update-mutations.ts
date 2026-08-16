import { useMutation, useQueryClient } from "@tanstack/react-query";

import { applySettingsUpdate, cancelSettingsUpdate } from "../adapters/settings-api";
import { settingsKeys } from "../lib/query-keys";
import type { SettingsUpdateTarget } from "../types";

/**
 * Update-operation mutations (ADR-009). These are host-global lifecycle actions
 * over the durable Update Operation, not config-section writes, so they record no
 * pending restart and live apart from `use-settings-mutations`.
 *
 * Both endpoints answer with acknowledgement, never a terminal verdict: `accepted`
 * means the operation record was durably acquired, `blocked` names the holder that
 * owns it. Terminal truth — staged phases, rollback, failure — arrives only through
 * `GET /api/settings/update`, which is why every mutation invalidates that read
 * instead of writing an optimistic snapshot.
 */
export function useApplySettingsUpdate() {
  const queryClient = useQueryClient();

  return useMutation({
    // Typed on the closed target vocabulary rather than the generated `string`,
    // so callers can read `variables.target` back without an assertion.
    mutationFn: (body: { target: SettingsUpdateTarget }) => applySettingsUpdate(body),
    onSettled: () => queryClient.invalidateQueries({ queryKey: settingsKeys.updateStatus() }),
  });
}

export function useCancelSettingsUpdate() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: () => cancelSettingsUpdate(),
    onSettled: () => queryClient.invalidateQueries({ queryKey: settingsKeys.updateStatus() }),
  });
}
