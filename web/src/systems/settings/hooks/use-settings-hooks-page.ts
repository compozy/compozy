import { useState } from "react";

import { useSettingsPage } from "./use-settings-page";

import {
  type CreateNotificationPresetRequest,
  type NotificationPresetEntry,
  useCreateNotificationPreset,
  useDeleteNotificationPreset,
  useNotificationPresets,
  useUpdateNotificationPreset,
} from "@/systems/notifications";
import {
  SettingsApiError,
  type SettingsHookEntry,
  type SettingsHookRequest,
  usePutSettingsHook,
  useSettingsHooks,
  useSettingsHooksExtensions,
} from "@/systems/settings";
import { useProfileReadScope } from "@/systems/profiles";
import { useActiveWorkspace } from "@/systems/workspace";

function errorMessage(error: unknown): string | null {
  if (error instanceof SettingsApiError || error instanceof Error) return error.message;
  return null;
}

export function useSettingsHooksPage() {
  const { destination } = useProfileReadScope();
  const { activeWorkspaceId } = useActiveWorkspace();
  const filter =
    destination === "default"
      ? ({ scope: "user" } as const)
      : ({
          scope: "profile",
          profile: destination,
          workspace_id: activeWorkspaceId ?? undefined,
        } as const);
  const query = useSettingsHooks(filter);
  const capabilityQuery = useSettingsHooksExtensions();
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
      { name: entry.name, body: { declaration }, filter },
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
    canMutateHooks: capabilityQuery.data?.transport_parity?.settings_http !== false,
    createNotificationPreset,
    deleteNotificationPreset,
    envelope: query.data ?? null,
    error: query.error ?? capabilityQuery.error,
    handleRetry: () =>
      void Promise.all([query.refetch(), capabilityQuery.refetch(), presetsQuery.refetch()]),
    hookError: errorMessage(hookMutation.error),
    hooks,
    hooksCounts: {
      enabled: hooks.filter(entry => entry.declaration.enabled !== false).length,
      total: hooks.length,
    },
    isLoading: query.isLoading || capabilityQuery.isLoading,
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
