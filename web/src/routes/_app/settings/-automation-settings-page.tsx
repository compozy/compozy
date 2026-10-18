import { AlertCircle, Zap } from "lucide-react";
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
  SettingsPageState,
  SettingsProvChip,
  SettingsSaveBar,
  useSettingsSaveBarState,
  useSettingsTopbar,
  type SettingsAutomationSection,
} from "@/systems/settings";
import { Alert, AlertDescription, AlertTitle, Eyebrow, Input, Switch, Time } from "@compozy/ui";

type AutomationConfig = SettingsAutomationSection["config"];
type AutomationRuntime = SettingsAutomationSection["runtime"];

export function AutomationSettingsPage() {
  const page = useSettingsAutomationPage();
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
  const runtime = page.envelope?.runtime;

  if (page.isLoading) {
    return <SettingsPageState slug="automation" state="loading" />;
  }

  if (page.error || !page.envelope || !page.draft) {
    return (
      <SettingsPageState
        error={page.error}
        onRetry={page.handleRetry}
        slug="automation"
        state="error"
      />
    );
  }

  if (!runtime) {
    return null;
  }
  const { draft, setDraft, restart } = page;

  return (
    <SettingsPageFrame
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
  const detail = unavailableParts.length > 0 ? unavailableParts.join(" · ") : undefined;

  return (
    <Alert
      data-testid="settings-page-automation-runtime-unavailable"
      role="alert"
      title={detail}
      variant="warning"
    >
      <AlertCircle aria-hidden="true" />
      <AlertTitle>Automation is off</AlertTitle>
      <AlertDescription>
        Turn on Run automation and restart CompozyOS. Your automations wait until then.
      </AlertDescription>
    </Alert>
  );
}

function AutomationHero({ runtime }: { runtime: AutomationRuntime }) {
  const running = runtime.running;
  const nextFire = runtime.next_fire;
  const lastSynced = runtime.last_synced_at;
  return (
    <SettingsHeroBoard
      data-testid="settings-page-automation-hero"
      state={running ? "Automation is running" : "Automation is stopped"}
      tone={running ? "success" : "neutral"}
      pulse={running}
      sub={
        nextFire || lastSynced ? (
          <span>
            {nextFire ? (
              <>
                Next run <Time iso={nextFire} mode="relative" />
              </>
            ) : null}
            {nextFire && lastSynced ? " · " : null}
            {lastSynced ? (
              <>
                Synced <Time iso={lastSynced} mode="relative" />
              </>
            ) : null}
          </span>
        ) : undefined
      }
    />
  );
}

function countLabel(count: number, singular: string): string {
  return `${count} ${count === 1 ? singular : `${singular}s`}`;
}

/** `7 automations, 6 on · 4 scheduled, 3 on events` — schedules are jobs, events are triggers. */
function automationsSummary(runtime: AutomationRuntime): string {
  const total = runtime.job_total + runtime.trigger_total;
  const on = runtime.job_enabled + runtime.trigger_enabled;
  return `${countLabel(total, "automation")}, ${on} on · ${runtime.job_total} scheduled, ${runtime.trigger_total} on events`;
}

function ManageSection({ runtime }: { runtime: AutomationRuntime }) {
  return (
    <SettingsGroup data-testid="settings-page-automation-operational-links" title="Manage">
      <SettingLinkRow
        data-testid="settings-page-automation-link-automations"
        description={automationsSummary(runtime)}
        label={
          <>
            <Zap aria-hidden="true" className="size-3.5 text-muted" />
            Automations
          </>
        }
        render={<Link to="/automations" />}
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
    <SettingsGroup title="Engine">
      <SettingsFieldRow
        data-testid="settings-page-automation-enabled"
        label="Run automation"
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
        label="Schedule time zone"
        help="Scheduled automations use this time zone"
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
    <SettingsGroup title="Limits">
      <SettingsFieldRow
        data-testid="settings-page-automation-max-concurrent"
        label="Scheduled automations at once"
        help={
          <span className="inline-flex flex-wrap items-center gap-1.5">
            How many scheduled automations can run at the same time
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
        label="Default run limit"
        help={
          <span className="inline-flex flex-wrap items-center gap-1.5">
            How often a new automation can start work
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
            <span className="text-form-hint text-subtle">per</span>
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
