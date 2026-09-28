import { useAddMarketplaceDialog } from "@/systems/marketplace";
import { useSettingsMarketplacePage } from "@/systems/settings/hooks/use-settings-marketplace-page";
import {
  SettingsMarketplaceCatalogSection,
  SettingsMarketplaceSourcesSection,
  SettingsPageFrame,
  SettingsPageState,
  SettingsSaveBar,
  useSettingsSaveBarState,
  useSettingsTopbar,
} from "@/systems/settings";

/**
 * Settings › Marketplace: the CompozyOS catalog keys behind the save bar, then the plugin
 * marketplaces, each row applying immediately through the sources routes. User scope only.
 */
export function MarketplaceSettingsPage() {
  const page = useSettingsMarketplacePage();
  const addMarketplace = useAddMarketplaceDialog();
  useSettingsTopbar("marketplace");
  const saveBarState = useSettingsSaveBarState({
    isDirty: page.isDirty,
    isInvalid: page.isInvalid,
    isSaving: page.isSaving,
    error: page.saveError,
    warnings: page.warnings,
    lastAppliedLabel: page.lastAppliedLabel,
  });

  if (page.isLoading) {
    return <SettingsPageState slug="marketplace" state="loading" />;
  }

  if (page.error || !page.envelope || !page.draft) {
    return (
      <SettingsPageState
        error={page.error}
        onRetry={page.handleRetry}
        slug="marketplace"
        state="error"
      />
    );
  }

  return (
    <SettingsPageFrame
      restart={page.restart}
      saveBar={
        <SettingsSaveBar
          onReset={page.handleReset}
          onSave={page.handleSave}
          slug="marketplace"
          state={saveBarState}
        />
      }
      slug="marketplace"
    >
      <SettingsMarketplaceCatalogSection draft={page.draft} onChange={page.setDraft} />
      <SettingsMarketplaceSourcesSection onAdd={addMarketplace.open} />
      {addMarketplace.dialog}
    </SettingsPageFrame>
  );
}
