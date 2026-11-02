import type { ReactNode } from "react";

import { useTopbarSlot } from "@compozy/ui";

import { AgentProfileOverflow, humanizeAgentSlug } from "@/systems/agent";

import type { UseAgentDetailResult } from "./use-agent-detail";

/**
 * One location of the Agents window: `Agents / Orchestrator / Check-ins`. A topbar has
 * one publisher, so a page that adds `actions` publishes alone and its loading shell
 * passes `publish: false` once that page is mounted.
 */
export function useAgentDetailTopbar(
  page: UseAgentDetailResult,
  name: string,
  leaf?: string,
  { actions, publish = true }: { actions?: ReactNode; publish?: boolean } = {}
) {
  const label = page.agent?.label ?? humanizeAgentSlug(name);
  const agents = { id: "agents", label: "Agents", onSelect: page.onBackToAgents };
  const slot = {
    onBack: leaf ? page.onBackToAgent : page.onBackToAgents,
    crumbs: leaf ? [agents, { id: name, label, onSelect: page.onBackToAgent }] : [agents],
    crumb: <span data-testid="agent-detail-header-name">{leaf ?? label}</span>,
    overflow:
      !leaf && page.agent ? (
        <AgentProfileOverflow onDelete={page.onDelete} onDuplicate={page.onDuplicate} />
      ) : undefined,
    actions,
  };
  useTopbarSlot(publish ? slot : null);
}
