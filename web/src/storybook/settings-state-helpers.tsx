import { useEffect } from "react";
import { useQueryClient } from "@tanstack/react-query";

import { settingsRestartStore } from "@/systems/settings/stores/settings-restart-store";
import { settingsKeys } from "@/systems/settings/lib/query-keys";

import type {
  SettingsRestartStatus,
  SettingsRestartStatusName,
  SettingsSectionName,
  SettingsUpdateStatus,
} from "@/systems/settings";

type RestartOverrides = {
  operationId?: string | null;
  status?: SettingsRestartStatusName | null;
  activeSessionCount?: number;
  failureReason?: string;
  mutationRestartRequired?: boolean;
};

/**
 * Seeds the settings restart store with a specific phase. Useful for story
 * states that need a deterministic banner tone without relying on the real
 * trigger + poll cycle.
 */
export function StorybookRestartPhaseSetup({
  section,
  overrides,
}: {
  section: SettingsSectionName;
  overrides: RestartOverrides;
}) {
  const queryClient = useQueryClient();
  const {
    activeSessionCount = 0,
    failureReason,
    mutationRestartRequired = false,
    operationId,
    status,
  } = overrides;

  useEffect(() => {
    settingsRestartStore.trigger.settingsMutationRecorded({
      mutation: mutationRestartRequired
        ? {
            section,
            restartRequired: true,
            restartScope: "global",
            warnings: [],
            completedAt: "2026-04-18T01:00:00Z",
          }
        : null,
    });
    if (operationId && status) {
      settingsRestartStore.trigger.restartOperationStarted({ operationId });
      const timestamp = "2026-04-18T01:00:00Z";
      const snapshot: SettingsRestartStatus = {
        active_session_count: activeSessionCount,
        failure_reason: failureReason,
        old_pid: 1000,
        old_socket_path: "/tmp/agh-story.sock",
        old_started_at: timestamp,
        operation_id: operationId,
        started_at: timestamp,
        status,
        updated_at: timestamp,
      };
      queryClient.setQueryData(settingsKeys.restartStatus(operationId), snapshot);
    } else {
      settingsRestartStore.trigger.restartOperationCleared();
    }
  }, [
    activeSessionCount,
    failureReason,
    mutationRestartRequired,
    operationId,
    queryClient,
    section,
    status,
  ]);

  return null;
}

/** Keeps a last-known update snapshot visible while the refresh query fails. */
export function StorybookSettingsUpdateRefreshErrorSetup({
  snapshot,
}: {
  snapshot: SettingsUpdateStatus;
}) {
  const queryClient = useQueryClient();

  useEffect(() => {
    queryClient.setQueryData(settingsKeys.updateStatus(), snapshot);
    const refetch = window.setTimeout(() => {
      void queryClient
        .refetchQueries({ exact: true, queryKey: settingsKeys.updateStatus() })
        .catch(() => undefined);
    }, 250);
    return () => window.clearTimeout(refetch);
  }, [queryClient, snapshot]);

  return null;
}

/**
 * Dirties the general settings draft by programmatically changing the daemon
 * memory-report interval. Uses RAF polling because the route tree mounts after
 * the story render fragment.
 */
export function StorybookGeneralDraftDirtySetup() {
  useEffect(() => {
    let cancelled = false;
    const setValue = (element: HTMLInputElement, next: string) => {
      const setter = Object.getOwnPropertyDescriptor(
        window.HTMLInputElement.prototype,
        "value"
      )?.set;
      setter?.call(element, next);
      element.dispatchEvent(new Event("input", { bubbles: true }));
      element.dispatchEvent(new Event("change", { bubbles: true }));
    };
    const tryDirty = () => {
      if (cancelled) return;
      const input = document.querySelector<HTMLInputElement>(
        '[data-testid="settings-page-general-memory-report-interval-input"]'
      );
      if (input) {
        setValue(input, "10m");
        return;
      }
      requestAnimationFrame(tryDirty);
    };
    requestAnimationFrame(tryDirty);
    return () => {
      cancelled = true;
    };
  }, []);
  return null;
}

/**
 * Programmatically dirties a settings field by typing `value` into the
 * input matching `testId`. Generic version of `StorybookGeneralDraftDirtySetup`
 * , reusable across every settings sub-route's dirty story state.
 */
export function StorybookFieldDirtySetup({ testId, value }: { testId: string; value: string }) {
  useEffect(() => {
    let cancelled = false;
    const setValue = (element: HTMLInputElement, next: string) => {
      const setter = Object.getOwnPropertyDescriptor(
        window.HTMLInputElement.prototype,
        "value"
      )?.set;
      setter?.call(element, next);
      element.dispatchEvent(new Event("input", { bubbles: true }));
      element.dispatchEvent(new Event("change", { bubbles: true }));
    };
    const tryDirty = () => {
      if (cancelled) return;
      const input = document.querySelector<HTMLInputElement>(`[data-testid="${testId}"]`);
      if (input) {
        setValue(input, value);
        return;
      }
      requestAnimationFrame(tryDirty);
    };
    requestAnimationFrame(tryDirty);
    return () => {
      cancelled = true;
    };
  }, [testId, value]);
  return null;
}

/**
 * Dirties the draft and clicks Save, leaving the PATCH request suspended so
 * the save-bar reads `isSaving=true` for the story.
 */
export function StorybookGeneralSavingSetup() {
  useEffect(() => {
    let cancelled = false;
    let stage: "dirty" | "save" = "dirty";
    const setValue = (element: HTMLInputElement, next: string) => {
      const setter = Object.getOwnPropertyDescriptor(
        window.HTMLInputElement.prototype,
        "value"
      )?.set;
      setter?.call(element, next);
      element.dispatchEvent(new Event("input", { bubbles: true }));
      element.dispatchEvent(new Event("change", { bubbles: true }));
    };

    const advance = () => {
      if (cancelled) return;
      if (stage === "dirty") {
        const input = document.querySelector<HTMLInputElement>(
          '[data-testid="settings-page-general-default-agent-input"]'
        );
        if (input) {
          setValue(input, "dirty-agent");
          stage = "save";
        }
      } else if (stage === "save") {
        const save = document.querySelector<HTMLButtonElement>(
          '[data-testid="settings-page-general-save"]'
        );
        if (save && !save.disabled) {
          save.click();
          return;
        }
      }
      requestAnimationFrame(advance);
    };
    requestAnimationFrame(advance);
    return () => {
      cancelled = true;
    };
  }, []);
  return null;
}
