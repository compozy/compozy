import { Panel, Section } from "@agh/ui";

import { formatHomeDurationSeconds } from "../lib/home-formatters";
import type { HomeOverview, HomePulseBucket } from "../types";

export interface HomePulseHeatmapProps {
  pulse: HomeOverview["pulse"];
}

const WEEKDAY_LABELS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"] as const;
/** Monday-first display order over the backend's Sunday-first weekday index. */
const WEEKDAY_ROWS = [1, 2, 3, 4, 5, 6, 0] as const;
/** Fixed weekday-label track + 24 hour cells, sized by the home-pulse tokens. */
const PULSE_GRID_COLUMNS =
  "var(--size-home-pulse-label) repeat(24, minmax(var(--size-home-pulse-cell-min), 1fr))";

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
    ? `busiest hour is ${WEEKDAY_LABELS[pulse.busiest.weekday]} ${String(pulse.busiest.hour).padStart(2, "0")}:00`
    : "no events in the window";

  return (
    <Section
      data-slot="home-pulse"
      label="Pulse · hour × weekday"
      right={<span className="text-micro text-faint">last 14 days — bounded by retention</span>}
    >
      <Panel bodyClassName="p-0">
        <div className="overflow-x-auto px-4 pt-4 pb-1.5">
          <div
            aria-label={`Activity heatmap by hour and weekday over the last 14 days; ${busiestLabel}`}
            className="grid min-w-[var(--size-home-pulse-min-w)] gap-[var(--space-home-pulse-gap)]"
            role="img"
            style={{ gridTemplateColumns: PULSE_GRID_COLUMNS }}
          >
            {WEEKDAY_ROWS.map(weekday => (
              <HeatmapRow byCell={byCell} key={weekday} max={max} weekday={weekday} />
            ))}
            <span aria-hidden="true" />
            {Array.from({ length: 24 }, (_, hour) => (
              <span
                aria-hidden="true"
                className="pt-0.5 text-left font-mono text-micro text-faint"
                key={hour}
              >
                {hour % 6 === 0 ? String(hour).padStart(2, "0") : ""}
              </span>
            ))}
          </div>
        </div>
        {pulse.busiest || pulse.longest_session ? (
          <div
            className="flex flex-wrap items-center gap-2.5 border-t border-line-soft px-4 py-2.5 text-small-body text-muted"
            data-slot="home-pulse-insights"
          >
            {pulse.busiest ? (
              <span>
                Busiest hour{" "}
                <span className="font-medium text-fg">
                  {WEEKDAY_LABELS[pulse.busiest.weekday]}{" "}
                  {String(pulse.busiest.hour).padStart(2, "0")}:00
                </span>{" "}
                <span className="font-mono text-micro tabular-nums text-subtle">
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
                <span className="font-mono text-micro tabular-nums text-subtle">
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
      <span className="self-center font-mono text-micro text-faint">{WEEKDAY_LABELS[weekday]}</span>
      {Array.from({ length: 24 }, (_, hour) => {
        const events = byCell.get(`${weekday}:${hour}`)?.events ?? 0;
        return (
          <div
            className="h-[var(--size-home-pulse-cell)] rounded-xs bg-input-fill"
            key={hour}
            style={{ background: cellBackground(events, max) }}
            title={`${WEEKDAY_LABELS[weekday]} ${String(hour).padStart(2, "0")}:00 · ${events} events`}
          />
        );
      })}
    </>
  );
}
