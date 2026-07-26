import type { ReactNode } from "react";

import { cn, Pill, type PillTone } from "@agh/ui";

export interface SettingsHeroGaugeProps {
  /** Headline usage, e.g. "842 MB of 1 GB". */
  usage: ReactNode;
  /** Utilization percent (0–100) driving the bar width. */
  percent: number;
  tone: PillTone;
  /** Status pill label, e.g. "Filling up" / "Healthy". */
  pill: ReactNode;
  /** Dot-separated breakdown line, e.g. "Global DB 512 MB · Sessions 330 MB · 182 MB free". */
  legend?: ReactNode;
  /** Activity foot line, e.g. "Capturing · 3 active sessions · 7-day retention". */
  foot?: ReactNode;
  "data-testid"?: string;
}

const GAUGE_FILL: Record<string, string> = {
  success: "bg-success",
  warning: "bg-warning",
  danger: "bg-danger",
  info: "bg-info",
  accent: "bg-accent",
  neutral: "bg-fg",
};

/**
 * Page-top capacity gauge (prototype `.gauge`): headline usage + percent +
 * tone pill over a tone-colored utilization bar with legend and activity
 * foot. All values come from runtime storage metrics.
 */
export function SettingsHeroGauge({
  usage,
  percent,
  tone,
  pill,
  legend,
  foot,
  "data-testid": testId,
}: SettingsHeroGaugeProps) {
  const clamped = Math.min(100, Math.max(0, percent));

  return (
    <section
      className="flex flex-col gap-3 rounded-lg border border-line bg-canvas-soft p-4"
      data-testid={testId ?? "settings-hero-gauge"}
    >
      <div className="flex min-w-0 items-baseline gap-2.5">
        <span className="min-w-0 truncate text-metric-value font-semibold tabular-nums text-fg-strong">
          {usage}
        </span>
        <span className="font-mono text-small-body tabular-nums text-subtle">
          {Math.round(clamped)}%
        </span>
        <span className="ml-auto shrink-0 self-center">
          <Pill tone={tone}>{pill}</Pill>
        </span>
      </div>
      <div
        aria-hidden="true"
        className="h-2 w-full overflow-hidden rounded-pill bg-bar-fill"
        data-slot="settings-gauge-track"
      >
        <div
          className={cn(
            "h-full rounded-pill transition-[width] duration-slow ease-out",
            GAUGE_FILL[tone] ?? GAUGE_FILL.neutral
          )}
          data-slot="settings-gauge-fill"
          style={{ width: `${clamped}%` }}
        />
      </div>
      {legend ? <div className="text-form-label text-subtle">{legend}</div> : null}
      {foot ? (
        <div className="border-t border-line-soft pt-2.5 text-form-label text-subtle">{foot}</div>
      ) : null}
    </section>
  );
}
