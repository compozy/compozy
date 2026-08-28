import type { Dispatch, SetStateAction } from "react";

import { Input, Switch } from "@compozy/ui";

import { SettingRow } from "./setting-row";
import { SettingsByteField } from "./settings-byte-field";
import { SettingsGroup } from "./settings-group";
import { SettingsNumberInput } from "./settings-number-input";
import type { TerminalSettingsConfig } from "../lib/terminal-settings-types";
import { validatePositiveDuration } from "../lib/terminal-settings-duration";

export type { TerminalSettingsConfig } from "../lib/terminal-settings-types";

/**
 * The ten `[terminal]` keys, exactly as `config.toml` declares them.
 *
 * Agent-autonomy policy is deliberately absent: the allowlist and its tiers live
 * with the permissions surfaces where all agent-approval policy lives, and
 * projecting them here would create a second policy editor.
 */
export interface TerminalSettingsSectionsProps {
  draft: Partial<TerminalSettingsConfig>;
  setDraft: Dispatch<SetStateAction<Partial<TerminalSettingsConfig> | null>>;
  /** Per-key refusals from config validation, keyed by the TOML key. */
  validationErrors: Record<string, string | null>;
  onValidationError?: (key: keyof TerminalSettingsConfig, message: string | null) => void;
}

export function TerminalSettingsSections({
  draft,
  setDraft,
  validationErrors,
  onValidationError,
}: TerminalSettingsSectionsProps) {
  const patch = <K extends keyof TerminalSettingsConfig>(
    key: K,
    value: TerminalSettingsConfig[K]
  ) => {
    setDraft(current => (current === null ? current : { ...current, [key]: value }));
  };

  return (
    <>
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
    </>
  );
}

function OptionalLimitInput({
  ariaLabel,
  testId,
  value,
  onValueChange,
  onValidityChange,
}: {
  ariaLabel: string;
  testId: string;
  value: number | undefined;
  onValueChange: (value: number) => void;
  onValidityChange: (message: string | null) => void;
}) {
  if (value === undefined) {
    return (
      <Input
        aria-invalid
        aria-label={ariaLabel}
        className="w-28 text-right font-mono"
        data-testid={testId}
        inputMode="numeric"
        onChange={event => {
          const parsed = Number.parseInt(event.target.value, 10);
          if (Number.isSafeInteger(parsed) && parsed >= 1) {
            onValidityChange(null);
            onValueChange(parsed);
          }
        }}
        value=""
      />
    );
  }
  return (
    <SettingsNumberInput
      aria-label={ariaLabel}
      className="w-28 text-right font-mono"
      data-testid={testId}
      min={1}
      onValidityChange={onValidityChange}
      onValueChange={onValueChange}
      value={value}
    />
  );
}

function validateTerminalShell(value: string): string | null {
  if (value === "") return null;
  const trimmed = value.trim();
  const containsSeparator = value.includes("/") || value.includes("\\");
  const absolute =
    value.startsWith("/") || /^[A-Za-z]:[\\/]/.test(value) || value.startsWith("\\\\");
  if (trimmed !== value || value.includes("\0") || (containsSeparator && !absolute)) {
    return "Enter a command name or an absolute path.";
  }
  return null;
}
