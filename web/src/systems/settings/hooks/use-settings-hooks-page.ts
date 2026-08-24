import { useRef, useState } from "react";

import { useSettingsPage } from "./use-settings-page";

import {
  type CreateNotificationPresetRequest,
  type NotificationPresetEntry,
  useCreateNotificationPreset,
  useDeleteNotificationPreset,
  useNotificationPresets,
  useSetNotificationPresetEnablement,
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

interface PendingProfileItem {
  profile: string;
  name: string;
  requestId: number;
  workspaceId: string | null;
}

export function useSettingsHooksPage() {
  const { destination, destinationOwner } = useProfileReadScope();
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
  const presetsQuery = useNotificationPresets({ profile: destination });
  const createPreset = useCreateNotificationPreset();
  const setPresetEnablement = useSetNotificationPresetEnablement();
  const deletePreset = useDeleteNotificationPreset();
  const page = useSettingsPage({ currentSlug: "hooks" });
  const [pendingHook, setPendingHook] = useState<PendingProfileItem | null>(null);
  const [pendingPreset, setPendingPreset] = useState<PendingProfileItem | null>(null);
  const nextPendingRequestId = useRef(0);
  const pendingWorkspaceId = filter.scope === "profile" ? (filter.workspace_id ?? null) : null;

  const pendingItem = (name: string): PendingProfileItem => ({
    name,
    profile: destination,
    requestId: ++nextPendingRequestId.current,
    workspaceId: pendingWorkspaceId,
  });
  const isPendingInCurrentScope = (pending: PendingProfileItem | null) =>
    pending?.profile === destination && pending.workspaceId === pendingWorkspaceId;

  const hooks: SettingsHookEntry[] = query.data?.hooks ?? [];
  const toggleHookEnabled = (entry: SettingsHookEntry, enabled: boolean) => {
    const pending = pendingItem(entry.name);
    setPendingHook(pending);
    const declaration: SettingsHookRequest["declaration"] = {
      ...entry.declaration,
      enabled,
    };
    hookMutation.mutate(
      { name: entry.name, body: { declaration }, filter },
      {
        onSettled: () =>
          setPendingHook(current => (current?.requestId === pending.requestId ? null : current)),
      }
    );
  };
  const createNotificationPreset = (body: CreateNotificationPresetRequest) => {
    const name = body.name ?? null;
    const pending = name === null ? null : pendingItem(name);
    setPendingPreset(pending);
    createPreset.mutate(
      { body, profile: destination },
      {
        onSettled: () =>
          setPendingPreset(current =>
            pending !== null && current?.requestId === pending.requestId ? null : current
          ),
      }
    );
  };
  const toggleNotificationPreset = (preset: NotificationPresetEntry, enabled: boolean) => {
    const pending = pendingItem(preset.name);
    setPendingPreset(pending);
    setPresetEnablement.mutate(
      { name: preset.name, body: { profile: destination, enabled } },
      {
        onSettled: () =>
          setPendingPreset(current => (current?.requestId === pending.requestId ? null : current)),
      }
    );
  };
  const deleteNotificationPreset = (preset: NotificationPresetEntry) => {
    const pending = pendingItem(preset.name);
    setPendingPreset(pending);
    deletePreset.mutate(preset.name, {
      onSettled: () =>
        setPendingPreset(current => (current?.requestId === pending.requestId ? null : current)),
    });
  };
  const mutationError =
    errorMessage(createPreset.error) ??
    errorMessage(setPresetEnablement.error) ??
    errorMessage(deletePreset.error);
  return {
    canMutateHooks: capabilityQuery.data?.transport_parity?.settings_http !== false,
    createNotificationPreset,
    deleteNotificationPreset,
    envelope: query.data ?? null,
    error: query.error ?? capabilityQuery.error ?? presetsQuery.error,
    handleRetry: () =>
      void Promise.all([query.refetch(), capabilityQuery.refetch(), presetsQuery.refetch()]),
    hookError: errorMessage(hookMutation.error),
    hooks,
    hooksCounts: {
      enabled: hooks.filter(entry => entry.declaration.enabled !== false).length,
      total: hooks.length,
    },
    isLoading: query.isLoading || capabilityQuery.isLoading || presetsQuery.isLoading,
    notificationPresetActionError: mutationError,
    notificationPresets: presetsQuery.data?.presets ?? [],
    notificationPresetsError: errorMessage(presetsQuery.error),
    notificationPresetsLoading: presetsQuery.isLoading,
    pendingHookName: pendingHook && isPendingInCurrentScope(pendingHook) ? pendingHook.name : null,
    pendingNotificationPresetName:
      pendingPreset && isPendingInCurrentScope(pendingPreset) ? pendingPreset.name : null,
    notificationPresetProfile: destinationOwner,
    restart: page.restart,
    toggleHookEnabled,
    toggleNotificationPreset,
  };
}
