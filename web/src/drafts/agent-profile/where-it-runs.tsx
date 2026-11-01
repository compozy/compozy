/** S8: the scopes a persistent agent runs in, with each pause reason in plain words. */
import { Archive, MessagesSquare, Pause, Play, Plus } from "lucide-react";

import {
  Alert,
  AlertDescription,
  Button,
  Empty,
  StatusDot,
  cn,
  type StatusDotTone,
} from "@compozy/ui";

import { AgentAvatar } from "../persistent-agents-shared/agent-avatar";
import { AGENT_STATE_LABEL, type AgentState } from "../persistent-agents-shared/agent-state";
import {
  PAUSE_COPY,
  SCOPE_LABEL,
  type DraftAgent,
  type PauseReason,
} from "../persistent-agents-shared/fixtures";

export interface PlaceRow {
  scope: string;
  state: AgentState;
  pauseReason?: PauseReason;
  last?: string;
  next?: string;
  session: string;
}

const DOT: Partial<Record<AgentState, StatusDotTone>> = {
  working: "success",
  failed: "danger",
  paused: "faint",
  idle: "faint",
  "needs-you": "accent",
};

function Fact({ label, value }: { label: string; value: string }) {
  return (
    <span className="flex flex-col gap-0.5">
      <span className="text-eyebrow text-subtle">{label}</span>
      <span className="text-small-body text-fg-2">{value}</span>
    </span>
  );
}

function canResume(reason: PauseReason | undefined) {
  return reason !== "definition_missing" && reason !== "definition_not_persistent";
}

function Row({ agent, place }: { agent: DraftAgent; place: PlaceRow }) {
  const paused = place.state === "paused";
  const dot = DOT[place.state];
  return (
    <article className="flex flex-col gap-3 p-4">
      <div className="flex items-center gap-3">
        <AgentAvatar
          appearance={agent.appearance}
          label={agent.label}
          provider={agent.provider}
          quietIdle
          scope={SCOPE_LABEL(place.scope)}
          size={32}
          state={place.state}
          surfaceClassName="ring-card"
        />
        <div className="flex min-w-0 flex-1 flex-col">
          <span className="text-item-title font-medium text-fg">
            {place.scope === "@user" ? "Global" : `Persistent in ${place.scope}`}
          </span>
          <span className="flex items-center gap-1.5 text-small-body text-muted">
            {dot ? <StatusDot size="sm" tone={dot} /> : null}
            {place.pauseReason ? PAUSE_COPY[place.pauseReason] : AGENT_STATE_LABEL[place.state]}
          </span>
        </div>
        <div className="flex items-center gap-1">
          <Button aria-label="Open main session" size="icon-sm" type="button" variant="quiet">
            <MessagesSquare aria-hidden="true" />
          </Button>
          {paused ? (
            <Button
              disabled={!canResume(place.pauseReason)}
              size="sm"
              type="button"
              variant="secondary"
            >
              <Play aria-hidden="true" />
              Resume
            </Button>
          ) : (
            <Button size="sm" type="button" variant="quiet">
              <Pause aria-hidden="true" />
              Pause
            </Button>
          )}
          <Button size="sm" type="button" variant="quiet">
            <Archive aria-hidden="true" />
            Archive
          </Button>
        </div>
      </div>
      <div className={cn("grid grid-cols-3 gap-4 rounded-md bg-sunken px-3 py-2.5")}>
        <Fact label="Last check" value={place.last ?? "No checks yet"} />
        {place.next ? (
          <Fact label="Next check" value={place.next} />
        ) : (
          <Fact label="Next check" value="Not scheduled" />
        )}
        <Fact label="Main session" value={place.session} />
      </div>
    </article>
  );
}

export function WhereItRuns({
  agent,
  places,
  capped = false,
}: {
  agent: DraftAgent;
  places: readonly PlaceRow[];
  capped?: boolean;
}) {
  return (
    <div className="flex flex-col gap-4 py-2">
      {capped ? (
        <Alert variant="warning">
          <AlertDescription>
            You have 10 active persistent agents. Pause one or raise
            agents.persistent.max_active_instances.
          </AlertDescription>
        </Alert>
      ) : null}
      {places.length === 0 ? (
        <Empty
          action={
            <Button size="sm" type="button" variant="secondary">
              <Plus aria-hidden="true" />
              Add to a workspace
            </Button>
          }
          icon={Plus}
          title="Not running anywhere"
        />
      ) : (
        <div className="flex flex-col divide-y divide-line-soft overflow-hidden rounded-lg bg-card shadow-card">
          {places.map(place => (
            <Row agent={agent} key={place.scope} place={place} />
          ))}
        </div>
      )}
    </div>
  );
}
