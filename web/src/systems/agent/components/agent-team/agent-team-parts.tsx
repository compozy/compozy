/** Small pieces every team card and row shares. */
import { Plus } from "@compozy/ui/icons";

import { Button, Pill, StatusDot, cn } from "@compozy/ui";

import {
  AGENT_STATE_DOT,
  agentNowLine,
  type AgentTeamMember,
  type AgentTeamPlace,
} from "../../lib/agent-team";
import type { AgentPlaceOption } from "../agent-add-pickers";
import { AgentScopePicker } from "../agent-add-pickers";

const PLACES_SHOWN = 3;

function placeOrder(place: AgentTeamPlace) {
  return place.state === "needs-you" ? 0 : place.state === "working" ? 1 : 2;
}

function PlaceChip({ place }: { place: AgentTeamPlace }) {
  const tone = AGENT_STATE_DOT[place.state];
  return (
    <Pill
      className="max-w-40 min-w-0 shrink"
      size="sm"
      tone={place.state === "needs-you" ? "brand" : "neutral"}
    >
      {tone ? <StatusDot size="sm" tone={tone} /> : null}
      <span className="truncate">{place.label}</span>
    </Pill>
  );
}

/** Where it works: the busiest places by name, then a count, however many there are. */
export function AgentPlaceChips({ member }: { member: AgentTeamMember }) {
  const places = [...member.places].sort((left, right) => placeOrder(left) - placeOrder(right));
  if (places.length === 0) {
    return <span className="text-small-body text-subtle">Not working anywhere yet</span>;
  }
  const rest = places.length - PLACES_SHOWN;
  return (
    <div className="flex min-w-0 items-center gap-1.5">
      {places.slice(0, PLACES_SHOWN).map(place => (
        <PlaceChip key={place.scope} place={place} />
      ))}
      {rest > 0 ? (
        <Pill className="shrink-0" form="plain" size="sm">
          +{rest} more
        </Pill>
      ) : null}
    </div>
  );
}

export interface AgentAddPlaceActions {
  /** Every place an agent could work: Global, then each project. */
  places: readonly AgentPlaceOption[];
  capped: boolean;
  /** Provider ids that aren't signed in, with their names. */
  signedOut: ReadonlyMap<string, string>;
  onAdd: (agent: string, place: string) => void;
  onSignIn: () => void;
}

export function AgentAddPlaceButton({
  member,
  add,
}: {
  member: AgentTeamMember;
  add: AgentAddPlaceActions;
}) {
  const taken = new Set(member.places.map(place => place.scope));
  return (
    <AgentScopePicker
      capped={add.capped}
      onPick={place => add.onAdd(member.name, place)}
      onSignIn={add.onSignIn}
      scopes={add.places.filter(place => !taken.has(place.id))}
      signedOut={add.signedOut.get(member.provider ?? "") ?? null}
      trigger={
        <Button
          aria-label={`Add ${member.label} to a project`}
          size="icon-sm"
          type="button"
          variant="quiet"
        >
          <Plus aria-hidden="true" />
        </Button>
      }
    />
  );
}

export function AgentNowLine({
  member,
  lastCheckIn,
  className,
}: {
  member: AgentTeamMember;
  lastCheckIn: string | null;
  className?: string;
}) {
  const { text, tone } = agentNowLine(member, lastCheckIn);
  const dot = AGENT_STATE_DOT[tone];
  const live = tone === "working" || tone === "needs-you";
  return (
    <span
      className={cn(
        "flex min-w-0 items-center gap-2 text-small-body",
        live ? "text-fg" : "text-muted-foreground",
        className
      )}
    >
      {dot ? <StatusDot className="shrink-0" size="sm" tone={dot} /> : null}
      <span className="min-w-0 truncate">{text}</span>
    </span>
  );
}

export function AgentFlags({ member }: { member: AgentTeamMember }) {
  return (
    <>
      {member.origin ? <Pill size="xs">{member.origin}</Pill> : null}
      {member.invalid ? (
        <Pill size="sm" tone="warning">
          Can't start
        </Pill>
      ) : null}
    </>
  );
}
