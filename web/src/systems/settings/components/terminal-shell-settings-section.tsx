import { Input, Switch } from "@compozy/ui";

import { SettingRow } from "./setting-row";
import { SettingsGroup } from "./settings-group";
import { validateTerminalShell } from "./terminal-shell-validation";
import type { TerminalSettingsSectionProps } from "./terminal-settings-section-types";

export function TerminalShellSettingsSection({
  draft,
  patch,
  validationErrors,
  onValidationError,
}: TerminalSettingsSectionProps) {
  return (
    <SettingsGroup title="Shell">
      <SettingRow
        control={
          <Input
            aria-label="Default shell"
            data-testid="settings-terminal-default-shell"
            onChange={event => {
              const value = event.target.value;
              onValidationError?.("default_shell", validateTerminalShell(value));
              patch("default_shell", value);
            }}
            placeholder="Your login shell"
            value={draft.default_shell ?? ""}
          />
        }
        data-testid="settings-terminal-default-shell-row"
        error={validationErrors.default_shell}
        label="Default shell"
      />
      <SettingRow
        control={
          <Switch
            aria-invalid={validationErrors.shell_integration ? true : undefined}
            aria-label="Mark command boundaries"
            checked={draft.shell_integration === true}
            data-testid="settings-terminal-shell-integration"
            onCheckedChange={checked => {
              onValidationError?.("shell_integration", null);
              patch("shell_integration", checked);
            }}
          />
        }
        data-testid="settings-terminal-shell-integration-row"
        error={validationErrors.shell_integration}
        label="Mark command boundaries"
      />
    </SettingsGroup>
  );
}
