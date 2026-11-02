import { use } from "react";
import {
  GLOBAL_SCOPE,
  useAgents,
  AgentIdentityScopeContext,
  type AgentPayload,
} from "@/systems/agent";

import type { SessionMentionSourceStatus } from "./use-session-file-search";

/** The picker's "Agents" group shows at most five rows, as "Sessions" does. */
export const SESSION_MENTION_AGENT_LIMIT = 5;

export interface SessionMentionAgentsView {
  agents: readonly AgentPayload[];
  status: SessionMentionSourceStatus;
}

function agentMatches(agent: AgentPayload, needle: string): boolean {
  return [agent.name, agent.label, agent.description ?? ""].some(text =>
    text.toLowerCase().includes(needle)
  );
}

/** The `@` picker's agents, from the same cached catalog every identity read uses. */
export function useSessionMentionAgents({
  workspaceId,
  query,
  enabled,
}: {
  workspaceId: string;
  query: string;
  enabled: boolean;
}): SessionMentionAgentsView {
  const scope = use(AgentIdentityScopeContext);
  const catalog = useAgents(workspaceId === GLOBAL_SCOPE ? null : workspaceId, {
    enabled: enabled && scope.enabled !== false,
    profile: scope.profile ?? undefined,
  });
  const needle = query.trim().toLowerCase();
  const agents = (catalog.data ?? [])
    .filter(agent => needle === "" || agentMatches(agent, needle))
    .slice(0, SESSION_MENTION_AGENT_LIMIT);
  const status: SessionMentionSourceStatus = catalog.isError
    ? "error"
    : catalog.data === undefined
      ? "loading"
      : "ready";
  return { agents, status };
}
