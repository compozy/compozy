import { useMutation, useQueryClient } from "@tanstack/react-query";

import {
  createSettingsNotificationPreset,
  deleteSettingsNotificationPreset,
  updateSettingsNotificationPreset,
} from "../adapters/settings-api";
import { settingsKeys } from "../lib/query-keys";
import type {
  SettingsCreateNotificationPresetRequest,
  SettingsNotificationPresetEntry,
  SettingsUpdateNotificationPresetRequest,
} from "../types";

interface SettingsNotificationPresetUpdateParams {
  name: string;
  body: SettingsUpdateNotificationPresetRequest;
}

function invalidateNotificationPresets(queryClient: ReturnType<typeof useQueryClient>) {
  return queryClient.invalidateQueries({ queryKey: settingsKeys.notificationsRoot() });
}

export function useCreateSettingsNotificationPreset() {
  const queryClient = useQueryClient();
  return useMutation<
    SettingsNotificationPresetEntry,
    Error,
    SettingsCreateNotificationPresetRequest
  >({
    mutationFn: body => createSettingsNotificationPreset(body),
    onSettled: () => invalidateNotificationPresets(queryClient),
  });
}

export function useUpdateSettingsNotificationPreset() {
  const queryClient = useQueryClient();
  return useMutation<
    SettingsNotificationPresetEntry,
    Error,
    SettingsNotificationPresetUpdateParams
  >({
    mutationFn: ({ name, body }) => updateSettingsNotificationPreset(name, body),
    onSettled: () => invalidateNotificationPresets(queryClient),
  });
}

export function useDeleteSettingsNotificationPreset() {
  const queryClient = useQueryClient();
  return useMutation<void, Error, string>({
    mutationFn: name => deleteSettingsNotificationPreset(name),
    onSettled: () => invalidateNotificationPresets(queryClient),
  });
}
