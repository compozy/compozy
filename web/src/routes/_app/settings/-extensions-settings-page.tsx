import { useSettingsExtensionsPage } from "@/systems/settings/hooks/use-settings-extensions-page";
import {
  SettingsPageFrame,
  SettingsPageState,
  SettingsSaveBar,
  useSettingsSaveBarState,
  useSettingsTopbar,
} from "@/systems/settings";

import { PolicySection } from "./-extensions-policy-section";
import { ExtensionPalettePanels } from "./-extension-palette-panel";

export function ExtensionsSettingsPage() {
  const page = useSettingsExtensionsPage();
  useSettingsTopbar("extensions");
  const saveBarState = useSettingsSaveBarState({
    isDirty: page.isPolicyDirty,
    isSaving: page.isSavingPolicy,
    error: page.savePolicyError,
    warnings: page.policyWarnings,
  });

  if (page.isLoading) return <SettingsPageState slug="extensions" state="loading" />;
  if (page.error || !page.envelope || !page.draft)
    return (
      <SettingsPageState
        error={page.error}
        onRetry={page.handleRetry}
        slug="extensions"
        state="error"
      />
    );
  const enabledSources: string[] = [];
  if (page.draft.sources.github.enabled) enabledSources.push("github");
  if (page.draft.sources.git.enabled) enabledSources.push("git");
  return (
    <SettingsPageFrame
      meta={[
        {
          key: "sources",
          content: (
            <span>
              Sources{" "}
              <span className="font-medium text-muted">
                {enabledSources.length ? enabledSources.join(", ") : "none"}
              </span>
            </span>
          ),
        },
        {
          key: "unverified",
          content: page.draft.trust.allow_unverified ? "Unverified allowed" : "Unverified blocked",
        },
      ]}
      restart={page.restart}
      saveBar={
        <SettingsSaveBar
          onReset={page.handleResetPolicy}
          onSave={page.handleSavePolicy}
          slug="extensions"
          state={saveBarState}
        />
      }
      slug="extensions"
    >
      <ExtensionPalettePanels extensions={page.envelope.installed ?? []} />
      <PolicySection
        canMutate={page.canMutatePolicy}
        draft={page.draft}
        setDraft={value =>
          page.updatePolicyDraft(current => (typeof value === "function" ? value(current) : value))
        }
      />
    </SettingsPageFrame>
  );
}
