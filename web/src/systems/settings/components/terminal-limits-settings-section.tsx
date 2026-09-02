import { SettingRow } from "./setting-row";
import { SettingsGroup } from "./settings-group";
import { OptionalLimitInput } from "./terminal-settings-field-helpers";
import type { TerminalSettingsSectionProps } from "./terminal-settings-section-types";

export function TerminalLimitsSettingsSection({
  draft,
  patch,
  validationErrors,
  onValidationError,
}: TerminalSettingsSectionProps) {
  return (
    <SettingsGroup title="Limits">
      <SettingRow
        control={
          <OptionalLimitInput
            ariaLabel="Terminals per project"
            testId="settings-terminal-max-per-workspace"
            value={draft.max_per_workspace}
            onValueChange={value => {
              onValidationError?.("max_per_workspace", null);
              patch("max_per_workspace", value);
            }}
            onValidityChange={message => onValidationError?.("max_per_workspace", message)}
          />
        }
        data-testid="settings-terminal-max-per-workspace-row"
        error={validationErrors.max_per_workspace}
        label="Terminals per project"
      />
      <SettingRow
        control={
          <OptionalLimitInput
            ariaLabel="Terminals per installation"
            testId="settings-terminal-max-per-daemon"
            value={draft.max_per_daemon}
            onValueChange={value => {
              onValidationError?.("max_per_daemon", null);
              patch("max_per_daemon", value);
            }}
            onValidityChange={message => onValidationError?.("max_per_daemon", message)}
          />
        }
        data-testid="settings-terminal-max-per-daemon-row"
        error={validationErrors.max_per_daemon}
        label="Terminals per installation"
      />
      <SettingRow
        control={
          <OptionalLimitInput
            ariaLabel="Viewers per terminal"
            testId="settings-terminal-max-subscribers"
            value={draft.max_subscribers}
            onValueChange={value => {
              onValidationError?.("max_subscribers", null);
              patch("max_subscribers", value);
            }}
            onValidityChange={message => onValidationError?.("max_subscribers", message)}
          />
        }
        data-testid="settings-terminal-max-subscribers-row"
        error={validationErrors.max_subscribers}
        label="Viewers per terminal"
      />
    </SettingsGroup>
  );
}
