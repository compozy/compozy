import { useState } from "react";

import { Alert, AlertDescription, Button, ConfirmDialog, Switch } from "@compozy/ui";

import {
  SettingsGroup,
  SettingsPageFrame,
  SettingsPageState,
  SettingRow,
  useSettingsPalettePage,
  useSettingsTopbar,
} from "@/systems/settings";

export function PaletteSettingsPage() {
  const page = useSettingsPalettePage();
  const [resetOpen, setResetOpen] = useState(false);
  useSettingsTopbar();

  if (page.isLoading) {
    return <SettingsPageState slug="palette" state="loading" />;
  }

  if (page.error || page.section === null) {
    return (
      <SettingsPageState
        error={page.error}
        onRetry={page.handleRetry}
        slug="palette"
        state="error"
      />
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
              disabled={!page.canResetPersonalization || page.isResetting}
              size="sm"
              type="button"
              variant="secondary"
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
        onConfirm={async () => {
          try {
            await page.resetPersonalization();
          } catch {
            return;
          }
          setResetOpen(false);
        }}
        onOpenChange={setResetOpen}
      />
    </SettingsPageFrame>
  );
}
