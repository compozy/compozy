import { AlertCircle } from "lucide-react";

import { Alert, AlertDescription, Button, Spinner, Switch } from "@compozy/ui";

import {
  SettingsGroup,
  SettingsPageFrame,
  SettingRow,
  useSettingsPalettePage,
  useSettingsTopbar,
} from "@/systems/settings";

export function PaletteSettingsPage() {
  const page = useSettingsPalettePage();
  useSettingsTopbar("palette");

  if (page.isLoading) {
    return (
      <div
        aria-label="Loading command palette settings"
        className="flex flex-1 items-center justify-center"
        data-testid="settings-page-palette-loading"
        role="status"
      >
        <Spinner aria-hidden="true" className="size-5 text-subtle" />
      </div>
    );
  }

  if (page.error || page.section === null) {
    return (
      <div
        className="flex flex-1 items-center justify-center"
        data-testid="settings-page-palette-error"
      >
        <div className="flex flex-col items-center gap-2 text-center">
          <AlertCircle aria-hidden="true" className="size-6 text-danger" />
          <p className="text-small-body text-subtle">
            {page.error?.message ?? "Failed to load command palette settings"}
          </p>
          <Button onClick={page.handleRetry} size="sm" type="button" variant="outline">
            Retry
          </Button>
        </div>
      </div>
    );
  }

  return (
    <SettingsPageFrame restart={page.restart} slug="palette">
      {page.saveError ? (
        <Alert data-testid="settings-palette-save-error" role="alert" variant="danger">
          <AlertDescription>{page.saveError}</AlertDescription>
        </Alert>
      ) : null}

      <SettingsGroup title="Personalization">
        <SettingRow
          control={
            <Switch
              aria-label="Personalization"
              checked={page.section.personalization}
              data-testid="settings-palette-personalization"
              disabled={page.isSaving}
              onCheckedChange={page.setPersonalization}
            />
          }
          // The one thing the label cannot say: turning this off is not a delete.
          description="Off stops recording; what was already learned is kept until you reset it."
          label="Learn from my usage"
        />
      </SettingsGroup>
    </SettingsPageFrame>
  );
}
