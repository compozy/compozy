import { Link } from "@tanstack/react-router";
import { ChevronRight } from "lucide-react";

import { Button, cn, Empty, Eyebrow, OwnerAvatar, Panel, Section } from "@agh/ui";

import type { HomeAgentRow } from "../hooks/use-home-agents";

export interface HomeAgentsPanelProps {
  rows: HomeAgentRow[];
}

const ROW_GRID = "grid-cols-[minmax(0,1.1fr)_52px_64px_52px_minmax(0,1fr)]";
const ROW_GRID_NARROW = "max-[760px]:grid-cols-[minmax(0,1fr)_52px_minmax(0,1fr)]";

function formatRuntimeHours(runtimeSeconds: number): string {
  return `${(runtimeSeconds / 3600).toFixed(1)}h`;
}

/**
 * Zone 5a — the agent fleet with exact lifetime session metrics; the runtime
 * bar normalizes against the busiest agent in neutral ink.
 */
export function HomeAgentsPanel({ rows }: HomeAgentsPanelProps) {
  return (
    <Section
      bodyClassName="flex flex-1 flex-col"
      className="flex min-h-full flex-col"
      data-slot="home-agents"
      label="Agents"
      right={<span className="text-micro text-faint">runtime · all time</span>}
    >
      <Panel
        bodyClassName="flex flex-1 flex-col p-0"
        className="flex-1"
        foot={
          <Button render={<Link to="/agents" />} size="sm" variant="ghost">
            View agents
            <ChevronRight aria-hidden="true" />
          </Button>
        }
      >
        {rows.length === 0 ? (
          <Empty
            description="Define an agent to put the fleet to work."
            title="No agents in this workspace"
          />
        ) : (
          <div className="flex flex-1 flex-col">
            <div
              aria-hidden="true"
              className={`grid ${ROW_GRID} items-center gap-3 border-b border-line-soft px-4 py-2 max-[760px]:hidden`}
            >
              <Eyebrow className="text-subtle">Agent</Eyebrow>
              <Eyebrow className="text-right text-subtle">Active</Eyebrow>
              <Eyebrow className="text-right text-subtle">Sessions</Eyebrow>
              <Eyebrow className="text-right text-subtle">Failed</Eyebrow>
              <Eyebrow className="text-subtle">Runtime</Eyebrow>
            </div>
            <div className="flex flex-1 flex-col divide-y divide-line-soft">
              {rows.map(row => (
                <Link
                  className={`grid ${ROW_GRID} ${ROW_GRID_NARROW} flex-1 items-center gap-3 px-4 py-2.5 transition-colors duration-base hover:bg-row-hover focus-visible:shadow-focus-inset focus-visible:outline-none`}
                  data-slot="home-agent-row"
                  key={row.name}
                  params={{ name: row.name }}
                  to="/agents/$name"
                >
                  <span className="flex min-w-0 items-center gap-2 text-small-body font-medium text-fg-strong">
                    <OwnerAvatar name={row.name} ownerId={row.name} ownerKind="agent" size="sm" />
                    <span className="truncate">{row.name}</span>
                  </span>
                  <AgentCount value={row.active} />
                  <AgentCount className="max-[760px]:hidden" value={row.sessions} />
                  <AgentCount
                    className="max-[760px]:hidden"
                    tone={row.failed > 0 ? "danger" : undefined}
                    value={row.failed}
                  />
                  <span className="flex min-w-0 items-center gap-2.5">
                    <span className="h-1 min-w-0 flex-1 overflow-hidden rounded-pill bg-input-fill">
                      <span
                        className="block h-full rounded-pill bg-viz-bar"
                        style={{ width: `${Math.round(row.runtimeShare * 100)}%` }}
                      />
                    </span>
                    <span className="shrink-0 font-mono text-mono-id tabular-nums text-subtle">
                      {formatRuntimeHours(row.runtimeSeconds)}
                    </span>
                  </span>
                </Link>
              ))}
            </div>
          </div>
        )}
      </Panel>
    </Section>
  );
}

function AgentCount({
  value,
  tone,
  className,
  ...props
}: {
  value: number;
  tone?: "danger";
} & React.ComponentProps<"span">) {
  const color = tone === "danger" ? "text-danger" : value === 0 ? "text-faint" : "text-muted";
  return (
    <span
      className={cn("text-right font-mono text-mono-id tabular-nums", color, className)}
      {...props}
    >
      {value}
    </span>
  );
}
