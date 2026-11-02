/** One group of the team, in cards or rows, and the first-run welcome. */
import { Infinity as InfinityIcon, MessageSquare, Plus } from "@compozy/ui/icons";
import { useState } from "react";

import { Button, Section, type ListingViewMode } from "@compozy/ui";

import type { AgentTeamMember } from "../../lib/agent-team";
import { FLEET_ROWS_CLASS } from "../agent-fleet-classes";
import { AgentMark } from "../agent-mark";
import { AgentOnDemandCard, AgentStaysOnCard, type AgentTeamActions } from "./agent-team-card";
import { AgentTeamRow } from "./agent-team-row";

const GRID = "grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3";

const GROUP = {
  "Stays on": { icon: InfinityIcon, testId: "agents-section-stays-on" },
  "On demand": { icon: MessageSquare, testId: "agents-section-on-demand" },
} as const;

export function AgentTeamSection({
  label,
  members,
  view,
  actions,
  pageSize,
}: {
  label: keyof typeof GROUP;
  members: readonly AgentTeamMember[];
  view: ListingViewMode;
  actions: AgentTeamActions;
  /** Agents shown before Load more; omit to show them all. */
  pageSize?: number;
}) {
  const [limit, setLimit] = useState(pageSize ?? members.length);
  const visible = members.slice(0, pageSize ? limit : members.length);
  return (
    <div data-testid={GROUP[label].testId}>
      <Section count={members.length} icon={GROUP[label].icon} label={label}>
        {view === "cards" ? (
          <div className={GRID}>
            {visible.map(member =>
              member.kind === "stays_on" ? (
                <AgentStaysOnCard actions={actions} key={member.key} member={member} />
              ) : (
                <AgentOnDemandCard actions={actions} key={member.key} member={member} />
              )
            )}
          </div>
        ) : (
          <div className={FLEET_ROWS_CLASS}>
            {visible.map(member => (
              <AgentTeamRow actions={actions} key={member.key} member={member} />
            ))}
          </div>
        )}
        {visible.length < members.length ? (
          <div className="flex justify-center pt-4">
            <Button
              data-testid="agent-fleet-load-more"
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
    </div>
  );
}

const SLEEPERS = ["night-owl", "morning-lark", "quiet-one"];

/** Nobody stays on yet: shows what it would look like instead of explaining it. */
export function AgentStaysOnWelcome({ onCreate }: { onCreate: () => void }) {
  return (
    <section
      className="flex flex-wrap items-center gap-6 rounded-lg bg-card p-6 shadow-card"
      data-testid="agents-stays-on-welcome"
    >
      <div aria-hidden="true" className="flex shrink-0 -space-x-3">
        {SLEEPERS.map(seed => (
          <AgentMark
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
