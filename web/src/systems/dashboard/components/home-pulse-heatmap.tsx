import { Panel, Section } from "@compozy/ui";

import { formatHomeDurationSeconds } from "../lib/home-formatters";
import type { HomeOverview, HomePulseBucket } from "../types";
import { HomePulseAxis } from "./home-zone-parts";
import {
  HOME_PULSE_GRID_CLASS,
  HOME_PULSE_GRID_COLUMNS,
  HOME_PULSE_INSIGHTS_CLASS,
  HOME_PULSE_WEEKDAY_LABELS,
  HOME_PULSE_WEEKDAY_ROWS,
} from "./home-zone-shape";

export interface HomePulseHeatmapProps {
  pulse: HomeOverview["pulse"];
}

function cellBackground(events: number, max: number): string | undefined {
  if (events <= 0 || max <= 0) {
    return undefined;
  }
  const share = 5 + Math.round((events / max) * 50);
  return `color-mix(in srgb, var(--color-viz-cell) ${share}%, transparent)`;
}

/**
 * Zone 6 — hour-by-weekday activity over the last 14 days, painted with the
 * neutral viz ink (magnitude only; no signal colors). Insights render only
 * when the daemon derived them — an empty window drops them entirely.
 */
export function HomePulseHeatmap({ pulse }: HomePulseHeatmapProps) {
  const byCell = new Map<string, HomePulseBucket>();
  let max = 0;
  for (const bucket of pulse.buckets) {
    byCell.set(`${bucket.weekday}:${bucket.hour}`, bucket);
    max = Math.max(max, bucket.events);
  }

  const busiestLabel = pulse.busiest
    ? `busiest hour is ${HOME_PULSE_WEEKDAY_LABELS[pulse.busiest.weekday]} ${String(pulse.busiest.hour).padStart(2, "0")}:00`
    : "no events in the window";

  return (
    <Section
      data-slot="home-pulse"
      label="When agents were busy"
      right={<span className="text-eyebrow text-muted-foreground">Last 14 days</span>}
    >
      <Panel bodyClassName="p-0">
        <div className="overflow-x-auto px-4 pt-4 pb-1.5">
          <div
            aria-label={`Activity heatmap by hour and weekday over the last 14 days; ${busiestLabel}`}
            className={HOME_PULSE_GRID_CLASS}
            role="img"
            style={{ gridTemplateColumns: HOME_PULSE_GRID_COLUMNS }}
          >
            {HOME_PULSE_WEEKDAY_ROWS.map(weekday => (
              <HeatmapRow byCell={byCell} key={weekday} max={max} weekday={weekday} />
            ))}
            <HomePulseAxis />
          </div>
        </div>
        {pulse.busiest || pulse.longest_session ? (
          <div className={HOME_PULSE_INSIGHTS_CLASS} data-slot="home-pulse-insights">
            {pulse.busiest ? (
              <span>
                Busiest hour{" "}
                <span className="font-medium text-fg">
                  {HOME_PULSE_WEEKDAY_LABELS[pulse.busiest.weekday]}{" "}
                  {String(pulse.busiest.hour).padStart(2, "0")}:00
                </span>{" "}
                <span className="text-eyebrow tabular-nums text-subtle">
                  {pulse.busiest.events} events
                </span>
              </span>
            ) : null}
            {pulse.busiest && pulse.longest_session ? (
              <span aria-hidden="true" className="size-0.5 rounded-full bg-faint" />
            ) : null}
            {pulse.longest_session ? (
              <span>
                Longest session{" "}
                <span className="font-medium text-fg">
                  {formatHomeDurationSeconds(pulse.longest_session.duration_seconds, "compact")}
                </span>{" "}
                <span className="text-eyebrow tabular-nums text-subtle">
                  {pulse.longest_session.agent_name} · {pulse.longest_session.date}
                </span>
              </span>
            ) : null}
          </div>
        ) : null}
      </Panel>
    </Section>
  );
}

function HeatmapRow({
  weekday,
  byCell,
  max,
}: {
  weekday: number;
  byCell: Map<string, HomePulseBucket>;
  max: number;
}) {
  return (
    <>
      <span className="self-center font-mono text-micro text-faint">
        {HOME_PULSE_WEEKDAY_LABELS[weekday]}
      </span>
      {Array.from({ length: 24 }, (_, hour) => {
        const events = byCell.get(`${weekday}:${hour}`)?.events ?? 0;
        return (
          <div
            className="h-[var(--size-home-pulse-cell)] rounded-xs bg-surface-2"
            key={hour}
            style={{ background: cellBackground(events, max) }}
            title={`${HOME_PULSE_WEEKDAY_LABELS[weekday]} ${String(hour).padStart(2, "0")}:00 · ${events} events`}
          />
        );
      })}
    </>
  );
}
