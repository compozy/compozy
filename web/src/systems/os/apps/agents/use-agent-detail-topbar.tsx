import { useTopbarSlot } from "@compozy/ui";

import { agentStatusSlot } from "./agent-detail-view";
import type { UseAgentDetailResult } from "./use-agent-detail";
import {
  AgentPageActions,
  AgentPageOverflow,
  AgentPageOverflowSkeleton,
  AgentPageStatusPill,
  AgentPageStatusSkeleton,
} from "@/systems/agent";

/**
 * Publishes the agent detail crumbs, status, and actions into the window topbar. The back arrow,
 * the name and New session come from the route, so they are there while the agent loads; the
 * overflow and the status follow the agent and its metrics, and hold their places until then.
 */
export function useAgentDetailTopbar(page: UseAgentDetailResult, name: string) {
  const navigation = {
    onBack: () => page.onBackToAgents(),
    crumbs: [{ id: "agents", label: "Agents", onSelect: () => page.onBackToAgents() }],
    crumb: <span data-testid="agent-detail-header-name">{name}</span>,
  };
  // The status follows the metrics: a placeholder holds its place while they load, nothing when
  // they are unavailable.
  const statusSlot = agentStatusSlot(page);
  const status =
    statusSlot === "pill" ? (
      <AgentPageStatusPill activeCount={page.activeSessionsTotal} />
    ) : statusSlot === "placeholder" ? (
      <AgentPageStatusSkeleton />
    ) : undefined;
  const actions = (
    <AgentPageActions
      onNewSession={page.onNewSession}
      isCreatingSession={page.isCreatingForAgent}
      newSessionDisabled={page.newSessionDisabled}
    />
  );
  const loaded = page.agent
    ? {
        ...navigation,
        status,
        actions,
        overflow: (
          <AgentPageOverflow
            onDelete={page.onDelete}
            onDuplicate={page.onDuplicate}
            onEditSettings={() => page.onEditSettings()}
          />
        ),
      }
    : null;
  const loading = { ...navigation, status, actions, overflow: <AgentPageOverflowSkeleton /> };
  useTopbarSlot(loaded ?? (page.agentLoading ? loading : navigation));
}
