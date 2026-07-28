import { useAgentCatalog } from "@/systems/agent";
import { useActiveWorkspace } from "@/systems/workspace";

export interface HomeAgentRow {
  name: string;
  active: number;
  sessions: number;
  failed: number;
  runtimeSeconds: number;
  runtimeShare: number;
}

export interface HomeAgentsModel {
  rows: HomeAgentRow[];
  workspaceId: string;
}

const HOME_AGENT_LIMIT = 6;

export function useHomeAgents(): HomeAgentsModel {
  const { activeWorkspaceId } = useActiveWorkspace();
  const workspaceId = activeWorkspaceId ?? "";
  const catalogQuery = useAgentCatalog(workspaceId, { limit: HOME_AGENT_LIMIT });

  const agents = catalogQuery.agents;
  const maxRuntime = agents.reduce(
    (max, item) => Math.max(max, item.sessions?.runtime_seconds ?? 0),
    0
  );

  const rows: HomeAgentRow[] = agents.map(item => {
    const runtimeSeconds = item.sessions?.runtime_seconds ?? 0;
    return {
      name: item.agent.name,
      active: item.sessions?.active ?? 0,
      sessions: item.sessions?.total ?? 0,
      failed: item.sessions?.failed ?? 0,
      runtimeSeconds,
      runtimeShare: maxRuntime > 0 ? runtimeSeconds / maxRuntime : 0,
    };
  });

  return { rows, workspaceId };
}
