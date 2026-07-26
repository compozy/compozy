import { AlertCircle } from "lucide-react";
import { useState, type Dispatch, type SetStateAction } from "react";
import { Link } from "@tanstack/react-router";

import { useSettingsAutomationPage } from "@/systems/settings/hooks/use-settings-automation-page";
import {
  SettingLinkRow,
  SettingsAdvancedFold,
  SettingsFieldRow,
  SettingsGroup,
  SettingsHeroBoard,
  SettingsNumberInput,
  SettingsPageFrame,
  SettingsProvChip,
  SettingsSaveBar,
  useSettingsSaveBarState,
  useSettingsTopbar,
  type SettingsAutomationSection,
} from "@/systems/settings";
import { Button, Eyebrow, Input, Spinner, Switch, Time } from "@agh/ui";

type AutomationConfig = SettingsAutomationSection["config"];
type AutomationRuntime = SettingsAutomationSection["runtime"];

export function AutomationSettingsPage() {
  const page = useSettingsAutomationPage();
  useSettingsTopbar("automation");
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
  const runtime = page.envelope?.runtime;

  if (page.isLoading) {
    return (
      <div
        className="flex flex-1 items-center justify-center"
        data-testid="settings-page-automation-loading"
      >
        <Spinner className="size-5 text-subtle" />
      </div>
    );
  }

  if (page.error || !page.envelope || !page.draft) {
    return (
      <div
        className="flex flex-1 items-center justify-center"
        data-testid="settings-page-automation-error"
      >
        <div className="flex flex-col items-center gap-2 text-center">
          <AlertCircle className="size-6 text-danger" />
          <p className="text-sm text-subtle">
            {page.error?.message ?? "Failed to load automation settings"}
          </p>
          <Button onClick={page.handleRetry} size="sm" type="button" variant="outline">
            Retry
          </Button>
        </div>
      </div>
    );
  }

  if (!runtime) {
    return null;
  }
  const { draft, setDraft, restart } = page;

  return (
    <SettingsPageFrame
      description="Scheduled jobs and event triggers this daemon runs on its own."
      meta={[
        {
          key: "jobs",
          content: (
            <span>
              <span className="font-medium text-muted">{runtime.job_enabled}</span> of{" "}
              {runtime.job_total} jobs active
            </span>
          ),
        },
        {
          key: "triggers",
          content: (
            <span>
              <span className="font-medium text-muted">{runtime.trigger_enabled}</span> of{" "}
              {runtime.trigger_total} triggers active
            </span>
          ),
        },
      ]}
      restart={restart}
      saveBar={
        <SettingsSaveBar
          slug="automation"
          state={saveBarState}
          onSave={page.handleSave}
          onReset={page.handleReset}
        />
      }
      slug="automation"
    >
      {!runtime.available ? <AutomationRuntimeUnavailable runtime={runtime} /> : null}
      <AutomationHero runtime={runtime} />
      <EngineSection draft={draft} setDraft={setDraft} />
      <ManageSection runtime={runtime} />
      <SettingsAdvancedFold
        data-testid="settings-page-automation-advanced"
        label="Advanced — limits"
        padded
      >
        <LimitsSection
          draft={draft}
          setDraft={setDraft}
          validationErrors={validationErrors}
          setValidationError={setValidationError}
        />
      </SettingsAdvancedFold>
    </SettingsPageFrame>
  );
}

function AutomationRuntimeUnavailable({ runtime }: { runtime: AutomationRuntime }) {
  const unavailableParts = [
    !runtime.running ? "engine stopped" : null,
    !runtime.scheduler_running ? "scheduler stopped" : null,
  ].filter((part): part is string => part !== null);
  const detail =
    unavailableParts.length > 0 ? unavailableParts.join(" · ") : "automation runtime unavailable";

  return (
    <div
      className="flex items-start gap-2 border border-warning/40 bg-warning-soft px-4 py-3 text-sm text-fg"
      data-testid="settings-page-automation-runtime-unavailable"
      role="alert"
    >
      <AlertCircle className="mt-0.5 size-4 shrink-0 text-warning" aria-hidden="true" />
      <div className="min-w-0">
        <p className="font-medium">Automation runtime is unavailable</p>
        <p className="mt-1 text-xs text-muted">
          Jobs and triggers cannot run until automation is enabled and the daemon is restarted.
        </p>
        <p className="mt-1 font-mono text-mono-id text-muted">{detail}</p>
      </div>
    </div>
  );
}

function AutomationHero({ runtime }: { runtime: AutomationRuntime }) {
  const running = runtime.running;
  return (
    <SettingsHeroBoard
      data-testid="settings-page-automation-hero"
      state={running ? "Automation running" : "Automation stopped"}
      tone={running ? "success" : "neutral"}
      pulse={running}
      pill={running ? "Running" : "Stopped"}
      sub={
        runtime.next_fire ? (
          <span>
            next fire <Time iso={runtime.next_fire} mode="relative" />
          </span>
        ) : undefined
      }
      stats={[
        {
          key: "jobs",
          value: `${runtime.job_enabled}/${runtime.job_total}`,
          label: "Jobs enabled",
        },
        {
          key: "triggers",
          value: `${runtime.trigger_enabled}/${runtime.trigger_total}`,
          label: "Triggers enabled",
        },
        {
          key: "synced",
          value: runtime.last_synced_at ? (
            <Time iso={runtime.last_synced_at} mode="relative" />
          ) : (
            "—"
          ),
          label: "Last synced",
        },
      ]}
    />
  );
}

function ManageSection({ runtime }: { runtime: AutomationRuntime }) {
  return (
    <SettingsGroup
      data-testid="settings-page-automation-operational-links"
      description="Jobs and triggers live in their own views; this page owns the engine itself."
      title="Manage"
    >
      <SettingLinkRow
        data-testid="settings-page-automation-link-jobs"
        description={`${runtime.job_total} defined, ${runtime.job_enabled} enabled`}
        label="Jobs"
        render={<Link to="/jobs" />}
      />
      <SettingLinkRow
        data-testid="settings-page-automation-link-triggers"
        description={`${runtime.trigger_total} defined, ${runtime.trigger_enabled} enabled`}
        label="Triggers"
        render={<Link to="/triggers" />}
      />
    </SettingsGroup>
  );
}

interface DraftSectionProps {
  draft: AutomationConfig;
  setDraft: Dispatch<SetStateAction<AutomationConfig | null>>;
}

function EngineSection({ draft, setDraft }: DraftSectionProps) {
  return (
    <SettingsGroup title="Engine" description="persisted to config.toml">
      <SettingsFieldRow
        data-testid="settings-page-automation-enabled"
        label="Run automation"
        description="Runs jobs and triggers on the daemon"
        control={
          <Switch
            data-testid="settings-page-automation-enabled-switch"
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
        data-testid="settings-page-automation-timezone"
        label="Schedule timezone"
        description="Used for cron schedule resolution"
        control={
          <Input
            className="w-56 font-mono"
            data-testid="settings-page-automation-timezone-input"
            value={draft.timezone ?? ""}
            placeholder="UTC"
            onChange={event =>
              setDraft(prev => {
                const current = prev ?? draft;
                return { ...current, timezone: event.target.value };
              })
            }
          />
        }
      />
    </SettingsGroup>
  );
}

function LimitsSection({
  draft,
  setDraft,
  validationErrors,
  setValidationError,
}: DraftSectionProps & {
  validationErrors: Record<string, string | null>;
  setValidationError: (key: string) => (message: string | null) => void;
}) {
  return (
    <SettingsGroup title="Limits" description="resource caps">
      <SettingsFieldRow
        data-testid="settings-page-automation-max-concurrent"
        label="Max jobs at once"
        description={
          <span className="inline-flex flex-wrap items-center gap-1.5">
            Caps the number of jobs running simultaneously
            <SettingsProvChip>automation.max_concurrent_jobs</SettingsProvChip>
          </span>
        }
        error={validationErrors.maxConcurrentJobs ?? undefined}
        control={
          <SettingsNumberInput
            min={0}
            className="w-24"
            data-testid="settings-page-automation-max-concurrent-input"
            value={draft.max_concurrent_jobs}
            onValidityChange={setValidationError("maxConcurrentJobs")}
            onValueChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  max_concurrent_jobs: value,
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid="settings-page-automation-fire-limit-max"
        label="Default fire limit"
        description={
          <span className="inline-flex flex-wrap items-center gap-1.5">
            Maximum invocations per window for new triggers
            <SettingsProvChip>automation.default_fire_limit</SettingsProvChip>
          </span>
        }
        error={validationErrors.defaultFireLimitMax ?? undefined}
        control={
          <div className="flex items-center gap-2">
            <SettingsNumberInput
              min={0}
              className="w-24"
              data-testid="settings-page-automation-fire-limit-max-input"
              value={draft.default_fire_limit.max}
              onValidityChange={setValidationError("defaultFireLimitMax")}
              onValueChange={value =>
                setDraft(prev => {
                  const current = prev ?? draft;
                  return {
                    ...current,
                    default_fire_limit: {
                      ...current.default_fire_limit,
                      max: value,
                    },
                  };
                })
              }
            />
            <Eyebrow className="text-muted">fires</Eyebrow>
            <span className="text-xs text-subtle">per</span>
            <Input
              className="w-24 font-mono"
              data-testid="settings-page-automation-fire-limit-window-input"
              value={draft.default_fire_limit.window ?? ""}
              placeholder="1m"
              onChange={event =>
                setDraft(prev => {
                  const current = prev ?? draft;
                  return {
                    ...current,
                    default_fire_limit: {
                      ...current.default_fire_limit,
                      window: event.target.value,
                    },
                  };
                })
              }
            />
          </div>
        }
      />
    </SettingsGroup>
  );
}
