import { AlertCircle } from "lucide-react";

import { useSettingsExtensionsPage } from "@/systems/settings/hooks/use-settings-extensions-page";
import {
  SettingsPageFrame,
  SettingsSaveBar,
  useSettingsSaveBarState,
  useSettingsTopbar,
} from "@/systems/settings";
import { Button, Spinner } from "@agh/ui";

import { PolicySection } from "./-extensions-policy-section";

export function ExtensionsSettingsPage() {
  const page = useSettingsExtensionsPage();
  useSettingsTopbar("extensions");
  const saveBarState = useSettingsSaveBarState({
    isDirty: page.isPolicyDirty,
    isSaving: page.isSavingPolicy,
    error: page.savePolicyError,
    warnings: page.policyWarnings,
  });

  if (page.isLoading)
    return (
      <div
        className="flex flex-1 items-center justify-center"
        data-testid="settings-page-extensions-loading"
      >
        <Spinner className="size-5 text-subtle" />
      </div>
    );
  if (page.error || !page.envelope || !page.draft)
    return (
      <div
        className="flex flex-1 items-center justify-center"
        data-testid="settings-page-extensions-error"
      >
        <div className="flex flex-col items-center gap-2 text-center">
          <AlertCircle className="size-6 text-danger" />
          <p className="text-sm text-subtle">
            {page.error?.message ?? "Failed to load extensions settings"}
          </p>
          <Button onClick={page.handleRetry} size="sm" type="button" variant="outline">
            Retry
          </Button>
        </div>
      </div>
    );
  const registry = page.draft.marketplace.registry?.trim();
  return (
    <SettingsPageFrame
      description="What extensions are allowed to run on this daemon, and from where."
      meta={[
        {
          key: "source",
          content: (
            <span>
              Source <span className="font-medium text-muted">{registry || "unset"}</span>
            </span>
          ),
        },
        {
          key: "unverified",
          content: page.draft.marketplace.allow_unverified
            ? "Unverified allowed"
            : "Unverified blocked",
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
