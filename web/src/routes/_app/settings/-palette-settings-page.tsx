import { useState } from "react";
import { AlertCircle } from "lucide-react";

import { Alert, AlertDescription, Button, ConfirmDialog, Spinner, Switch } from "@compozy/ui";

import {
  SettingsGroup,
  SettingsPageFrame,
  SettingRow,
  useSettingsPalettePage,
  useSettingsTopbar,
} from "@/systems/settings";

export function PaletteSettingsPage() {
  const page = useSettingsPalettePage();
  const [resetOpen, setResetOpen] = useState(false);
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
      {page.saveError || page.resetError ? (
        <Alert data-testid="settings-palette-save-error" role="alert" variant="danger">
          <AlertDescription>{page.saveError ?? page.resetError}</AlertDescription>
        </Alert>
      ) : null}

      <SettingsGroup title="Palette">
        <SettingRow
          control={
            <Switch
              aria-label="Agent fallback"
              checked={page.section.fallback_agent_enabled}
              data-testid="settings-palette-fallback-agent"
              disabled={page.isSaving}
              onCheckedChange={checked => page.setFallbackAgentEnabled(checked)}
            />
          }
          label="Agent fallback"
        />
        <SettingRow
          control={
            <Switch
              aria-label="Palette personalization"
              checked={page.section.personalization}
              data-testid="settings-palette-personalization"
              disabled={page.isSaving}
              onCheckedChange={checked => page.setPersonalization(checked)}
            />
          }
          label="Palette personalization"
        />
        <SettingRow
          control={
            <Button
              data-testid="settings-palette-reset"
              disabled={page.isResetting}
              size="sm"
              type="button"
              variant="outline"
              onClick={() => setResetOpen(true)}
            >
              Reset
            </Button>
          }
          description={page.scopeLabel}
          label="Reset palette personalization"
        />
      </SettingsGroup>

      <ConfirmDialog
        cancelLabel="Cancel"
        confirmLabel="Reset personalization"
        description={`This removes learned ranking and recents for ${page.scopeLabel}. Pins stay in place.`}
        error={page.resetError}
        isPending={page.isResetting}
        open={resetOpen}
        title="Reset palette personalization?"
        tone="warning"
        onConfirm={page.resetPersonalization}
        onOpenChange={setResetOpen}
      />
    </SettingsPageFrame>
  );
}
