import { useEffect } from "react";

import { useSettingsRestartStore } from "@/systems/settings/stores/use-settings-restart-store";

import type { SettingsRestartStatusName, SettingsSectionName } from "@/systems/settings";

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
  const {
    activeSessionCount = 0,
    failureReason,
    mutationRestartRequired = false,
    operationId,
    status,
  } = overrides;

  useEffect(() => {
    const store = useSettingsRestartStore.getState();
    store.recordMutation(
      mutationRestartRequired
        ? {
            section,
            restartRequired: true,
            restartScope: "global",
            warnings: [],
            completedAt: "2026-04-18T01:00:00Z",
          }
        : null
    );
    if (operationId && status) {
      store.startRestart({
        operationId,
        status,
        activeSessionCount,
      });
      store.updateRestart({
        status,
        activeSessionCount,
        failureReason,
      });
    } else {
      store.clearRestart();
    }
  }, [activeSessionCount, failureReason, mutationRestartRequired, operationId, section, status]);

  return null;
}

/**
 * Dirties the general settings draft by programmatically typing into the
 * default-agent input. Uses RAF polling because the route tree mounts after
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
        '[data-testid="settings-page-general-default-agent-input"]'
      );
      if (input) {
        setValue(input, "dirty-agent");
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
