import type { Dispatch, SetStateAction } from "react";

import { Input, Switch } from "@compozy/ui";

import { SettingRow } from "./setting-row";
import { SettingsByteField } from "./settings-byte-field";
import { SettingsGroup } from "./settings-group";
import { SettingsNumberInput } from "./settings-number-input";

/**
 * The ten `[terminal]` keys, exactly as `config.toml` declares them.
 *
 * Agent-autonomy policy is deliberately absent: the allowlist and its tiers live
 * with the permissions surfaces where all agent-approval policy lives, and
 * projecting them here would create a second policy editor.
 */
export interface TerminalSettingsConfig {
  default_shell: string;
  shell_integration: boolean;
  scrollback_bytes: number;
  detached_ttl: string;
  exit_retention: string;
  recording: boolean;
  recording_retention_days: number;
  max_per_workspace: number;
  max_per_daemon: number;
  max_subscribers: number;
}

export interface TerminalSettingsSectionsProps {
  draft: TerminalSettingsConfig;
  setDraft: Dispatch<SetStateAction<TerminalSettingsConfig | null>>;
  /** Per-key refusals from config validation, keyed by the TOML key. */
  validationErrors: Record<string, string | null>;
}

export function TerminalSettingsSections({
  draft,
  setDraft,
  validationErrors,
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
              onChange={event => patch("default_shell", event.target.value)}
              placeholder="Your login shell"
              value={draft.default_shell}
            />
          }
          data-testid="settings-terminal-default-shell-row"
          error={validationErrors.default_shell}
          help="An unavailable shell falls back down the chain, and the terminal states which one actually started."
          label="Default shell"
        />
        <SettingRow
          control={
            <Switch
              aria-label="Mark command boundaries"
              checked={draft.shell_integration}
              data-testid="settings-terminal-shell-integration"
              onCheckedChange={checked => patch("shell_integration", checked)}
            />
          }
          data-testid="settings-terminal-shell-integration-row"
          error={validationErrors.shell_integration}
          help="Off, journal rows record when a command was noticed rather than when it started, and are marked estimated."
          label="Mark command boundaries"
        />
      </SettingsGroup>

      <SettingsGroup title="Retention">
        <SettingRow
          control={
            <SettingsByteField
              data-testid="settings-terminal-scrollback"
              label="Scrollback kept per terminal"
              onChange={value => patch("scrollback_bytes", value)}
              value={draft.scrollback_bytes}
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
              onChange={event => patch("detached_ttl", event.target.value)}
              value={draft.detached_ttl}
            />
          }
          data-testid="settings-terminal-detached-ttl-row"
          error={validationErrors.detached_ttl}
          help="Activity resets the clock: output or a viewer attaching both count."
          label="Reclaim idle terminals after"
        />
        <SettingRow
          control={
            <Input
              aria-label="Keep exited terminals readable for"
              data-testid="settings-terminal-exit-retention"
              onChange={event => patch("exit_retention", event.target.value)}
              value={draft.exit_retention}
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
              aria-label="Record every terminal"
              checked={draft.recording}
              data-testid="settings-terminal-recording"
              onCheckedChange={checked => patch("recording", checked)}
            />
          }
          data-testid="settings-terminal-recording-row"
          error={validationErrors.recording}
          label="Record every terminal"
        />
        <SettingRow
          control={
            <SettingsNumberInput
              aria-label="Keep recordings for, in days"
              data-testid="settings-terminal-recording-retention"
              min={1}
              onValueChange={value => patch("recording_retention_days", value)}
              value={draft.recording_retention_days}
            />
          }
          data-testid="settings-terminal-recording-retention-row"
          error={validationErrors.recording_retention_days}
          label="Keep recordings for"
        />
      </SettingsGroup>

      <SettingsGroup title="Limits">
        <SettingRow
          control={
            <SettingsNumberInput
              aria-label="Terminals per project"
              data-testid="settings-terminal-max-per-workspace"
              min={1}
              onValueChange={value => patch("max_per_workspace", value)}
              value={draft.max_per_workspace}
            />
          }
          data-testid="settings-terminal-max-per-workspace-row"
          error={validationErrors.max_per_workspace}
          help="Counted per profile, so another profile's terminals never consume this budget."
          label="Terminals per project"
        />
        <SettingRow
          control={
            <SettingsNumberInput
              aria-label="Terminals per installation"
              data-testid="settings-terminal-max-per-daemon"
              min={1}
              onValueChange={value => patch("max_per_daemon", value)}
              value={draft.max_per_daemon}
            />
          }
          data-testid="settings-terminal-max-per-daemon-row"
          error={validationErrors.max_per_daemon}
          help="Counted across every project and profile on this machine, so it can be reached before a project's own limit is."
          label="Terminals per installation"
        />
        <SettingRow
          control={
            <SettingsNumberInput
              aria-label="Viewers per terminal"
              data-testid="settings-terminal-max-subscribers"
              min={1}
              onValueChange={value => patch("max_subscribers", value)}
              value={draft.max_subscribers}
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
