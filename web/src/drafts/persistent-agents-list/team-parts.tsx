/** Small pieces every team card and row shares. */
import { Plus } from "lucide-react";

import { Button, Pill, StatusDot, cn, type StatusDotTone } from "@compozy/ui";

import { ScopePicker } from "../persistent-agents-shared/add-pickers";
import type { AgentState } from "../persistent-agents-shared/agent-state";
import type {
  AddScopeState,
  ListedAgent,
  ListedPlace,
} from "../persistent-agents-shared/agents-page";
import { SCOPE_LABEL, WORKSPACES } from "../persistent-agents-shared/fixtures";
import { nowLine } from "./team-now";

const DOT: Partial<Record<AgentState, StatusDotTone>> = {
  working: "success",
  "needs-you": "accent",
  failed: "danger",
  paused: "faint",
  idle: "faint",
  starting: "faint",
};

const PLACES_SHOWN = 3;

function placeOrder(place: ListedPlace) {
  return place.state === "needs-you" ? 0 : place.state === "working" ? 1 : 2;
}

function PlaceChip({ place }: { place: ListedPlace }) {
  const tone = DOT[place.state];
  return (
    <Pill
      className="max-w-40 min-w-0 shrink"
      size="sm"
      tone={place.state === "needs-you" ? "accent" : "neutral"}
    >
      {tone ? <StatusDot size="sm" tone={tone} /> : null}
      <span className="truncate">{SCOPE_LABEL(place.scope)}</span>
    </Pill>
  );
}

/** Where it works: the busiest places by name, then a count, however many there are. */
export function PlaceChips({ agent }: { agent: ListedAgent }) {
  const places = [...(agent.places ?? [])].sort(
    (left, right) => placeOrder(left) - placeOrder(right)
  );
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

export function AddPlaceButton({ agent, adding }: { agent: ListedAgent; adding?: AddScopeState }) {
  const open = adding?.agent === agent.name;
  const taken = new Set((agent.places ?? []).map(place => place.scope));
  const scopes = ["@user", ...WORKSPACES].filter(scope => !taken.has(scope));
  return (
    <ScopePicker
      defaultOpen={open}
      mode={open ? adding.mode : "scopes"}
      provider={agent.provider === "codex" ? "Codex" : "Claude"}
      scopes={open && adding.mode === "none" ? [] : scopes}
      trigger={
        <Button
          aria-label={`Add ${agent.label} to a project`}
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

export function NowLine({ agent, className }: { agent: ListedAgent; className?: string }) {
  const { text, tone } = nowLine(agent);
  const dot = DOT[tone];
  const live = tone === "working" || tone === "needs-you";
  return (
    <span
      className={cn(
        "flex min-w-0 items-center gap-2 text-small-body",
        live ? "text-fg" : "text-muted",
        className
      )}
    >
      {dot ? <StatusDot className="shrink-0" size="sm" tone={dot} /> : null}
      <span className="min-w-0 truncate">{text}</span>
    </span>
  );
}

export function AgentFlags({ agent }: { agent: ListedAgent }) {
  return (
    <>
      {agent.origin ? <Pill size="xs">{agent.origin}</Pill> : null}
      {agent.invalid ? (
        <Pill size="sm" tone="warning">
          Can't start
        </Pill>
      ) : null}
    </>
  );
}
