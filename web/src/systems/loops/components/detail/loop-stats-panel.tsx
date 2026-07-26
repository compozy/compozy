import { Section } from "@agh/ui";

import { successRateLabel } from "../../lib/loop-catalog";
import type { LoopAggregate30d } from "../../types";

interface LoopStatsPanelProps {
  successRate: number;
  aggregate: LoopAggregate30d;
}

/**
 * Right-rail "Last 30 days" panel. The daemon projection exposes success rate and
 * the run/succeeded/failed aggregate only; average generations and median
 * duration from the design mock are not in the contract, so the four stats render
 * exactly what the daemon computes (truthful UI, §9.11).
 */
export function LoopStatsPanel({ successRate, aggregate }: LoopStatsPanelProps) {
  const stats = [
    { label: "Success rate", value: successRateLabel(successRate) },
    { label: "Total runs", value: String(aggregate.runs) },
    { label: "Succeeded", value: String(aggregate.succeeded) },
    { label: "Failed", value: String(aggregate.failed) },
  ];
  return (
    <Section label="Last 30 days" data-testid="loop-stats">
      <div className="grid grid-cols-2 rounded-lg border border-line bg-canvas-soft">
        {stats.map((stat, index) => (
          <div
            key={stat.label}
            className={`px-3.5 py-3 ${index >= 2 ? "border-t border-line-soft" : ""} ${
              index % 2 === 0 ? "border-r border-line-soft" : ""
            }`}
          >
            <div className="text-lg font-medium tabular-nums tracking-detail-h1 text-fg-strong">
              {stat.value}
            </div>
            <div className="mt-0.5 text-badge text-faint">{stat.label}</div>
          </div>
        ))}
      </div>
    </Section>
  );
}
