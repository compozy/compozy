import { useNavigate } from "@tanstack/react-router";

import {
  useAgent,
  useAgentCatalogMetrics,
  useAgentCreateHost,
  useAgentDeleteFlow,
  useAgentWindowScope,
  type AgentPayload,
} from "@/systems/agent";
import { useActiveWorkspace } from "@/systems/workspace";

export interface UseAgentDetailResult {
  agent: AgentPayload | undefined;
  agentLoading: boolean;
  agentError: Error | null;
  /** Sessions running now, from exact catalog metrics; zero until they answer. */
  active: number;
  onDuplicate: () => void;
  onDelete: () => void;
  onBackToAgents: () => void;
  onBackToAgent: () => void;
  deleteDialog: ReturnType<typeof useAgentDeleteFlow>["confirmDialog"];
}

/** What every agent location reads: the definition, its overflow actions and the way back. */
export function useAgentDetail(name: string): UseAgentDetailResult {
  const navigate = useNavigate();
  const { pending } = useActiveWorkspace();
  const { workspace, projectId, bindingSearch } = useAgentWindowScope();
  const createHost = useAgentCreateHost();
  const { data: agent, isLoading: agentLoading, error: agentError } = useAgent(name, workspace);
  const metrics = useAgentCatalogMetrics(workspace, name, { enabled: !pending });
  const deleteFlow = useAgentDeleteFlow({ agent, workspaceId: projectId });

  return {
    agent,
    agentLoading,
    agentError: (agentError as Error | null) ?? null,
    active: metrics.active,
    onDuplicate: () => {
      if (agent) createHost.openForDuplicate(agent);
    },
    onDelete: deleteFlow.openDialog,
    onBackToAgents: () => void navigate({ to: "/agents" }),
    onBackToAgent: () =>
      void navigate({ to: "/agents/$name", params: { name }, search: bindingSearch }),
    deleteDialog: deleteFlow.confirmDialog,
  };
}
