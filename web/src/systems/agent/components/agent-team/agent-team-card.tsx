/** The two ways to meet a teammate: a big stays-on card and a lighter on-demand card. */
import { CatalogCard, Sparkline } from "@compozy/ui";

import { agentLastCheckIn } from "../../lib/agent-profile";
import { useAgentPulse } from "../../hooks/use-agent-pulse";
import { agentOverallState, agentUsageLine, type AgentTeamMember } from "../../lib/agent-team";
import { AgentFleetNewSessionButton } from "../agent-fleet-new-session-button";
import { AgentMark } from "../agent-mark";
import {
  AgentAddPlaceButton,
  AgentFlags,
  AgentNowLine,
  AgentPlaceChips,
  type AgentAddPlaceActions,
} from "./agent-team-parts";

export interface AgentTeamActions {
  onOpen: (member: AgentTeamMember) => void;
  onNewSession: (name: string) => void;
  newSessionDisabled: boolean;
  add: AgentAddPlaceActions;
}

/** The name is the card's link; its hit area stretches over the card, under the controls. */
function CardName({
  member,
  large = false,
  onOpen,
}: {
  member: AgentTeamMember;
  large?: boolean;
  onOpen: (member: AgentTeamMember) => void;
}) {
  return (
    <button
      className={`min-w-0 text-left outline-none after:absolute after:inset-0 after:rounded-lg focus-visible:after:shadow-focus-ring ${large ? "text-heading" : ""}`}
      data-testid={`agent-fleet-row-link-${member.name}`}
      onClick={() => onOpen(member)}
      type="button"
    >
      <CatalogCard.Title className="line-clamp-1 whitespace-normal">
        {member.label}
      </CatalogCard.Title>
    </button>
  );
}

function Pulse({ member }: { member: AgentTeamMember }) {
  const { pulse, known } = useAgentPulse(member);
  if (!known || !pulse) return <div className="h-7" />;
  if (pulse.values.every(value => value === 0)) {
    return <div className="flex h-7 items-end text-eyebrow text-subtle">No activity yet</div>;
  }
  return (
    <div className="flex items-end justify-between gap-3">
      <Sparkline
        ariaLabel={`${member.label}'s activity over the last 2 weeks`}
        className="w-36 shrink-0 gap-0.5"
        height={28}
        values={pulse.values}
      />
      <span className="min-w-0 truncate text-eyebrow text-subtle tabular-nums">
        {pulse.reports} {pulse.reports === 1 ? "report" : "reports"} in 2 weeks
      </span>
    </div>
  );
}

export function AgentStaysOnCard({
  member,
  actions,
}: {
  member: AgentTeamMember;
  actions: AgentTeamActions;
}) {
  return (
    <CatalogCard
      actionable
      className="relative gap-4 p-5"
      data-testid={`agent-card-${member.name}`}
    >
      <div className="flex min-w-0 items-center gap-4">
        <AgentMark
          appearance={member.appearance}
          label={member.label}
          provider={member.provider}
          quietIdle
          size={72}
          state={agentOverallState(member)}
          surfaceClassName="ring-card"
        />
        <div className="flex min-w-0 flex-1 flex-col gap-1">
          <div className="flex min-w-0 items-center gap-2">
            <CardName large member={member} onOpen={actions.onOpen} />
            <AgentFlags member={member} />
          </div>
          <CatalogCard.Description className="line-clamp-2">
            {member.description}
          </CatalogCard.Description>
        </div>
      </div>
      <AgentNowLine lastCheckIn={agentLastCheckIn(member.places)} member={member} />
      <Pulse member={member} />
      <CatalogCard.Actions className="relative z-1 mt-auto flex-nowrap justify-between">
        <AgentPlaceChips member={member} />
        <AgentAddPlaceButton add={actions.add} member={member} />
      </CatalogCard.Actions>
    </CatalogCard>
  );
}

export function AgentOnDemandCard({
  member,
  actions,
}: {
  member: AgentTeamMember;
  actions: AgentTeamActions;
}) {
  return (
    <CatalogCard actionable className="relative gap-3" data-testid={`agent-card-${member.name}`}>
      <div className="flex min-w-0 items-center gap-3">
        <AgentMark
          appearance={member.appearance}
          label={member.label}
          provider={member.provider}
          quietIdle
          size={48}
          state={agentOverallState(member)}
          surfaceClassName="ring-card"
        />
        <div className="flex min-w-0 flex-1 flex-col gap-0.5">
          <div className="flex min-w-0 items-center gap-2">
            <CardName member={member} onOpen={actions.onOpen} />
            <AgentFlags member={member} />
          </div>
          <CatalogCard.Description className="line-clamp-2">
            {member.description}
          </CatalogCard.Description>
        </div>
      </div>
      <CatalogCard.Actions className="relative z-1 mt-auto justify-between">
        <span className="min-w-0 truncate text-eyebrow text-subtle">{agentUsageLine(member)}</span>
        <AgentFleetNewSessionButton
          agentName={member.name}
          disabled={actions.newSessionDisabled}
          onNewSession={actions.onNewSession}
        />
      </CatalogCard.Actions>
    </CatalogCard>
  );
}
