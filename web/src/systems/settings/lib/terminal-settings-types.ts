/** The ten `[terminal]` keys projected from `config.toml`. */
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
