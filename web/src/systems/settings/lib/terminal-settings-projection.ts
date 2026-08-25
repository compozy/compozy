import type { TerminalSettingsConfig } from "../components/terminal-settings-sections";

/**
 * Reads the `[terminal]` block out of the general settings envelope.
 *
 * The daemon projects `config.toml` into this envelope section by section. When
 * the terminal block is absent the section does not render at all — an empty
 * form filled with the documented defaults would claim to show live values it
 * never read, and a person would edit numbers the runtime is not using.
 */
export function readTerminalSettings(config: unknown): TerminalSettingsConfig | null {
  const terminal = asRecord(asRecord(config).terminal);
  if (Object.keys(terminal).length === 0) return null;
  // Every one of the ten keys, with the type it is supposed to have. A missing
  // string is not an empty shell and a missing boolean is not `false`: both
  // would put a value the daemon never projected into an editable form, and the
  // next save would write it back as if a person had chosen it. An empty string
  // that *is* present stays valid — that is how "use the login shell" is
  // spelled.
  const projected = {
    default_shell: asString(terminal.default_shell),
    shell_integration: asBoolean(terminal.shell_integration),
    scrollback_bytes: asNumber(terminal.scrollback_bytes),
    detached_ttl: asString(terminal.detached_ttl),
    exit_retention: asString(terminal.exit_retention),
    recording: asBoolean(terminal.recording),
    recording_retention_days: asNumber(terminal.recording_retention_days),
    max_per_workspace: asNumber(terminal.max_per_workspace),
    max_per_daemon: asNumber(terminal.max_per_daemon),
    max_subscribers: asNumber(terminal.max_subscribers),
  };
  // A half-projected block is not a projection: rendering the keys that did
  // arrive would put a partial truth in front of an editable form.
  if (Object.values(projected).some(value => value === null)) return null;
  return projected as TerminalSettingsConfig;
}

function asRecord(value: unknown): Record<string, unknown> {
  return typeof value === "object" && value !== null ? (value as Record<string, unknown>) : {};
}

function asString(value: unknown): string | null {
  return typeof value === "string" ? value : null;
}

function asNumber(value: unknown): number | null {
  return typeof value === "number" && Number.isFinite(value) ? value : null;
}

function asBoolean(value: unknown): boolean | null {
  return typeof value === "boolean" ? value : null;
}
