import { AlertCircle } from "lucide-react";

import { useAddMarketplaceDialog } from "@/systems/marketplace";
import { useSettingsMarketplacePage } from "@/systems/settings/hooks/use-settings-marketplace-page";
import {
  SettingsMarketplaceCatalogSection,
  SettingsMarketplaceSourcesSection,
  SettingsPageFrame,
  SettingsSaveBar,
  useSettingsSaveBarState,
  useSettingsTopbar,
} from "@/systems/settings";
import { Button, Spinner } from "@compozy/ui";

/**
 * Settings › Marketplace: the Compozy catalog keys behind the save bar, then the plugin
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
    return (
      <div
        aria-label="Loading marketplace settings"
        className="flex flex-1 items-center justify-center"
        data-testid="settings-page-marketplace-loading"
        role="status"
      >
        <Spinner aria-hidden="true" className="size-5 text-subtle" />
      </div>
    );
  }

  if (page.error || !page.envelope || !page.draft) {
    return (
      <div
        className="flex flex-1 items-center justify-center"
        data-testid="settings-page-marketplace-error"
      >
        <div className="flex flex-col items-center gap-2 text-center">
          <AlertCircle className="size-6 text-danger" />
          <p className="text-sm text-subtle">
            {page.error?.message ?? "Failed to load marketplace settings"}
          </p>
          <Button onClick={page.handleRetry} size="sm" type="button" variant="outline">
            Retry
          </Button>
        </div>
      </div>
    );
  }

  return (
    <SettingsPageFrame
      meta={[{ key: "scope", content: <span>scope user</span> }]}
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
