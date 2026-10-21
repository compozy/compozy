import { useProviderUsageHost } from "@/systems/os";
import {
  ProviderUsageCollectionSection,
  ProviderUsageKeychainSection,
  ProviderUsageProvidersSection,
  SettingsPageFrame,
  SettingsPageState,
  SettingsSaveBar,
  useSettingsProviderUsagePage,
  useSettingsSaveBarState,
  useSettingsTopbar,
} from "@/systems/settings";

/**
 * Settings › Provider usage: the master switch, background cadence, which
 * providers are asked for usage, and — on a macOS daemon host — Claude
 * Keychain consent. One draft, saved through the apply lifecycle; a rejected
 * apply keeps the draft and shows the daemon's message in the save bar.
 */
export function ProviderUsageSettingsPage() {
  const page = useSettingsProviderUsagePage();
  const { isMacHost } = useProviderUsageHost();
  useSettingsTopbar();
  const saveBarState = useSettingsSaveBarState({
    isDirty: page.isDirty,
    isInvalid: false,
    isSaving: page.isSaving,
    error: page.saveError,
    warnings: page.warnings,
    lastAppliedLabel: page.lastAppliedLabel,
  });

  if (page.isLoading) {
    return <SettingsPageState slug="provider-usage" state="loading" />;
  }

  if (page.error || !page.envelope || !page.draft) {
    return (
      <SettingsPageState
        error={page.error}
        onRetry={page.handleRetry}
        slug="provider-usage"
        state="error"
      />
    );
  }

  const draft = page.draft;
  return (
    <SettingsPageFrame
      restart={page.restart}
      saveBar={
        <SettingsSaveBar
          onReset={page.handleReset}
          onSave={page.handleSave}
          slug="provider-usage"
          state={saveBarState}
        />
      }
      slug="provider-usage"
    >
      <ProviderUsageCollectionSection
        disabled={page.isSaving}
        draft={draft}
        onChange={page.setDraft}
      />
      <ProviderUsageProvidersSection
        disabled={page.isSaving}
        draft={draft}
        onChange={page.setDraft}
        providers={page.providers}
      />
      <ProviderUsageKeychainSection
        disabled={page.isSaving}
        draft={draft}
        isMacHost={isMacHost}
        onChange={page.setDraft}
      />
    </SettingsPageFrame>
  );
}
