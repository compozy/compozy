import { useTopbarSlot } from "@compozy/ui";

import { agentMetricsReady } from "./agent-detail-view";
import type { UseAgentDetailResult } from "./use-agent-detail";
import { AgentPageActions, AgentPageOverflow, AgentPageStatusPill } from "@/systems/agent";

/** Publishes the agent detail crumbs, status, and actions into the window topbar. */
export function useAgentDetailTopbar(page: UseAgentDetailResult, name: string) {
  const metricsReady = agentMetricsReady(page);
  useTopbarSlot(
    page.agent
      ? {
          onBack: () => page.onBackToAgents(),
          crumbs: [{ id: "agents", label: "Agents", onSelect: () => page.onBackToAgents() }],
          crumb: <span data-testid="agent-detail-header-name">{name}</span>,
          status: metricsReady ? (
            <AgentPageStatusPill activeCount={page.activeSessionsTotal} />
          ) : undefined,
          actions: (
            <AgentPageActions
              onNewSession={page.onNewSession}
              isCreatingSession={page.isCreatingForAgent}
              newSessionDisabled={page.newSessionDisabled}
            />
          ),
          overflow: (
            <AgentPageOverflow
              onDelete={page.onDelete}
              onDuplicate={page.onDuplicate}
              onEditSettings={() => page.onEditSettings()}
            />
          ),
        }
      : null
  );
}
