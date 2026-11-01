/** The dense reading of the team, for scanning 100 agents. */
import type { ReactNode } from "react";

import { ListingRow, Sparkline } from "@compozy/ui";

import { AgentFleetNewSessionButton } from "@/systems/agent/components/agent-fleet-new-session-button";

import { AgentAvatar } from "../stays-on-agents-shared/agent-avatar";
import type { AddScopeState, ListedAgent } from "../stays-on-agents-shared/agents-page";
import { SCOPE_LABEL } from "../stays-on-agents-shared/fixtures";
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

/** Where it works, in as few words as a column has room for. */
function placesSummary(agent: ListedAgent) {
  const places = agent.places ?? [];
  if (places.length === 0) return "Not added yet";
  return places.length === 1 ? SCOPE_LABEL(places[0]!.scope) : `${places.length} projects`;
}

/** Only a column wide enough keeps this; the name and the action never give way. */
function ColumnNote({ children }: { children: ReactNode }) {
  return (
    <span className="hidden w-24 truncate text-right text-eyebrow text-subtle @lg/section:block">
      {children}
    </span>
  );
}

export function TeamRow({
  agent,
  adding,
  inColumn = false,
}: {
  agent: ListedAgent;
  adding?: AddScopeState;
  /** A half-width column: chips give way to a short summary. */
  inColumn?: boolean;
}) {
  const staysOn = agent.kind === "stays_on";
  const lastUsed = agent.lastActive ?? "Never used";
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
            {staysOn ? (
              <NowLine agent={agent} />
            ) : (
              <span className="min-w-0 truncate text-small-body text-muted-foreground">
                {agent.description}
              </span>
            )}
          </ListingRow.Meta>
        </ListingRow.Main>
      </ListingRow.Link>
      <ListingRow.Trail className="gap-4">
        {staysOn ? (
          <>
            {inColumn ? (
              <ColumnNote>{placesSummary(agent)}</ColumnNote>
            ) : (
              <PlaceChips agent={agent} />
            )}
            <RowPulse agent={agent} />
            <AddPlaceButton adding={adding} agent={agent} />
          </>
        ) : (
          <>
            {inColumn ? (
              <ColumnNote>{lastUsed}</ColumnNote>
            ) : (
              <span className="text-eyebrow text-subtle">{lastUsed}</span>
            )}
            <RowPulse agent={agent} />
            <AgentFleetNewSessionButton agentName={agent.label} onNewSession={() => undefined} />
          </>
        )}
      </ListingRow.Trail>
    </ListingRow>
  );
}
