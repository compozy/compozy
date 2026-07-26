import { AlertCircle } from "lucide-react";
import { useState, type Dispatch, type SetStateAction } from "react";

import { useSettingsObservabilityPage } from "@/systems/settings/hooks/use-settings-observability-page";
import {
  SettingsAdvancedFold,
  SettingsByteField,
  SettingsFieldRow,
  SettingsGroup,
  SettingsHeroGauge,
  SettingsPageFrame,
  SettingsProvChip,
  SettingsSaveBar,
  SettingsRuntimeUnavailable,
  useSettingsSaveBarState,
  useSettingsTopbar,
  type SettingsObservabilitySection,
} from "@/systems/settings";
import { Button, Spinner, Switch } from "@agh/ui";

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
    return (
      <div
        className="flex flex-1 items-center justify-center"
        data-testid="settings-page-observability-loading"
      >
        <Spinner className="size-5 text-subtle" />
      </div>
    );
  }

  if (page.error || !page.envelope || !page.draft) {
    return (
      <div
        className="flex flex-1 items-center justify-center"
        data-testid="settings-page-observability-error"
      >
        <div className="flex flex-col items-center gap-2 text-center">
          <AlertCircle className="size-6 text-danger" />
          <p className="text-sm text-subtle">
            {page.error?.message ?? "Failed to load observability settings"}
          </p>
          <Button onClick={page.handleRetry} size="sm" type="button" variant="outline">
            Retry
          </Button>
        </div>
      </div>
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
      description="What this daemon records about sessions, and how much disk it may use."
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
              {
                key: "storage",
                content: (
                  <span data-testid="settings-page-observability-storage-summary">
                    storage {formatBytes(totalStorage)} of {formatBytes(cap)}
                  </span>
                ),
              },
            ]
          : [{ key: "runtime", content: <span>runtime unavailable</span> }]
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
          legend={`Global DB ${formatBytes(runtime.global_db_size_bytes)} · Sessions ${formatBytes(runtime.session_db_size_bytes)} · ${formatBytes(Math.max(0, cap - totalStorage))} free`}
          foot={`${draft.enabled ? "Capturing" : "Capture off"} · ${runtime.active_sessions} active sessions · ${runtime.active_agents} agents working · ${draft.retention_days}-day retention`}
        />
      ) : (
        <SettingsRuntimeUnavailable
          slug="observability"
          description="Session, agent, and storage measurements could not be read."
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
    <SettingsGroup title="Capture" description="events, transcripts, logs">
      <SettingsFieldRow
        data-testid="settings-page-observability-enabled"
        label="Record activity"
        description="Persist every session event to SQLite for replay"
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
        description="Older events are pruned on the retention sweep"
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
    <SettingsGroup title="Transcripts" description="full replay of agent I/O">
      <SettingsFieldRow
        data-testid="settings-page-observability-transcripts-enabled"
        label="Save full transcripts"
        description="Chunked segment-based replay of every prompt + response"
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
    <SettingsGroup title="Storage limits" description="byte caps for capture stores">
      <SettingsFieldRow
        data-testid="settings-page-observability-max-global"
        label="Global storage cap"
        description={
          <span className="inline-flex flex-wrap items-center gap-1.5">
            Soft cap across the global event store
            <SettingsProvChip>observability.max_global_bytes</SettingsProvChip>
          </span>
        }
        control={
          <SettingsByteField
            data-testid="settings-page-observability-max-global-bytes"
            label="Global storage cap"
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
        description={
          <span className="inline-flex flex-wrap items-center gap-1.5">
            Chunk size for transcript segments
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
        description={
          <span className="inline-flex flex-wrap items-center gap-1.5">
            Per-session transcript ceiling
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
