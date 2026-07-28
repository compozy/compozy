import { isAgentSessionFailure } from "./session-status";
import type { SessionPayload } from "@/systems/session";

export const AGENT_DETAIL_TABS = ["overview", "instructions", "configuration", "sessions"] as const;
export type AgentDetailTab = (typeof AGENT_DETAIL_TABS)[number];

export const AGENT_INSTRUCTION_FILES = ["agent", "soul", "heartbeat"] as const;
export type AgentInstructionFile = (typeof AGENT_INSTRUCTION_FILES)[number];

export const AGENT_SESSION_FILTERS = ["all", "active", "failed", "done"] as const;
export type AgentSessionFilter = (typeof AGENT_SESSION_FILTERS)[number];

/** Optional URL params — defaults applied via `resolveAgentDetailSearch`. */
export interface AgentDetailSearch {
  tab?: AgentDetailTab;
  file?: AgentInstructionFile;
  filter?: AgentSessionFilter;
}

export interface ResolvedAgentDetailSearch {
  tab: AgentDetailTab;
  file: AgentInstructionFile;
  filter: AgentSessionFilter;
}

function parseEnum<T extends string>(value: unknown, allowed: readonly T[], fallback: T): T {
  return typeof value === "string" && (allowed as readonly string[]).includes(value)
    ? (value as T)
    : fallback;
}

export function validateAgentDetailSearch(search: Record<string, unknown>): AgentDetailSearch {
  return {
    tab: parseEnum(search.tab, AGENT_DETAIL_TABS, "overview"),
    file: parseEnum(search.file, AGENT_INSTRUCTION_FILES, "agent"),
    filter: parseEnum(search.filter, AGENT_SESSION_FILTERS, "all"),
  };
}

export function resolveAgentDetailSearch(search: AgentDetailSearch): ResolvedAgentDetailSearch {
  return {
    tab: search.tab ?? "overview",
    file: search.file ?? "agent",
    filter: search.filter ?? "all",
  };
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
