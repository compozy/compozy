/** S2: the Agents window's body. A team to meet first, a catalog to search second. */
import { Users2 } from "@compozy/ui/icons";

import { Button, Empty, type ListingViewMode } from "@compozy/ui";

import { agentStaysOn, agentsByAttention, type AgentTeamMember } from "../../lib/agent-team";
import { AgentFleetSkeleton } from "../agent-fleet-skeleton";
import type { AgentTeamActions } from "./agent-team-card";
import { AgentStaysOnWelcome, AgentTeamSection } from "./agent-team-section";

const ON_DEMAND_PAGE = 12;

export type AgentTeamStatus = "loading" | "first-run-empty" | "filtered-empty" | "ready";

export interface AgentTeamPageProps {
  status: AgentTeamStatus;
  members: readonly AgentTeamMember[];
  view: ListingViewMode;
  /** True when no search, filter or tab narrows the page. */
  unfiltered: boolean;
  actions: AgentTeamActions;
  onCreate: (options?: { staysOn?: boolean }) => void;
  onClearFilters: () => void;
}

export function AgentTeamPage({
  status,
  members,
  view,
  unfiltered,
  actions,
  onCreate,
  onClearFilters,
}: AgentTeamPageProps) {
  if (status === "loading") return <AgentFleetSkeleton view={view} />;
  if (status === "first-run-empty") {
    return (
      <div
        className="flex min-h-0 flex-1 items-center justify-center px-6 py-10"
        data-testid="agent-fleet-empty"
      >
        <Empty
          action={
            <Button
              data-testid="agent-fleet-empty-create"
              onClick={() => onCreate()}
              size="sm"
              type="button"
            >
              New agent
            </Button>
          }
          description="An agent is an assistant with its own instructions and model. Create one to start working with it."
          icon={Users2}
          title="No agents yet"
        />
      </div>
    );
  }
  if (status === "filtered-empty") {
    return (
      <div
        className="flex min-h-0 flex-1 items-center justify-center px-6 py-10"
        data-testid="agent-fleet-filtered-empty"
      >
        <Empty
          action={
            <Button
              data-testid="agent-fleet-clear-filters"
              onClick={onClearFilters}
              size="sm"
              type="button"
              variant="ghost"
            >
              Clear filters
            </Button>
          }
          description="Try a different search or clear the active filters."
          icon={Users2}
          title="No agents match"
        />
      </div>
    );
  }
  const staysOn = members.filter(agentStaysOn).sort(agentsByAttention);
  const onDemand = members.filter(member => !agentStaysOn(member));
  return (
    <div className="flex flex-col gap-8" data-testid="agent-fleet-list">
      {staysOn.length > 0 ? (
        <AgentTeamSection actions={actions} label="Stays on" members={staysOn} view={view} />
      ) : unfiltered ? (
        <AgentStaysOnWelcome onCreate={() => onCreate({ staysOn: true })} />
      ) : null}
      {onDemand.length > 0 ? (
        <AgentTeamSection
          actions={actions}
          label="On demand"
          members={onDemand}
          pageSize={ON_DEMAND_PAGE}
          view={view}
        />
      ) : null}
    </div>
  );
}
