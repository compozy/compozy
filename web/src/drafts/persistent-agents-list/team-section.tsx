/** One group of the team, in cards or rows, plus the welcome when nobody stays on yet. */
import { Plus } from "lucide-react";
import { useState } from "react";

import { Button, ListGroup, cn, type ListingViewMode } from "@compozy/ui";

import { FLEET_ROWS_CLASS } from "@/systems/agent/components/agent-fleet-classes";

import { AgentAvatar } from "../persistent-agents-shared/agent-avatar";
import type { AddScopeState, ListedAgent } from "../persistent-agents-shared/agents-page";
import { OnDemandCard, StayOnCard } from "./team-card";
import { TeamRow } from "./team-row";

const GRID = "grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3";

function agentKey(agent: ListedAgent) {
  return `${agent.name}-${agent.origin ?? ""}`;
}

export function TeamSection({
  label,
  hint,
  agents,
  view,
  adding,
  pageSize,
}: {
  label: string;
  hint: string;
  agents: readonly ListedAgent[];
  view: ListingViewMode;
  adding?: AddScopeState;
  /** Cards shown before Show more; omit to show them all. */
  pageSize?: number;
}) {
  const [limit, setLimit] = useState(pageSize ?? agents.length);
  const visible = agents.slice(0, pageSize ? limit : agents.length);
  const rest = agents.length - visible.length;
  return (
    <ListGroup
      count={agents.length}
      label={
        <span className="flex min-w-0 items-baseline gap-2.5">
          <span>{label}</span>
          <span className="truncate text-eyebrow font-normal text-subtle">{hint}</span>
        </span>
      }
    >
      {view === "cards" ? (
        <div className={cn(GRID, "pt-2.5")}>
          {visible.map(agent =>
            agent.kind === "persistent" ? (
              <StayOnCard adding={adding} agent={agent} key={agentKey(agent)} />
            ) : (
              <OnDemandCard agent={agent} key={agentKey(agent)} />
            )
          )}
        </div>
      ) : (
        <div className={cn(FLEET_ROWS_CLASS, "mt-2.5")}>
          {visible.map(agent => (
            <TeamRow adding={adding} agent={agent} key={agentKey(agent)} />
          ))}
        </div>
      )}
      {rest > 0 ? (
        <div className="flex justify-center pt-4">
          <Button
            onClick={() => setLimit(limit + (pageSize ?? 0))}
            size="sm"
            type="button"
            variant="ghost"
          >
            Show {Math.min(rest, pageSize ?? rest)} more of {rest}
          </Button>
        </div>
      ) : null}
    </ListGroup>
  );
}

const SLEEPERS = ["night-owl", "morning-lark", "quiet-one"];

/** Nobody stays on yet: shows what it would look like instead of explaining it. */
export function StayOnWelcome({ onCreate }: { onCreate: () => void }) {
  return (
    <section className="flex items-center gap-6 rounded-lg bg-card p-6 shadow-card">
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
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <h2 className="text-heading font-medium text-fg-strong">
          Give someone a job that keeps going
        </h2>
        <p className="text-small-body text-muted">
          A stay-on agent lives in your project, checks in by itself and tells you what it found.
        </p>
      </div>
      <Button onClick={onCreate} type="button" variant="secondary">
        <Plus aria-hidden="true" />
        New stay-on agent
      </Button>
    </section>
  );
}
