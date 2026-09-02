import { Input, Switch } from "@compozy/ui";

import { SettingRow } from "./setting-row";
import { SettingsGroup } from "./settings-group";
import { SettingsNumberInput } from "./settings-number-input";
import type { TerminalSettingsSectionProps } from "./terminal-settings-section-types";

export function TerminalRecordingSettingsSection({
  draft,
  patch,
  validationErrors,
  onValidationError,
}: TerminalSettingsSectionProps) {
  return (
    <SettingsGroup title="Recording">
      <SettingRow
        control={
          <Switch
            aria-invalid={validationErrors.recording ? true : undefined}
            aria-label="Record every terminal"
            checked={draft.recording === true}
            data-testid="settings-terminal-recording"
            onCheckedChange={checked => {
              onValidationError?.("recording", null);
              patch("recording", checked);
            }}
          />
        }
        data-testid="settings-terminal-recording-row"
        error={validationErrors.recording}
        label="Record every terminal"
      />
      <SettingRow
        control={
          <span className="flex items-center gap-2">
            {draft.recording_retention_days === undefined ? (
              <Input
                aria-invalid
                aria-label="Keep recordings for, in days"
                className="w-28 text-right font-mono"
                data-testid="settings-terminal-recording-retention"
                inputMode="numeric"
                onChange={event => {
                  const parsed = Number.parseInt(event.target.value, 10);
                  if (Number.isSafeInteger(parsed) && parsed >= 1) {
                    onValidationError?.("recording_retention_days", null);
                    patch("recording_retention_days", parsed);
                  }
                }}
                value=""
              />
            ) : (
              <SettingsNumberInput
                aria-label="Keep recordings for, in days"
                className="w-28 text-right font-mono"
                data-testid="settings-terminal-recording-retention"
                min={1}
                onValidityChange={message =>
                  onValidationError?.("recording_retention_days", message)
                }
                onValueChange={value => patch("recording_retention_days", value)}
                value={draft.recording_retention_days}
              />
            )}
            <span className="text-form-label text-subtle">days</span>
          </span>
        }
        data-testid="settings-terminal-recording-retention-row"
        error={validationErrors.recording_retention_days}
        label="Keep recordings for"
      />
    </SettingsGroup>
  );
}
