/** The two ways to meet a teammate: a big stays-on card and a lighter on-demand card. */
import { CatalogCard, Sparkline } from "@compozy/ui";

import { AgentFleetNewSessionButton } from "@/systems/agent/components/agent-fleet-new-session-button";

import { AgentAvatar } from "../stays-on-agents-shared/agent-avatar";
import type { AddScopeState, ListedAgent } from "../stays-on-agents-shared/agents-page";
import { overallState } from "./team-filters";
import { openProfile } from "./team-now";
import { AddPlaceButton, AgentFlags, NowLine, PlaceChips } from "./team-parts";

/** The name is the card's link; its hit area stretches over the card, under the controls. */
function CardName({ agent, large = false }: { agent: ListedAgent; large?: boolean }) {
  return (
    <button
      className={`min-w-0 text-left outline-none after:absolute after:inset-0 after:rounded-lg focus-visible:after:shadow-focus-ring ${large ? "text-heading" : ""}`}
      onClick={() => openProfile(agent)}
      type="button"
    >
      <CatalogCard.Title className="line-clamp-1 whitespace-normal">
        {agent.label}
      </CatalogCard.Title>
    </button>
  );
}

function Pulse({ agent }: { agent: ListedAgent }) {
  const values = agent.activity ?? [];
  if (values.length === 0) {
    return <div className="flex h-7 items-end text-eyebrow text-subtle">No activity yet</div>;
  }
  return (
    <div className="flex items-end justify-between gap-3">
      <Sparkline
        ariaLabel={`${agent.label}'s activity over the last 2 weeks`}
        className="w-36 shrink-0 gap-0.5"
        height={28}
        values={values}
      />
      <span className="min-w-0 truncate text-eyebrow text-subtle tabular-nums">{agent.recap}</span>
    </div>
  );
}

export function StaysOnCard({ agent, adding }: { agent: ListedAgent; adding?: AddScopeState }) {
  return (
    <CatalogCard actionable className="relative gap-4 p-5">
      <div className="flex min-w-0 items-center gap-4">
        <AgentAvatar
          appearance={agent.appearance}
          label={agent.label}
          provider={agent.provider}
          quietIdle
          size={72}
          state={overallState(agent)}
          surfaceClassName="ring-card"
        />
        <div className="flex min-w-0 flex-1 flex-col gap-1">
          <div className="flex min-w-0 items-center gap-2">
            <CardName agent={agent} large />
            <AgentFlags agent={agent} />
          </div>
          <CatalogCard.Description className="line-clamp-2">
            {agent.description}
          </CatalogCard.Description>
        </div>
      </div>
      <NowLine agent={agent} />
      <Pulse agent={agent} />
      <CatalogCard.Actions className="relative z-1 mt-auto flex-nowrap justify-between">
        <PlaceChips agent={agent} />
        <AddPlaceButton adding={adding} agent={agent} />
      </CatalogCard.Actions>
    </CatalogCard>
  );
}

export function OnDemandCard({ agent }: { agent: ListedAgent }) {
  return (
    <CatalogCard actionable className="relative gap-3">
      <div className="flex min-w-0 items-center gap-3">
        <AgentAvatar
          appearance={agent.appearance}
          label={agent.label}
          provider={agent.provider}
          quietIdle
          size={48}
          state={overallState(agent)}
          surfaceClassName="ring-card"
        />
        <div className="flex min-w-0 flex-1 flex-col gap-0.5">
          <div className="flex min-w-0 items-center gap-2">
            <CardName agent={agent} />
            <AgentFlags agent={agent} />
          </div>
          <CatalogCard.Description className="line-clamp-2">
            {agent.description}
          </CatalogCard.Description>
        </div>
      </div>
      <CatalogCard.Actions className="relative z-1 mt-auto justify-between">
        <span className="min-w-0 truncate text-eyebrow text-subtle">
          {agent.active > 0
            ? `${agent.active} running now`
            : agent.lastActive
              ? `Last used ${agent.lastActive.toLowerCase()}`
              : "Never used"}
          {agent.recap ? ` · ${agent.recap}` : ""}
        </span>
        <AgentFleetNewSessionButton agentName={agent.label} onNewSession={() => undefined} />
      </CatalogCard.Actions>
    </CatalogCard>
  );
}
