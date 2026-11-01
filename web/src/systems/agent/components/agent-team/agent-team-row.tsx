/** The dense reading of the team, for scanning 100 agents. */
import { ListingRow, Sparkline } from "@compozy/ui";

import { agentLastCheckIn } from "../../lib/agent-profile";
import { agentOverallState, agentUsageLine, type AgentTeamMember } from "../../lib/agent-team";
import { AgentFleetNewSessionButton } from "../agent-fleet-new-session-button";
import { AgentMark } from "../agent-mark";
import { useAgentPulse } from "../../hooks/use-agent-pulse";
import type { AgentTeamActions } from "./agent-team-card";
import { AgentAddPlaceButton, AgentFlags, AgentNowLine, AgentPlaceChips } from "./agent-team-parts";

/** Fixed width, so a column of them lines up however many chips sit beside each. */
function RowPulse({ member }: { member: AgentTeamMember }) {
  const { pulse } = useAgentPulse(member);
  return (
    <span className="w-20 shrink-0">
      {pulse?.values.some(value => value > 0) ? (
        <Sparkline
          ariaLabel={`${member.label}'s activity over the last 2 weeks`}
          className="gap-0.5"
          height={20}
          values={pulse.values}
        />
      ) : null}
    </span>
  );
}

export function AgentTeamRow({
  member,
  actions,
}: {
  member: AgentTeamMember;
  actions: AgentTeamActions;
}) {
  const staysOn = member.kind === "stays_on";
  return (
    <ListingRow data-testid={`agent-row-${member.name}`}>
      <ListingRow.Link
        render={
          <button
            aria-label={`Open ${member.label}`}
            data-testid={`agent-fleet-row-link-${member.name}`}
            onClick={() => actions.onOpen(member)}
            type="button"
          />
        }
      >
        <ListingRow.Icon className="size-8 bg-transparent">
          <AgentMark
            appearance={member.appearance}
            label={member.label}
            provider={member.provider}
            quietIdle
            size={32}
            state={agentOverallState(member)}
            surfaceClassName="ring-card"
          />
        </ListingRow.Icon>
        <ListingRow.Main>
          <ListingRow.Name>
            <ListingRow.Title>{member.label}</ListingRow.Title>
            <AgentFlags member={member} />
          </ListingRow.Name>
          <ListingRow.Meta>
            {staysOn ? (
              <AgentNowLine lastCheckIn={agentLastCheckIn(member.places)} member={member} />
            ) : (
              <span className="min-w-0 truncate text-small-body text-muted-foreground">
                {member.description}
              </span>
            )}
          </ListingRow.Meta>
        </ListingRow.Main>
      </ListingRow.Link>
      <ListingRow.Trail className="gap-4">
        {staysOn ? (
          <>
            <AgentPlaceChips member={member} />
            <RowPulse member={member} />
            <AgentAddPlaceButton add={actions.add} member={member} />
          </>
        ) : (
          <>
            <span className="text-eyebrow text-subtle">{agentUsageLine(member)}</span>
            <span className="w-20 shrink-0" />
            <AgentFleetNewSessionButton
              agentName={member.name}
              disabled={actions.newSessionDisabled}
              onNewSession={actions.onNewSession}
            />
          </>
        )}
      </ListingRow.Trail>
    </ListingRow>
  );
}
