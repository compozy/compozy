import { useState, type SetStateAction } from "react";

import { useSettingsGeneralPage } from "@/systems/settings/hooks/use-settings-general-page";
import {
  SettingsPageFrame,
  SettingsPageState,
  SettingsSaveBar,
  TerminalSettingsSections,
  readTerminalSettings,
  terminalSettingsInvalidKeyMessage,
  useSettingsSaveBarState,
  useSettingsTopbar,
  type TerminalSettingsConfig,
  type TerminalSettingsKey,
} from "@/systems/settings";

export function TerminalSettingsPage() {
  const page = useSettingsGeneralPage();
  useSettingsTopbar();
  const [validationErrors, setValidationErrors] = useState<Record<string, string | null>>({});
  const setValidationError = (key: string) => (message: string | null) => {
    setValidationErrors(current =>
      current[key] === message ? current : { ...current, [key]: message }
    );
  };
  const isInvalid = Object.values(validationErrors).some(message => message !== null);
  const saveBarState = useSettingsSaveBarState({
    isDirty: page.isDirty,
    isInvalid,
    isSaving: page.isSaving,
    error: page.saveError,
    warnings: page.warnings,
    lastAppliedLabel: page.lastAppliedLabel,
  });

  if (page.isLoading) {
    return <SettingsPageState slug="terminal" state="loading" />;
  }

  if (page.error || !page.envelope || !page.draft) {
    return (
      <SettingsPageState
        error={page.error}
        onRetry={page.handleRetry}
        slug="terminal"
        state="error"
      />
    );
  }

  return (
    <TerminalSettingsLoaded
      draft={page.draft}
      isInvalid={isInvalid}
      restart={page.restart}
      saveBarState={saveBarState}
      setDraft={page.setDraft}
      setValidationError={setValidationError}
      validationErrors={validationErrors}
      onReset={page.handleReset}
      onSave={page.handleSave}
    />
  );
}

function TerminalSettingsLoaded({
  draft,
  isInvalid,
  restart,
  saveBarState,
  setDraft,
  setValidationError,
  validationErrors,
  onReset,
  onSave,
}: {
  draft: NonNullable<ReturnType<typeof useSettingsGeneralPage>["draft"]>;
  isInvalid: boolean;
  restart: ReturnType<typeof useSettingsGeneralPage>["restart"];
  saveBarState: ReturnType<typeof useSettingsSaveBarState>;
  setDraft: ReturnType<typeof useSettingsGeneralPage>["setDraft"];
  setValidationError: (key: string) => (message: string | null) => void;
  validationErrors: Record<string, string | null>;
  onReset: () => void;
  onSave: () => void;
}) {
  const projection = readTerminalSettings(draft);
  const mergedErrors = mergeTerminalSettingsErrors(validationErrors, projection.invalidKeys);
  const formInvalid = isInvalid || projection.invalidKeys.length > 0;
  const saveState = formInvalid ? { ...saveBarState, isInvalid: true } : saveBarState;

  return (
    <SettingsPageFrame
      description="Changes apply to the next open or attach."
      restart={restart}
      saveBar={
        <SettingsSaveBar slug="terminal" state={saveState} onSave={onSave} onReset={onReset} />
      }
      slug="terminal"
    >
      {projection.status === "ready" ? (
        <TerminalSettingsSections
          draft={projection.values}
          setDraft={update =>
            setDraft(previous => {
              const current = previous ?? draft;
              return mergeTerminalDraft(current, projection.values, update);
            })
          }
          onValidationError={(key, message) => setValidationError(key)(message)}
          validationErrors={mergedErrors}
        />
      ) : null}
    </SettingsPageFrame>
  );
}

function mergeTerminalSettingsErrors(
  validationErrors: Record<string, string | null>,
  invalidKeys: readonly TerminalSettingsKey[]
): Record<string, string | null> {
  const next = { ...validationErrors };
  for (const key of invalidKeys) {
    if (next[key] == null) next[key] = terminalSettingsInvalidKeyMessage(key);
  }
  return next;
}

function mergeTerminalDraft(
  current: NonNullable<ReturnType<typeof useSettingsGeneralPage>["draft"]>,
  viewed: Partial<TerminalSettingsConfig>,
  update: SetStateAction<Partial<TerminalSettingsConfig> | null>
): NonNullable<ReturnType<typeof useSettingsGeneralPage>["draft"]> {
  const next = typeof update === "function" ? update(viewed) : update;
  if (next === null) return current;
  const existing = current.terminal;
  if (existing == null || typeof existing !== "object") {
    return current;
  }
  return { ...current, terminal: { ...existing, ...next } };
}
