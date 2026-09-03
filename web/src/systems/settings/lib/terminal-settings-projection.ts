import {
  TERMINAL_SETTINGS_KEYS,
  terminalSettingsInvalidKeyMessage,
  type TerminalSettingsConfig,
  type TerminalSettingsKey,
  type TerminalSettingsProjection,
} from "./terminal-settings-types";

export type { TerminalSettingsProjection } from "./terminal-settings-types";
export { terminalSettingsInvalidKeyMessage };

/**
 * Reads the `[terminal]` block out of the general settings envelope.
 *
 * The daemon projects `config.toml` into this envelope section by section. When
 * the terminal block is absent the form does not render — an empty form filled
 * with documented defaults would claim to show live values it never read. When
 * the block exists but a key is missing or the wrong type, the section stays
 * visible and the refusal names that key.
 */
export function readTerminalSettings(config: unknown): TerminalSettingsProjection {
  const terminal = asRecord(asRecord(config).terminal);
  if (Object.keys(terminal).length === 0) {
    return { status: "absent", values: {}, invalidKeys: [] };
  }
  const values: Partial<TerminalSettingsConfig> = {};
  const invalidKeys: TerminalSettingsKey[] = [];
  for (const key of TERMINAL_SETTINGS_KEYS) {
    const parsed = parseTerminalSetting(key, terminal[key]);
    if (parsed === undefined) {
      invalidKeys.push(key);
    } else {
      assignTerminalSetting(values, key, parsed);
    }
  }
  return { status: "ready", values, invalidKeys };
}

function parseTerminalSetting(
  key: TerminalSettingsKey,
  value: unknown
): TerminalSettingsConfig[TerminalSettingsKey] | undefined {
  switch (key) {
    case "default_shell":
    case "detached_ttl":
    case "exit_retention":
      return typeof value === "string" ? value : undefined;
    case "shell_integration":
    case "recording":
      return typeof value === "boolean" ? value : undefined;
    case "scrollback_bytes":
    case "recording_retention_days":
    case "max_per_workspace":
    case "max_per_daemon":
    case "max_subscribers":
      return typeof value === "number" && Number.isFinite(value) ? value : undefined;
  }
}

function assignTerminalSetting<K extends TerminalSettingsKey>(
  values: Partial<TerminalSettingsConfig>,
  key: K,
  value: TerminalSettingsConfig[K]
): void {
  values[key] = value;
}

function asRecord(value: unknown): Record<string, unknown> {
  return typeof value === "object" && value !== null ? (value as Record<string, unknown>) : {};
}
