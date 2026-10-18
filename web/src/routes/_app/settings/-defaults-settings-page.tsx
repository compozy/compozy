import { Input, NativeSelect, NativeSelectOption, Switch } from "@compozy/ui";

import { useSmoothStreamingPreference } from "@/systems/session";

import {
  SettingRow,
  SettingsGroup,
  SettingsPageFrame,
  SettingsPageState,
  SettingsSaveBar,
  useSettingsPersonaPage,
  useSettingsProviders,
  useSettingsSaveBarState,
  useSettingsTopbar,
} from "@/systems/settings";

// Client-local presentation preference (ADR-008, S10): stored per browser,
// never a daemon key, so it lives outside the persona draft and its save bar.
function SmoothStreamingSetting() {
  const smoothStreaming = useSmoothStreamingPreference();
  return (
    <SettingRow
      control={
        <Switch
          aria-label="Smooth streaming"
          checked={smoothStreaming.enabled}
          data-testid="settings-page-defaults-smooth-streaming"
          onCheckedChange={checked => smoothStreaming.setEnabled(checked === true)}
        />
      }
      label="Smooth streaming"
    />
  );
}

export function DefaultsSettingsPage() {
  const page = useSettingsPersonaPage();
  const providers = useSettingsProviders();
  useSettingsTopbar();
  const saveBarState = useSettingsSaveBarState({
    isDirty: page.isDirty,
    isInvalid: false,
    isSaving: page.isSaving,
    error: page.saveError,
    warnings: page.warnings,
    lastAppliedLabel: null,
  });
  const dependencyError = providers.error;

  if (page.isLoading || providers.isLoading) {
    return <SettingsPageState slug="defaults" state="loading" />;
  }

  if (
    page.error !== null ||
    dependencyError !== null ||
    page.envelope === null ||
    page.draft === null
  ) {
    return (
      <SettingsPageState
        error={page.error ?? dependencyError}
        onRetry={() => {
          page.handleRetry();
          void providers.refetch();
        }}
        slug="defaults"
        state="error"
      />
    );
  }

  const providerOptions = (providers.data?.providers ?? []).map(entry => ({
    name: entry.name,
    label: entry.settings?.display_name?.trim() || entry.name,
  }));
  const { draft, setDraft } = page;

  return (
    <SettingsPageFrame
      meta={[
        {
          key: "profile",
          content: (
            <span>
              Profile <span className="font-medium text-muted">{page.profileName}</span>
            </span>
          ),
        },
      ]}
      restart={page.restart}
      saveBar={
        <SettingsSaveBar
          slug="defaults"
          state={saveBarState}
          onReset={page.handleReset}
          onSave={page.handleSave}
        />
      }
      slug="defaults"
    >
      <SettingsGroup data-testid="settings-page-defaults-session" title="Session defaults">
        <SettingRow
          control={
            <Input
              className="w-52"
              data-testid="settings-page-defaults-agent"
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
          control={
            <NativeSelect
              className="w-52"
              data-testid="settings-page-defaults-provider"
              onChange={event => {
                const provider = event.currentTarget.value;
                setDraft(current => (current === null ? current : { ...current, provider }));
              }}
              value={draft.provider ?? ""}
            >
              <NativeSelectOption value="">Automatic (recommended)</NativeSelectOption>
              {providerOptions.map(option => (
                <NativeSelectOption key={option.name} value={option.name}>
                  {option.label}
                </NativeSelectOption>
              ))}
            </NativeSelect>
          }
          label="Provider"
        />

        <SmoothStreamingSetting />
      </SettingsGroup>
    </SettingsPageFrame>
  );
}
