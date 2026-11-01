/** The dense reading of the team, for scanning 100 agents. */
import { ListingRow, Sparkline } from "@compozy/ui";

import { AgentFleetNewSessionButton } from "@/systems/agent/components/agent-fleet-new-session-button";

import { AgentAvatar } from "../persistent-agents-shared/agent-avatar";
import type { AddScopeState, ListedAgent } from "../persistent-agents-shared/agents-page";
import { overallState } from "./team-filters";
import { openProfile } from "./team-now";
import { AddPlaceButton, AgentFlags, NowLine, PlaceChips } from "./team-parts";

/** Fixed width, so a column of them lines up however many chips sit beside each. */
function RowPulse({ agent }: { agent: ListedAgent }) {
  return (
    <span className="w-20 shrink-0">
      {(agent.activity?.length ?? 0) > 0 ? (
        <Sparkline
          ariaLabel={`${agent.label}'s activity over the last 2 weeks`}
          className="gap-0.5"
          height={20}
          values={agent.activity!}
        />
      ) : null}
    </span>
  );
}

export function TeamRow({ agent, adding }: { agent: ListedAgent; adding?: AddScopeState }) {
  const stayOn = agent.kind === "persistent";
  return (
    <ListingRow>
      <ListingRow.Link
        render={
          <button
            aria-label={`Open ${agent.label}`}
            onClick={() => openProfile(agent)}
            type="button"
          />
        }
      >
        <ListingRow.Icon className="size-8 bg-transparent">
          <AgentAvatar
            appearance={agent.appearance}
            label={agent.label}
            provider={agent.provider}
            quietIdle
            size={32}
            state={overallState(agent)}
            surfaceClassName="ring-card"
          />
        </ListingRow.Icon>
        <ListingRow.Main>
          <ListingRow.Name>
            <ListingRow.Title>{agent.label}</ListingRow.Title>
            <AgentFlags agent={agent} />
          </ListingRow.Name>
          <ListingRow.Meta>
            {stayOn ? (
              <NowLine agent={agent} />
            ) : (
              <span className="min-w-0 truncate">{agent.description}</span>
            )}
          </ListingRow.Meta>
        </ListingRow.Main>
      </ListingRow.Link>
      <ListingRow.Trail className="gap-4">
        {stayOn ? (
          <>
            <PlaceChips agent={agent} />
            <RowPulse agent={agent} />
            <AddPlaceButton adding={adding} agent={agent} />
          </>
        ) : (
          <>
            <span className="text-eyebrow text-subtle">{agent.lastActive ?? "Never used"}</span>
            <RowPulse agent={agent} />
            <AgentFleetNewSessionButton agentName={agent.label} onNewSession={() => undefined} />
          </>
        )}
      </ListingRow.Trail>
    </ListingRow>
  );
}
