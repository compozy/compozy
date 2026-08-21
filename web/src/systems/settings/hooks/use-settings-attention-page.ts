import { useRef, useState } from "react";

import {
  requestSystemNotifications,
  systemNotificationState,
  type SystemNotificationState,
} from "@/systems/os";
import { useProfileReadScope } from "@/systems/profiles";

import { SettingsApiError } from "../adapters/settings-api";
import { settingsAttentionFilterForProfile } from "../lib/query-options";
import type { SettingsAttentionSection, SettingsUpdateAttentionRequest } from "../types";
import { useSettingsPage } from "./use-settings-page";
import { useSettingsAttention } from "./use-settings-sections";
import { useUpdateSettingsAttention } from "./use-settings-mutations";

type AttentionConfig = SettingsAttentionSection["config"];

export interface SettingsAttentionPageModel {
  config: AttentionConfig | null;
  systemState: SystemNotificationState;
  isLoading: boolean;
  isSaving: boolean;
  error: Error | null;
  saveError: string | null;
  restart: ReturnType<typeof useSettingsPage>["restart"];
  setToasts: (enabled: boolean) => void;
  setSound: (enabled: boolean) => void;
  setSystem: (enabled: boolean) => void;
  unmuteWorkspace: (workspaceId: string) => void;
  muteWorkspace: (workspaceId: string) => void;
  handleRetry: () => void;
}

/**
 * Attention settings apply live, so this page has no save bar: every control is
 * the commit. Delivery channels are global config; workspace mutes are rows
 * owned by the active profile. The typed Settings route returns both as one
 * candidate, so a pending "Save" would create a second source of truth.
 *
 * Enabling the system channel walks the platform permission flow first and
 * writes the setting only if permission was actually granted — the toggle must
 * never claim to be armed when the platform refused.
 */
export function useSettingsAttentionPage(): SettingsAttentionPageModel {
  const profile = useProfileReadScope();
  const filter = settingsAttentionFilterForProfile(profile.destination);
  const query = useSettingsAttention(filter);
  const mutation = useUpdateSettingsAttention();
  const page = useSettingsPage({ currentSlug: "attention" });
  const [systemState, setSystemState] = useState<SystemNotificationState>(() =>
    systemNotificationState()
  );
  const [requestingSystemPermission, setRequestingSystemPermission] = useState(false);
  const permissionRequestRef = useRef(false);
  const pendingFilter = mutation.variables?.filter;
  const pendingConfig =
    mutation.isPending &&
    pendingFilter?.scope === filter.scope &&
    (pendingFilter?.profile ?? "") === (filter.profile ?? "")
      ? mutation.variables?.body.config
      : undefined;
  const config: AttentionConfig | null = pendingConfig
    ? {
        toasts: pendingConfig.toasts,
        sound: pendingConfig.sound,
        system: pendingConfig.system,
        muted_workspaces:
          pendingConfig.muted_workspaces ?? query.data?.config.muted_workspaces ?? [],
      }
    : (query.data?.config ?? null);
  const isSaving = mutation.isPending || requestingSystemPermission;

  const apply = (next: AttentionConfig) => {
    if (isSaving) return;
    const body: SettingsUpdateAttentionRequest = { config: next };
    mutation.mutate({ body, filter });
  };

  const setSystem = (enabled: boolean) => {
    if (config === null || isSaving) return;
    if (!enabled) {
      setSystemState(systemNotificationState());
      apply({ ...config, system: false });
      return;
    }
    if (permissionRequestRef.current) return;
    permissionRequestRef.current = true;
    setRequestingSystemPermission(true);
    void requestSystemNotifications()
      .then(state => {
        setSystemState(state);
        // A refused permission leaves the setting off; the chip explains why.
        if (state === "granted") {
          mutation.mutate({ body: { config: { ...config, system: true } }, filter });
        }
      })
      .finally(() => {
        permissionRequestRef.current = false;
        setRequestingSystemPermission(false);
      });
  };

  return {
    config,
    systemState,
    isLoading: query.isLoading,
    isSaving,
    error: query.error,
    saveError:
      mutation.error instanceof SettingsApiError
        ? mutation.error.message
        : mutation.error instanceof Error
          ? mutation.error.message
          : null,
    restart: page.restart,
    setToasts: enabled => config && apply({ ...config, toasts: enabled }),
    setSound: enabled => config && apply({ ...config, sound: enabled }),
    setSystem,
    muteWorkspace: workspaceId =>
      config &&
      apply({
        ...config,
        muted_workspaces: [...new Set([...config.muted_workspaces, workspaceId])],
      }),
    unmuteWorkspace: workspaceId =>
      config &&
      apply({
        ...config,
        muted_workspaces: config.muted_workspaces.filter(entry => entry !== workspaceId),
      }),
    handleRetry: () => void query.refetch(),
  };
}
