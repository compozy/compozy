import { useIntentPreload } from "@/hooks/use-intent-preload";
/** The two ways to meet a teammate: a compact always-on card and a lighter on-demand card. */
import { CatalogCard, Sparkline } from "@compozy/ui";

import { agentLastCheckIn } from "../../lib/agent-profile";
import type { AgentPulseRead } from "../../hooks/use-agent-pulse";
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
  onPreload?: (member: AgentTeamMember) => void | Promise<unknown>;
  onNewSession: (name: string) => void;
  newSessionDisabled: boolean;
  add: AgentAddPlaceActions;
}

/** The name is the card's link; its hit area stretches over the card, under the controls. */
function CardName({
  member,
  title,
  onOpen,
  onPreload,
}: {
  member: AgentTeamMember;
  /** Shown on hover anywhere over the card, since the link covers it. */
  title?: string;
  onOpen: (member: AgentTeamMember) => void;
  onPreload?: (member: AgentTeamMember) => void | Promise<unknown>;
}) {
  const preload = useIntentPreload(() => onPreload?.(member), Boolean(onPreload));
  return (
    <button
      {...preload}
      className="min-w-0 text-left outline-none after:absolute after:inset-0 after:rounded-lg focus-visible:after:shadow-focus-ring"
      data-testid={`agent-fleet-row-link-${member.name}`}
      onClick={() => onOpen(member)}
      title={title || undefined}
      type="button"
    >
      <CatalogCard.Title className="line-clamp-1 whitespace-normal">
        {member.label}
      </CatalogCard.Title>
    </button>
  );
}

/** Two weeks of reports at a glance; the count is in its label and, above the card link, on hover. */
function CardPulse({ member, pulseRead }: { member: AgentTeamMember; pulseRead?: AgentPulseRead }) {
  const pulse = pulseRead?.pulse;
  if (!pulse?.values.some(value => value > 0)) return null;
  const recap = `${pulse.reports} ${pulse.reports === 1 ? "report" : "reports"} in 2 weeks`;
  return (
    <Sparkline
      ariaLabel={`${member.label}'s activity over the last 2 weeks: ${recap}`}
      className="relative z-1 w-14 shrink-0 gap-px"
      height={16}
      title={recap}
      values={pulse.values}
    />
  );
}

/** Compact, so a team of dozens still fits a screen: who, what now, where. Detail lives on its page. */
export function AgentAlwaysOnCard({
  member,
  actions,
  pulseRead,
}: {
  member: AgentTeamMember;
  actions: AgentTeamActions;
  pulseRead?: AgentPulseRead;
}) {
  return (
    <CatalogCard actionable className="relative gap-3" data-testid={`agent-card-${member.name}`}>
      <div className="flex min-w-0 items-center gap-3">
        <AgentMark
          appearance={member.appearance}
          label={member.label}
          provider={member.provider}
          quietIdle
          size={40}
          state={agentOverallState(member)}
          surfaceClassName="ring-card"
        />
        <div className="flex min-w-0 flex-1 flex-col">
          <div className="flex min-w-0 items-center gap-2">
            <CardName
              member={member}
              onOpen={actions.onOpen}
              onPreload={actions.onPreload}
              title={member.description}
            />
            <AgentFlags member={member} />
          </div>
          <CatalogCard.Description className="truncate">
            {member.description}
          </CatalogCard.Description>
        </div>
      </div>
      <div className="flex min-w-0 items-center justify-between gap-3">
        <AgentNowLine lastCheckIn={agentLastCheckIn(member.places)} member={member} />
        <CardPulse member={member} pulseRead={pulseRead} />
      </div>
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
            <CardName member={member} onOpen={actions.onOpen} onPreload={actions.onPreload} />
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
