export const AGENT_SETTINGS_SECTIONS = [
  "basics",
  "runtime",
  "instructions",
  "access",
  "mcp",
  "danger",
] as const;

export type AgentSettingsSection = (typeof AGENT_SETTINGS_SECTIONS)[number];

/** Optional URL param — default `basics` via `resolveAgentSettingsSearch`. */
export interface AgentSettingsSearch {
  section?: AgentSettingsSection;
}

export function validateAgentSettingsSearch(search: Record<string, unknown>): AgentSettingsSearch {
  const value = search.section;
  const section =
    typeof value === "string" && (AGENT_SETTINGS_SECTIONS as readonly string[]).includes(value)
      ? (value as AgentSettingsSection)
      : "basics";
  return { section };
}

export function resolveAgentSettingsSearch(search: AgentSettingsSearch): {
  section: AgentSettingsSection;
} {
  return { section: search.section ?? "basics" };
}
