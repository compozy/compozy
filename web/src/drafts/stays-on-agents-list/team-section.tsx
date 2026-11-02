/** One group of the team, in cards or rows, the two-column variation, and the first-run welcome. */
import { Infinity as InfinityIcon, MessageSquare, Plus } from "@compozy/ui/icons";
import { useState, type ReactNode } from "react";

import { Button, Section, type ListingViewMode } from "@compozy/ui";

import { FLEET_ROWS_CLASS } from "@/systems/agent/components/agent-fleet-classes";

import { AgentAvatar } from "../stays-on-agents-shared/agent-avatar";
import type { AddScopeState, ListedAgent } from "../stays-on-agents-shared/agents-page";
import { OnDemandCard, StaysOnCard } from "./team-card";
import { TeamRow } from "./team-row";

const GRID = "grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3";

function agentKey(agent: ListedAgent) {
  return `${agent.name}-${agent.origin ?? ""}`;
}

const GROUP = {
  "Stays on": { icon: InfinityIcon },
  "On demand": { icon: MessageSquare },
} as const;

type GroupLabel = keyof typeof GROUP;

export function TeamSection({
  label,
  agents,
  view,
  adding,
  pageSize,
  inColumn = false,
}: {
  label: GroupLabel;
  agents: readonly ListedAgent[];
  view: ListingViewMode;
  adding?: AddScopeState;
  /** Agents shown before Load more; omit to show them all. */
  pageSize?: number;
  inColumn?: boolean;
}) {
  const [limit, setLimit] = useState(pageSize ?? agents.length);
  const visible = agents.slice(0, pageSize ? limit : agents.length);
  return (
    <Section count={agents.length} icon={GROUP[label].icon} label={label}>
      {view === "cards" ? (
        <div className={GRID}>
          {visible.map(agent =>
            agent.kind === "stays_on" ? (
              <StaysOnCard adding={adding} agent={agent} key={agentKey(agent)} />
            ) : (
              <OnDemandCard agent={agent} key={agentKey(agent)} />
            )
          )}
        </div>
      ) : (
        <div className={FLEET_ROWS_CLASS}>
          {visible.map(agent => (
            <TeamRow adding={adding} agent={agent} inColumn={inColumn} key={agentKey(agent)} />
          ))}
        </div>
      )}
      {visible.length < agents.length ? (
        <div className="flex justify-center pt-4">
          <Button
            onClick={() => setLimit(limit + (pageSize ?? 0))}
            size="sm"
            type="button"
            variant="neutral"
          >
            Load more agents
          </Button>
        </div>
      ) : null}
    </Section>
  );
}

const COLUMN_PAGE = 12;

/** Holds a column's place when a search leaves it with nobody to show. */
function NobodyHere({ label }: { label: GroupLabel }) {
  return (
    <Section count={0} icon={GROUP[label].icon} label={label}>
      <p className="py-3 text-small-body text-subtle">Nobody here matches.</p>
    </Section>
  );
}

/**
 * The variation to compare with Cards and Rows: both groups side by side, in rows only. It reads the
 * window's width rather than the screen's, since a tiled window can be narrow on any display.
 */
export function TeamColumns({
  staysOn,
  onDemand,
  adding,
  welcome,
}: {
  staysOn: readonly ListedAgent[];
  onDemand: readonly ListedAgent[];
  adding?: AddScopeState;
  /** Shown in the first column when nobody stays on yet. */
  welcome?: ReactNode;
}) {
  return (
    <div className="@container w-full min-w-0" data-testid="team-columns">
      <div className="grid grid-cols-1 items-start gap-8 @4xl:grid-cols-2 @4xl:gap-6">
        {staysOn.length > 0 ? (
          <TeamSection
            adding={adding}
            agents={staysOn}
            inColumn
            label="Stays on"
            pageSize={COLUMN_PAGE}
            view="rows"
          />
        ) : welcome ? (
          <Section count={0} icon={GROUP["Stays on"].icon} label="Stays on">
            {welcome}
          </Section>
        ) : (
          <NobodyHere label="Stays on" />
        )}
        {onDemand.length > 0 ? (
          <TeamSection
            agents={onDemand}
            inColumn
            label="On demand"
            pageSize={COLUMN_PAGE}
            view="rows"
          />
        ) : (
          <NobodyHere label="On demand" />
        )}
      </div>
    </div>
  );
}

const SLEEPERS = ["night-owl", "morning-lark", "quiet-one"];

/** Nobody stays on yet: shows what it would look like instead of explaining it. */
export function StaysOnWelcome({ onCreate }: { onCreate: () => void }) {
  return (
    <section className="flex flex-wrap items-center gap-6 rounded-lg bg-card p-6 shadow-card">
      <div aria-hidden="true" className="flex shrink-0 -space-x-3">
        {SLEEPERS.map(seed => (
          <AgentAvatar
            appearance={{ avatar: "generated", seed }}
            key={seed}
            label=""
            quietIdle
            size={64}
            state="paused"
            surfaceClassName="ring-card"
          />
        ))}
      </div>
      <div className="flex min-w-56 flex-1 flex-col gap-1">
        <h2 className="text-heading font-medium text-fg-strong">
          Give someone a job that keeps going
        </h2>
        <p className="text-small-body text-muted-foreground">
          A stays-on agent lives in your project, checks in by itself and tells you what it found.
        </p>
      </div>
      <Button onClick={onCreate} type="button" variant="secondary">
        <Plus aria-hidden="true" />
        New stays-on agent
      </Button>
    </section>
  );
}
