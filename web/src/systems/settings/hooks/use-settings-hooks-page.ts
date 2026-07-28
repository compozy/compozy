import { useState } from "react";

import { useSettingsPage } from "./use-settings-page";
import {
  SettingsApiError,
  usePutSettingsHook,
  useSettingsHooksExtensions,
  type SettingsHookEntry,
  type SettingsHookRequest,
} from "@/systems/settings";
import {
  useCreateNotificationPreset,
  useDeleteNotificationPreset,
  useNotificationPresets,
  useUpdateNotificationPreset,
  type CreateNotificationPresetRequest,
  type NotificationPresetEntry,
} from "@/systems/notifications";

function errorMessage(error: unknown): string | null {
  if (error instanceof SettingsApiError || error instanceof Error) return error.message;
  return null;
}

export function useSettingsHooksPage() {
  const query = useSettingsHooksExtensions();
  const hookMutation = usePutSettingsHook();
  const presetsQuery = useNotificationPresets();
  const createPreset = useCreateNotificationPreset();
  const updatePreset = useUpdateNotificationPreset();
  const deletePreset = useDeleteNotificationPreset();
  const page = useSettingsPage({ currentSlug: "hooks" });
  const [pendingHookName, setPendingHookName] = useState<string | null>(null);
  const [pendingPresetName, setPendingPresetName] = useState<string | null>(null);

  const hooks: SettingsHookEntry[] = query.data?.hooks ?? [];
  const toggleHookEnabled = (entry: SettingsHookEntry, enabled: boolean) => {
    setPendingHookName(entry.name);
    const declaration: SettingsHookRequest["declaration"] = {
      ...entry.declaration,
      enabled,
    };
    hookMutation.mutate(
      { name: entry.name, body: { declaration } },
      { onSettled: () => setPendingHookName(null) }
    );
  };
  const createNotificationPreset = (body: CreateNotificationPresetRequest) => {
    setPendingPresetName(body.name ?? null);
    createPreset.mutate(body, { onSettled: () => setPendingPresetName(null) });
  };
  const toggleNotificationPreset = (preset: NotificationPresetEntry, enabled: boolean) => {
    setPendingPresetName(preset.name);
    updatePreset.mutate(
      { name: preset.name, body: { enabled } },
      { onSettled: () => setPendingPresetName(null) }
    );
  };
  const deleteNotificationPreset = (preset: NotificationPresetEntry) => {
    setPendingPresetName(preset.name);
    deletePreset.mutate(preset.name, { onSettled: () => setPendingPresetName(null) });
  };
  const mutationError =
    errorMessage(createPreset.error) ??
    errorMessage(updatePreset.error) ??
    errorMessage(deletePreset.error);
  return {
    canMutateHooks: query.data?.transport_parity?.settings_http !== false,
    createNotificationPreset,
    deleteNotificationPreset,
    envelope: query.data ?? null,
    error: query.error,
    handleRetry: () => void Promise.all([query.refetch(), presetsQuery.refetch()]),
    hookError: errorMessage(hookMutation.error),
    hooks,
    hooksCounts: {
      enabled: hooks.filter(entry => entry.declaration.enabled !== false).length,
      total: hooks.length,
    },
    isLoading: query.isLoading,
    notificationPresetActionError: mutationError,
    notificationPresets: presetsQuery.data?.presets ?? [],
    notificationPresetsError: errorMessage(presetsQuery.error),
    notificationPresetsLoading: presetsQuery.isLoading,
    pendingHookName,
    pendingNotificationPresetName: pendingPresetName,
    restart: page.restart,
    toggleHookEnabled,
    toggleNotificationPreset,
  };
}
