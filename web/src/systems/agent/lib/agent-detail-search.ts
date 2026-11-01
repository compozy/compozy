import { AGENT_PROFILE_PANELS, type AgentProfilePanel } from "./agent-profile";
import { isAgentSessionFailure } from "./session-status";

import type { SessionPayload } from "@/systems/session";

export const AGENT_SESSION_FILTERS = ["all", "active", "failed", "done"] as const;
export type AgentSessionFilter = (typeof AGENT_SESSION_FILTERS)[number];

/** Optional URL params of the agent's home. */
export interface AgentDetailSearch {
  /** The side panel open over the home. */
  panel?: AgentProfilePanel;
  /** The window's data binding (a project id or `@user`), kept across in-window navigation. */
  workspace?: string;
}

function parsePanel(value: unknown): AgentProfilePanel | undefined {
  return AGENT_PROFILE_PANELS.find(panel => panel === value);
}

export function validateAgentDetailSearch(search: Record<string, unknown>): AgentDetailSearch {
  const panel = parsePanel(search.panel);
  return { ...(panel ? { panel } : {}), ...windowBinding(search.workspace) };
}

/** A non-empty `workspace` binding survives validation; anything else drops. */
export function windowBinding(value: unknown): { workspace?: string } {
  return typeof value === "string" && value.trim() !== "" ? { workspace: value.trim() } : {};
}

export function filterAgentSessionsByStatus(
  sessions: readonly SessionPayload[],
  filter: AgentSessionFilter
): SessionPayload[] {
  switch (filter) {
    case "active":
      return sessions.filter(session => session.state === "active");
    case "failed":
      return sessions.filter(session => isAgentSessionFailure(session));
    case "done":
      return sessions.filter(
        session => session.state === "stopped" && !isAgentSessionFailure(session)
      );
    case "all":
    default:
      return [...sessions];
  }
}
