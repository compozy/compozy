import { useState, type Dispatch, type SetStateAction } from "react";

import { useSettingsObservabilityPage } from "@/systems/settings/hooks/use-settings-observability-page";
import {
  SettingsAdvancedFold,
  SettingsByteField,
  SettingsFieldRow,
  SettingsGroup,
  SettingsHeroGauge,
  SettingsPageFrame,
  SettingsPageState,
  SettingsProvChip,
  SettingsSaveBar,
  SettingsRuntimeUnavailable,
  useSettingsSaveBarState,
  useSettingsTopbar,
  type SettingsObservabilitySection,
} from "@/systems/settings";
import { Switch } from "@compozy/ui";

type ObservabilityConfig = SettingsObservabilitySection["config"];

import { ObservabilityNumberField as NumberField } from "./-observability-fields";
import { formatBytes } from "./-observability-format";
import { ObservabilityDiagnosticsSection } from "./-observability-support-bundle-section";

export function ObservabilitySettingsPage() {
  const page = useSettingsObservabilityPage();
  useSettingsTopbar("observability");
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
    return <SettingsPageState slug="observability" state="loading" />;
  }

  if (page.error || !page.envelope || !page.draft) {
    return (
      <SettingsPageState
        error={page.error}
        onRetry={page.handleRetry}
        slug="observability"
        state="error"
      />
    );
  }

  const { envelope, draft, setDraft, restart } = page;
  const runtime = envelope.runtime;
  const logTail = envelope.log_tail;
  const totalStorage = runtime.global_db_size_bytes + runtime.session_db_size_bytes;
  const cap = draft.max_global_bytes;
  const capPercent = cap > 0 ? Math.min(100, Math.round((totalStorage / cap) * 100)) : 0;

  return (
    <SettingsPageFrame
      meta={
        runtime.available
          ? [
              {
                key: "sessions",
                content: (
                  <span>
                    <span className="font-medium text-muted">{runtime.active_sessions}</span> active
                    sessions
                  </span>
                ),
              },
            ]
          : [{ key: "runtime", content: <span>CompozyOS isn't reachable</span> }]
      }
      restart={restart}
      saveBar={
        <SettingsSaveBar
          slug="observability"
          state={saveBarState}
          onSave={page.handleSave}
          onReset={() => {
            setValidationErrors({});
            page.handleReset();
          }}
        />
      }
      slug="observability"
    >
      {runtime.available ? (
        <SettingsHeroGauge
          data-testid="settings-page-observability-hero"
          usage={`${formatBytes(totalStorage)} of ${formatBytes(cap)}`}
          percent={capPercent}
          tone={capPercent >= 95 ? "danger" : capPercent >= 80 ? "warning" : "success"}
          pill={capPercent >= 80 ? "Filling up" : "Healthy"}
          legend={`App data ${formatBytes(runtime.global_db_size_bytes)} · Sessions ${formatBytes(runtime.session_db_size_bytes)} · ${formatBytes(Math.max(0, cap - totalStorage))} free`}
        />
      ) : (
        <SettingsRuntimeUnavailable
          slug="observability"
          description="Session and storage numbers couldn't be read."
        />
      )}
      <CaptureSection
        draft={draft}
        setDraft={setDraft}
        validationErrors={validationErrors}
        setValidationError={setValidationError}
      />
      <TranscriptsSection draft={draft} setDraft={setDraft} />
      <ObservabilityDiagnosticsSection logTail={logTail} />
      <SettingsAdvancedFold
        data-testid="settings-page-observability-advanced"
        label="Advanced — storage limits"
        padded
      >
        <StorageLimitsSection draft={draft} setDraft={setDraft} />
      </SettingsAdvancedFold>
    </SettingsPageFrame>
  );
}

interface DraftSectionProps {
  draft: ObservabilityConfig;
  setDraft: Dispatch<SetStateAction<ObservabilityConfig | null>>;
}

function CaptureSection({
  draft,
  setDraft,
  validationErrors,
  setValidationError,
}: DraftSectionProps & {
  validationErrors: Record<string, string | null>;
  setValidationError: (key: string) => (message: string | null) => void;
}) {
  return (
    <SettingsGroup title="Capture">
      <SettingsFieldRow
        data-testid="settings-page-observability-enabled"
        label="Record activity"
        help="Save every session step so you can replay it"
        control={
          <Switch
            data-testid="settings-page-observability-enabled-switch"
            checked={draft.enabled}
            onCheckedChange={checked =>
              setDraft(prev => {
                const current = prev ?? draft;
                return { ...current, enabled: checked };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid="settings-page-observability-retention"
        label="Keep records for"
        description="Older activity is deleted automatically"
        error={validationErrors.retentionDays ?? undefined}
        control={
          <NumberField
            label="Keep records for"
            testId="settings-page-observability-retention-days"
            value={draft.retention_days}
            errorMessage={validationErrors.retentionDays ?? undefined}
            suffix="days"
            hideLabel
            onValidityChange={setValidationError("retentionDays")}
            onChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return { ...current, retention_days: value };
              })
            }
          />
        }
      />
    </SettingsGroup>
  );
}

function TranscriptsSection({ draft, setDraft }: DraftSectionProps) {
  return (
    <SettingsGroup title="Transcripts">
      <SettingsFieldRow
        data-testid="settings-page-observability-transcripts-enabled"
        label="Save full transcripts"
        help="Keep every prompt and reply so you can replay a session"
        control={
          <Switch
            data-testid="settings-page-observability-transcripts-enabled-switch"
            checked={draft.transcripts.enabled}
            onCheckedChange={checked =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  transcripts: { ...current.transcripts, enabled: checked },
                };
              })
            }
          />
        }
      />
    </SettingsGroup>
  );
}

function StorageLimitsSection({ draft, setDraft }: DraftSectionProps) {
  return (
    <SettingsGroup title="Storage limits">
      <SettingsFieldRow
        data-testid="settings-page-observability-max-global"
        label="Storage limit"
        help={
          <span className="inline-flex flex-wrap items-center gap-1.5">
            Suggested size limit for all saved activity
            <SettingsProvChip>observability.max_global_bytes</SettingsProvChip>
          </span>
        }
        control={
          <SettingsByteField
            data-testid="settings-page-observability-max-global-bytes"
            label="Storage limit"
            value={draft.max_global_bytes}
            onChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return { ...current, max_global_bytes: value };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid="settings-page-observability-segment"
        label="Transcript segment size"
        help={
          <span className="inline-flex flex-wrap items-center gap-1.5">
            Size of each saved transcript piece
            <SettingsProvChip>observability.transcripts.segment_bytes</SettingsProvChip>
          </span>
        }
        control={
          <SettingsByteField
            data-testid="settings-page-observability-segment-bytes"
            label="Transcript segment size"
            value={draft.transcripts.segment_bytes}
            onChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  transcripts: { ...current.transcripts, segment_bytes: value },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid="settings-page-observability-max-per-session"
        label="Transcript cap per session"
        help={
          <span className="inline-flex flex-wrap items-center gap-1.5">
            Largest transcript kept for one session
            <SettingsProvChip>observability.transcripts.max_bytes_per_session</SettingsProvChip>
          </span>
        }
        control={
          <SettingsByteField
            data-testid="settings-page-observability-transcripts-max-bytes"
            label="Transcript cap per session"
            value={draft.transcripts.max_bytes_per_session}
            onChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  transcripts: {
                    ...current.transcripts,
                    max_bytes_per_session: value,
                  },
                };
              })
            }
          />
        }
      />
    </SettingsGroup>
  );
}
