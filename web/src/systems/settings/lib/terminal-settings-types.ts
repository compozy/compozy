/** The ten `[terminal]` keys projected from `config.toml`. */
export const TERMINAL_SETTINGS_KEYS = [
  "default_shell",
  "shell_integration",
  "scrollback_bytes",
  "detached_ttl",
  "exit_retention",
  "recording",
  "recording_retention_days",
  "max_per_workspace",
  "max_per_daemon",
  "max_subscribers",
] as const;

export type TerminalSettingsKey = (typeof TERMINAL_SETTINGS_KEYS)[number];

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

export type TerminalSettingsProjection =
  | {
      status: "absent";
      values: Partial<TerminalSettingsConfig>;
      invalidKeys: readonly TerminalSettingsKey[];
    }
  | {
      status: "ready";
      values: Partial<TerminalSettingsConfig>;
      invalidKeys: readonly TerminalSettingsKey[];
    };

export function terminalSettingsInvalidKeyMessage(key: TerminalSettingsKey): string {
  return `${key} is missing or invalid`;
}
