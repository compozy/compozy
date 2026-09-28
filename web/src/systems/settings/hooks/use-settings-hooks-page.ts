import { useRef, useState } from "react";

import { useSettingsPage } from "./use-settings-page";

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
  const page = useSettingsPage({ currentSlug: "hooks" });
  const [pendingHook, setPendingHook] = useState<PendingProfileItem | null>(null);
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
  return {
    canMutateHooks: capabilityQuery.data?.transport_parity?.settings_http !== false,
    envelope: query.data ?? null,
    error: query.error ?? capabilityQuery.error,
    handleRetry: () => void Promise.all([query.refetch(), capabilityQuery.refetch()]),
    hookError: errorMessage(hookMutation.error),
    hooks,
    hooksCounts: {
      enabled: hooks.filter(entry => entry.declaration.enabled !== false).length,
      total: hooks.length,
    },
    isLoading: query.isLoading || capabilityQuery.isLoading,
    pendingHookName: pendingHook && isPendingInCurrentScope(pendingHook) ? pendingHook.name : null,
    restart: page.restart,
    toggleHookEnabled,
  };
}
