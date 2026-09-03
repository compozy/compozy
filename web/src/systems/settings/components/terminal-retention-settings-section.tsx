import { Input } from "@compozy/ui";

import { SettingRow } from "./setting-row";
import { SettingsByteField } from "./settings-byte-field";
import { SettingsGroup } from "./settings-group";
import type { TerminalSettingsSectionProps } from "./terminal-settings-section-types";
import { validatePositiveDuration } from "../lib/terminal-settings-duration";

export function TerminalRetentionSettingsSection({
  draft,
  patch,
  validationErrors,
  onValidationError,
}: TerminalSettingsSectionProps) {
  return (
    <SettingsGroup title="Retention">
      <SettingRow
        control={
          <SettingsByteField
            data-testid="settings-terminal-scrollback"
            label="Scrollback kept per terminal"
            minBytes={1}
            onChange={value => {
              onValidationError?.("scrollback_bytes", null);
              patch("scrollback_bytes", value);
            }}
            onValidityChange={message => onValidationError?.("scrollback_bytes", message)}
            value={draft.scrollback_bytes ?? 0}
          />
        }
        data-testid="settings-terminal-scrollback-row"
        error={validationErrors.scrollback_bytes}
        label="Scrollback kept per terminal"
      />
      <SettingRow
        control={
          <Input
            aria-label="Reclaim idle terminals after"
            data-testid="settings-terminal-detached-ttl"
            onChange={event => {
              const value = event.target.value;
              onValidationError?.("detached_ttl", validatePositiveDuration(value));
              patch("detached_ttl", value);
            }}
            placeholder="e.g. 24h"
            value={draft.detached_ttl ?? ""}
          />
        }
        data-testid="settings-terminal-detached-ttl-row"
        error={validationErrors.detached_ttl}
        label="Reclaim idle terminals after"
      />
      <SettingRow
        control={
          <Input
            aria-label="Keep exited terminals readable for"
            data-testid="settings-terminal-exit-retention"
            onChange={event => {
              const value = event.target.value;
              onValidationError?.("exit_retention", validatePositiveDuration(value));
              patch("exit_retention", value);
            }}
            placeholder="e.g. 15m"
            value={draft.exit_retention ?? ""}
          />
        }
        data-testid="settings-terminal-exit-retention-row"
        error={validationErrors.exit_retention}
        label="Keep exited terminals readable for"
      />
    </SettingsGroup>
  );
}
