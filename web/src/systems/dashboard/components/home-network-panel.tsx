import { Link } from "@tanstack/react-router";
import { ChevronRight } from "lucide-react";

import { Button, Panel, Pill, Section } from "@agh/ui";

import type { HomeNetworkRow } from "../lib/home-network";

export interface HomeNetworkPanelProps {
  rows: HomeNetworkRow[];
}

/**
 * Zone 3b — network coordination at a glance. Rows render only for counters
 * the daemon actually reports.
 */
export function HomeNetworkPanel({ rows }: HomeNetworkPanelProps) {
  return (
    <Section
      bodyClassName="flex flex-1 flex-col"
      className="flex min-h-full flex-col"
      label="Network"
    >
      <Panel
        bodyClassName="flex flex-1 flex-col p-0"
        className="flex-1"
        foot={
          <Button render={<Link to="/network" />} size="sm" variant="ghost">
            View network
            <ChevronRight aria-hidden="true" />
          </Button>
        }
      >
        <div className="flex flex-1 flex-col divide-y divide-line-soft">
          {rows.length === 0 ? (
            <p className="px-4 py-3.5 text-small-body text-subtle">Network coordination is idle.</p>
          ) : (
            rows.map(row => (
              <div
                className="flex min-h-9 items-center justify-between gap-3 px-4 py-2"
                data-slot="home-network-row"
                key={row.key}
              >
                <span className="text-small-body text-muted">{row.label}</span>
                <span className="flex items-center gap-2 text-small-body font-medium text-fg">
                  {row.tone ? <Pill.Dot tone={row.tone} /> : null}
                  <span
                    className={
                      row.mono ? "font-mono text-small-body tabular-nums text-subtle" : undefined
                    }
                  >
                    {row.value}
                  </span>
                </span>
              </div>
            ))
          )}
        </div>
      </Panel>
    </Section>
  );
}
