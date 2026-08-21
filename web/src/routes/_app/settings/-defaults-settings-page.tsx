import { AlertCircle } from "lucide-react";

import { Button, Input, NativeSelect, NativeSelectOption, Spinner } from "@compozy/ui";

import {
  SettingRow,
  SettingsGroup,
  SettingsPageFrame,
  SettingsSaveBar,
  useSettingsPersonaPage,
  useSettingsProviders,
  useSettingsSandboxes,
  useSettingsSaveBarState,
  useSettingsTopbar,
} from "@/systems/settings";

export function DefaultsSettingsPage() {
  const page = useSettingsPersonaPage();
  const providers = useSettingsProviders();
  const sandboxes = useSettingsSandboxes();
  useSettingsTopbar("defaults");
  const saveBarState = useSettingsSaveBarState({
    isDirty: page.isDirty,
    isInvalid: false,
    isSaving: page.isSaving,
    error: page.saveError,
    warnings: page.warnings,
    lastAppliedLabel: null,
  });
  const dependencyError = providers.error ?? sandboxes.error;
  const dependencyErrorMessage =
    dependencyError instanceof Error ? dependencyError.message : "Failed to load runtime options";

  if (page.isLoading || providers.isLoading || sandboxes.isLoading) {
    return (
      <div
        aria-label="Loading profile defaults"
        className="flex flex-1 items-center justify-center"
        data-testid="settings-page-defaults-loading"
        role="status"
      >
        <Spinner aria-hidden="true" className="size-5 text-subtle" />
      </div>
    );
  }

  if (
    page.error !== null ||
    dependencyError !== null ||
    page.envelope === null ||
    page.draft === null
  ) {
    return (
      <div
        className="flex flex-1 items-center justify-center"
        data-testid="settings-page-defaults-error"
      >
        <div className="flex flex-col items-center gap-2 text-center">
          <AlertCircle aria-hidden="true" className="size-6 text-danger" />
          <p className="text-small-body text-subtle">
            {page.error?.message ?? dependencyErrorMessage}
          </p>
          <Button
            onClick={() => {
              page.handleRetry();
              void providers.refetch();
              void sandboxes.refetch();
            }}
            size="sm"
            type="button"
            variant="outline"
          >
            Retry
          </Button>
        </div>
      </div>
    );
  }

  const providerNames = (providers.data?.providers ?? []).map(entry => entry.name);
  const sandboxNames = (sandboxes.data?.sandboxes ?? []).map(entry => entry.name);
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
              <NativeSelectOption value="">auto</NativeSelectOption>
              {providerNames.map(name => (
                <NativeSelectOption key={name} value={name}>
                  {name}
                </NativeSelectOption>
              ))}
            </NativeSelect>
          }
          label="Provider"
        />
        <SettingRow
          control={
            <NativeSelect
              className="w-52 font-mono"
              data-testid="settings-page-defaults-sandbox"
              onChange={event => {
                const sandbox = event.currentTarget.value;
                setDraft(current => (current === null ? current : { ...current, sandbox }));
              }}
              value={draft.sandbox ?? ""}
            >
              <NativeSelectOption value="">local</NativeSelectOption>
              {sandboxNames.map(name => (
                <NativeSelectOption key={name} value={name}>
                  {name}
                </NativeSelectOption>
              ))}
            </NativeSelect>
          }
          label="Sandbox"
        />
      </SettingsGroup>
    </SettingsPageFrame>
  );
}
