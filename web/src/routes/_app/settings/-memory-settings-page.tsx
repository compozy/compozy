import { useState } from "react";

import { useSettingsMemoryPage } from "@/systems/settings/hooks/use-settings-memory-page";
import {
  SettingsAdvancedFold,
  SettingsPageFrame,
  SettingsPageState,
  SettingsSaveBar,
  SettingsRuntimeUnavailable,
  useSettingsSaveBarState,
  useSettingsTopbar,
} from "@/systems/settings";
import { Time } from "@compozy/ui";
import { ControllerSection } from "./-memory-controller-sections";
import { DreamSection, TidyUpRow } from "./-memory-dream-section";
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
  useSettingsTopbar();
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
    return <SettingsPageState slug="memory" state="loading" />;
  }

  if (page.error || !page.envelope || !page.draft) {
    return (
      <SettingsPageState
        error={page.error}
        onRetry={page.handleRetry}
        slug="memory"
        state="error"
      />
    );
  }

  const { envelope, draft, setDraft, restart } = page;
  const health = envelope.health;
  const dreamAvailable =
    health.available && envelope.actions.consolidate.available && envelope.health.dream_enabled;
  const validatedSectionProps = { draft, setDraft, validationErrors, setValidationError };

  return (
    <SettingsPageFrame
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
                    last tidied <Time iso={health.last_consolidated_at} mode="relative" />
                  </span>
                ) : (
                  <span data-testid={`${TEST_PREFIX}-last-consolidated`}>not tidied yet</span>
                ),
              },
            ]
          : [{ key: "runtime", content: <span>CompozyOS isn't reachable</span> }]
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
      <MemorySystemSection draft={draft} setDraft={setDraft}>
        <TidyUpRow
          actionMessage={page.actionMessage}
          dreamAvailable={dreamAvailable}
          dreamPending={page.isTriggeringDream}
          onTriggerDream={page.handleTriggerDream}
        />
      </MemorySystemSection>

      <SettingsAdvancedFold data-testid={`${TEST_PREFIX}-advanced`} padded>
        <RecallSection {...validatedSectionProps} />
        <DreamSection {...validatedSectionProps} />
        <SessionLedgerSection {...validatedSectionProps} />
        <DailyLogsSection {...validatedSectionProps} />
        <FileCapsSection {...validatedSectionProps} />
        <WorkspaceIdentitySection draft={draft} setDraft={setDraft} />
        <ProviderResilienceSection {...validatedSectionProps} />
        <ControllerSection {...validatedSectionProps} />
        <DecisionsSection {...validatedSectionProps} />
        <ExtractorSection {...validatedSectionProps} />
      </SettingsAdvancedFold>
    </SettingsPageFrame>
  );
}
