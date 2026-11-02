import { windowBinding } from "./agent-detail-search";
import type { AgentEditorPage, AgentProfilePanel } from "./agent-profile";

export const AGENT_SETTINGS_SECTIONS = [
  "basics",
  "runtime",
  "instructions",
  "access",
  "mcp",
] as const;

export type AgentSettingsSection = (typeof AGENT_SETTINGS_SECTIONS)[number];

/** Optional URL param — default `basics` via `resolveAgentSettingsSearch`. */
export interface AgentSettingsSearch {
  section?: AgentSettingsSection;
  /** The window's data binding, kept across in-window navigation. */
  workspace?: string;
}

export function validateAgentSettingsSearch(search: Record<string, unknown>): AgentSettingsSearch {
  // `danger` was its own section until Delete moved to the bottom of Basics; old links land there.
  const value = search.section === "danger" ? "basics" : search.section;
  const section =
    typeof value === "string" && (AGENT_SETTINGS_SECTIONS as readonly string[]).includes(value)
      ? (value as AgentSettingsSection)
      : "basics";
  return { section, ...windowBinding(search.workspace) };
}

export function resolveAgentSettingsSearch(search: AgentSettingsSearch): {
  section: AgentSettingsSection;
} {
  return { section: search.section ?? "basics" };
}

export type AgentLegacyDestination =
  | { kind: "home"; panel?: AgentProfilePanel }
  | { kind: "page"; page: AgentEditorPage };

/** Where an old `settings?section=` link lands now that the modal is gone. */
export function agentSettingsDestination(section: AgentSettingsSection): AgentLegacyDestination {
  switch (section) {
    case "runtime":
      return { kind: "home", panel: "model" };
    case "access":
    case "mcp":
      return { kind: "home", panel: "tools" };
    case "instructions":
      return { kind: "page", page: "what-it-does" };
    default:
      return { kind: "home" };
  }
}

/** Where an old `?tab=` link lands now that the tabs are gone. */
export function agentTabDestination(tab: unknown): AgentLegacyDestination {
  if (tab === "instructions") return { kind: "page", page: "what-it-does" };
  if (tab === "configuration") return { kind: "home", panel: "model" };
  return { kind: "home" };
}
