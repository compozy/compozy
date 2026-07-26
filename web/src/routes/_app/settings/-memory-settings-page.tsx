import { AlertCircle } from "lucide-react";
import { useState } from "react";

import { useSettingsMemoryPage } from "@/systems/settings/hooks/use-settings-memory-page";
import {
  SettingsAdvancedFold,
  SettingsPageFrame,
  SettingsSaveBar,
  SettingsRuntimeUnavailable,
  useSettingsSaveBarState,
  useSettingsTopbar,
} from "@/systems/settings";
import { Button, Spinner, Time } from "@agh/ui";
import { ControllerSection } from "./-memory-controller-sections";
import { DreamSection } from "./-memory-dream-section";
import {
  DailyLogsSection,
  FileCapsSection,
  SessionLedgerSection,
  WorkspaceIdentitySection,
} from "./-memory-file-sections";
import { DecisionsSection, ExtractorSection } from "./-memory-processing-sections";
import { RecallSection } from "./-memory-recall-section";
import { TEST_PREFIX, type ValidationSetter } from "./-memory-settings-types";
import { MemorySystemSection, ProviderResilienceSection } from "./-memory-system-sections";

export function MemorySettingsPage() {
  const page = useSettingsMemoryPage();
  useSettingsTopbar("memory");
  const [validationErrors, setValidationErrors] = useState<Record<string, string | null>>({});
  const setValidationError: ValidationSetter = (key: string) => (message: string | null) => {
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
    return (
      <div
        className="flex flex-1 items-center justify-center"
        data-testid={`${TEST_PREFIX}-loading`}
      >
        <Spinner className="size-5 text-subtle" />
      </div>
    );
  }

  if (page.error || !page.envelope || !page.draft) {
    return (
      <div className="flex flex-1 items-center justify-center" data-testid={`${TEST_PREFIX}-error`}>
        <div className="flex flex-col items-center gap-2 text-center">
          <AlertCircle className="size-6 text-danger" />
          <p className="text-sm text-subtle">
            {page.error?.message ?? "Failed to load memory settings"}
          </p>
          <Button onClick={page.handleRetry} size="sm" type="button" variant="outline">
            Retry
          </Button>
        </div>
      </div>
    );
  }

  const { envelope, draft, setDraft, restart } = page;
  const health = envelope.health;
  const dreamAvailable =
    health.available && envelope.actions.consolidate.available && envelope.health.dream_enabled;
  const validatedSectionProps = { draft, setDraft, validationErrors, setValidationError };

  return (
    <SettingsPageFrame
      description="What your agents remember across sessions, and how those memories are made."
      meta={
        health.available
          ? [
              {
                key: "files",
                content: (
                  <span>
                    <span className="font-medium text-muted">{health.file_count}</span> memory files
                  </span>
                ),
              },
              {
                key: "dream",
                content: health.last_consolidated_at ? (
                  <span
                    className="inline-flex items-center gap-1"
                    data-testid={`${TEST_PREFIX}-last-consolidated`}
                  >
                    last dream <Time iso={health.last_consolidated_at} mode="relative" />
                  </span>
                ) : (
                  <span data-testid={`${TEST_PREFIX}-last-consolidated`}>no dream runs yet</span>
                ),
              },
            ]
          : [{ key: "runtime", content: <span>runtime unavailable</span> }]
      }
      restart={restart}
      saveBar={
        <SettingsSaveBar
          slug="memory"
          state={saveBarState}
          onSave={page.handleSave}
          onReset={page.handleReset}
        />
      }
      slug="memory"
    >
      {!health.available ? (
        <SettingsRuntimeUnavailable
          slug="memory"
          description="Memory health and file counts could not be measured."
        />
      ) : null}
      <MemorySystemSection draft={draft} setDraft={setDraft} />
      <RecallSection {...validatedSectionProps} />
      <DreamSection
        {...validatedSectionProps}
        dreamAvailable={dreamAvailable}
        dreamPending={page.isTriggeringDream}
        onTriggerDream={page.handleTriggerDream}
        actionMessage={page.actionMessage}
      />
      <SessionLedgerSection {...validatedSectionProps} />
      <DailyLogsSection {...validatedSectionProps} />
      <WorkspaceIdentitySection draft={draft} setDraft={setDraft} />

      <SettingsAdvancedFold data-testid={`${TEST_PREFIX}-advanced`} padded>
        <ProviderResilienceSection {...validatedSectionProps} />
        <ControllerSection {...validatedSectionProps} />
        <DecisionsSection {...validatedSectionProps} />
        <ExtractorSection {...validatedSectionProps} />
        <FileCapsSection {...validatedSectionProps} />
      </SettingsAdvancedFold>
    </SettingsPageFrame>
  );
}
