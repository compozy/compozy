import { useTopbarSlot } from "@compozy/ui";

import { AgentProfileOverflow, humanizeAgentSlug } from "@/systems/agent";

import type { UseAgentDetailResult } from "./use-agent-detail";

/** One location of the Agents window: `Agents / Orchestrator / Check-ins`. */
export function useAgentDetailTopbar(page: UseAgentDetailResult, name: string, leaf?: string) {
  const label = page.agent?.label ?? humanizeAgentSlug(name);
  const agents = { id: "agents", label: "Agents", onSelect: page.onBackToAgents };
  useTopbarSlot({
    onBack: leaf ? page.onBackToAgent : page.onBackToAgents,
    crumbs: leaf ? [agents, { id: name, label, onSelect: page.onBackToAgent }] : [agents],
    crumb: <span data-testid="agent-detail-header-name">{leaf ?? label}</span>,
    overflow:
      !leaf && page.agent ? (
        <AgentProfileOverflow onDelete={page.onDelete} onDuplicate={page.onDuplicate} />
      ) : undefined,
  });
}
