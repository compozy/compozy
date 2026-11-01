import { useId } from "react";

import { RuntimeSelector, type RuntimeSelectorValue } from "@/systems/runtime";
import { useSmoothStreamingPreference } from "@/systems/session";
import {
  personaConfigWithRuntime,
  sessionDefaultsRuntimeValue,
  SettingRow,
  SettingsGroup,
  type SettingsRuntimeCatalog,
  type useSettingsPersonaPage,
} from "@/systems/settings";
import { Button, Input, Switch } from "@compozy/ui";

type PersonaPageModel = ReturnType<typeof useSettingsPersonaPage>;

const AUTOMATIC_RUNTIME_LABEL = "Automatic (recommended)";

const AUTOMATIC_RUNTIME: RuntimeSelectorValue = { provider: "", model: "", reasoning_effort: "" };

// Client-local presentation preference (ADR-008, S10): stored per browser,
// never a daemon key, so it lives outside the persona draft and the save bar.
function SmoothStreamingSetting() {
  const smoothStreaming = useSmoothStreamingPreference();
  return (
    <SettingRow
      control={
        <Switch
          aria-label="Smooth streaming"
          checked={smoothStreaming.enabled}
          data-testid="settings-page-general-smooth-streaming"
          onCheckedChange={checked => smoothStreaming.setEnabled(checked === true)}
        />
      }
      label="Smooth streaming"
    />
  );
}

/**
 * What a new session starts with when nothing picks otherwise: the profile's
 * default agent and runtime (`[defaults]`), plus this browser's streaming
 * preference. The persona draft shares the General save bar.
 */
export function SessionDefaultsGroup({
  persona,
  runtime,
  disabled,
}: {
  persona: PersonaPageModel;
  runtime: SettingsRuntimeCatalog;
  disabled: boolean;
}) {
  const runtimeLabelId = `${useId().replace(/:/g, "")}-runtime-label`;
  const { draft, setDraft } = persona;
  if (draft === null) return null;

  const value = sessionDefaultsRuntimeValue(draft);
  const setRuntime = (next: RuntimeSelectorValue) =>
    setDraft(current => (current === null ? current : personaConfigWithRuntime(current, next)));

  return (
    <SettingsGroup
      data-testid="settings-page-general-session-defaults"
      description={
        <span data-testid="settings-page-general-session-defaults-scope">
          New sessions in the{" "}
          <span className="font-medium text-muted-foreground">{persona.profileName}</span> profile
          start with these.
        </span>
      }
      title="Session defaults"
    >
      <SettingRow
        control={
          <Input
            className="w-52"
            data-testid="settings-page-general-default-agent"
            disabled={disabled}
            onChange={event => {
              const agent = event.currentTarget.value;
              setDraft(current => (current === null ? current : { ...current, agent }));
            }}
            value={draft.agent}
          />
        }
        label="Agent"
      />
      <SettingRow
        data-testid="settings-page-general-default-runtime"
        help="Provider, model and reasoning effort for new sessions that don't pick their own."
        label={<span id={runtimeLabelId}>Model</span>}
        helpLabel="About model"
        control={
          <div className="flex flex-wrap items-center gap-2">
            {value.provider ? (
              <Button
                data-testid="settings-page-general-default-runtime-clear"
                disabled={disabled}
                onClick={() => setRuntime(AUTOMATIC_RUNTIME)}
                size="sm"
                type="button"
                variant="ghost"
              >
                Use automatic
              </Button>
            ) : null}
            {value.provider && (value.model || value.reasoning_effort) ? (
              <Button
                data-testid="settings-page-general-provider-default"
                disabled={disabled}
                onClick={() => setRuntime({ ...AUTOMATIC_RUNTIME, provider: value.provider })}
                size="sm"
                type="button"
                variant="ghost"
              >
                Use provider default
              </Button>
            ) : null}
            <RuntimeSelector
              ariaLabelledby={runtimeLabelId}
              disabled={disabled || runtime.providers.length === 0}
              loading={runtime.loading}
              modelPlaceholder={value.provider ? "Provider default" : AUTOMATIC_RUNTIME_LABEL}
              models={runtime.models}
              onChange={setRuntime}
              onRefreshCatalog={runtime.refresh}
              providers={runtime.providers}
              refreshing={runtime.refreshing}
              triggerTestId="settings-page-general-default-runtime-select"
              value={value}
              variant="small"
              {...(runtime.catalogError ? { catalogStatus: runtime.catalogError } : {})}
            />
          </div>
        }
      />
      <SmoothStreamingSetting />
    </SettingsGroup>
  );
}
