"use client";

import * as React from "react";

import { toneText } from "../../lib/tone";
import { cn } from "../../lib/utils";
import { Surface, type SurfaceSize } from "../surface";
import { Eyebrow } from "./eyebrow";

type IconComponent = React.ComponentType<{ className?: string; size?: number }>;

export type MetricTone = "default" | "accent" | "success" | "warning" | "danger";
export type MetricSize = "compact" | "default" | "lg";
export type MetricLabelCase = "sentence" | "eyebrow";

export interface MetricProps extends Omit<React.ComponentProps<"div">, "title"> {
  label: React.ReactNode;
  value: React.ReactNode;
  /** Small inline detail baseline-aligned with the value — mono micro-unit (e.g. "+12%"). */
  detail?: React.ReactNode;
  /** Secondary line rendered below the value. */
  subtext?: React.ReactNode;
  /** Optional leading head glyph (dashboard KPI voice). */
  icon?: IconComponent;
  /** Optional trailing head slot — delta, sparkline, or link. */
  trailing?: React.ReactNode;
  tone?: MetricTone;
  /**
   * `compact` — 17px value, in-line KPI density.
   * `default` / `lg` — 24px display value (`--text-kpi-value`) at
   * `--font-weight-display` (620), the approved dashboard scale.
   */
  size?: MetricSize;
  /** `sentence` — Inter sentence-case label; `eyebrow` — uppercase KPI label. */
  labelCase?: MetricLabelCase;
}

function metricValueToneClass(tone: MetricTone): string {
  return tone === "default" ? "text-fg-strong" : toneText(tone);
}

/**
 * Metric tile — one voice for every KPI and stat. Label (sentence or eyebrow) +
 * display value at `--text-kpi-value` / `--font-weight-display` + optional
 * inline detail, subtext, head icon, and trailing slot. Composes `Surface`;
 * semantic tone colors the value.
 */
function Metric({
  label,
  value,
  detail,
  subtext,
  icon: Icon,
  trailing,
  tone = "default",
  size = "default",
  labelCase = "sentence",
  className,
  ...props
}: MetricProps) {
  const surfaceSize: SurfaceSize = size === "compact" ? "compact" : "default";
  const valueSizeClass =
    size === "compact"
      ? "text-kpi-compact leading-none tracking-tight"
      : "text-kpi-value leading-(--text-kpi-value--line-height) tracking-detail-h1";
  return (
    <Surface
      size={surfaceSize}
      data-slot="metric"
      data-tone={tone}
      data-size={size}
      className={cn("flex min-w-0 flex-col gap-2", className)}
      {...props}
    >
      <div data-slot="metric-head" className="flex min-w-0 items-center gap-2">
        {Icon ? (
          <span
            aria-hidden="true"
            data-slot="metric-icon"
            className="inline-flex size-5 shrink-0 items-center justify-center text-muted"
          >
            <Icon className="size-3" />
          </span>
        ) : null}
        {labelCase === "eyebrow" ? (
          <Eyebrow data-slot="metric-label" className="min-w-0 truncate text-muted">
            {label}
          </Eyebrow>
        ) : (
          <span
            data-slot="metric-label"
            className="block min-w-0 truncate text-form-label text-muted"
          >
            {label}
          </span>
        )}
        {trailing ? (
          <span data-slot="metric-trailing" className="ml-auto inline-flex shrink-0 items-center">
            {trailing}
          </span>
        ) : null}
      </div>
      <div data-slot="metric-value-row" className="flex min-w-0 items-baseline gap-2">
        <span
          data-slot="metric-value"
          className={cn(
            "min-w-0 truncate tabular-nums",
            valueSizeClass,
            metricValueToneClass(tone)
          )}
          style={{ fontWeight: "var(--font-weight-display)" }}
        >
          {value}
        </span>
        {detail !== undefined ? (
          <span
            data-slot="metric-detail"
            className="shrink-0 truncate font-mono text-eyebrow leading-4 text-subtle"
          >
            {detail}
          </span>
        ) : null}
      </div>
      {subtext !== undefined ? (
        <p data-slot="metric-subtext" className="truncate text-small-body leading-5 text-muted">
          {subtext}
        </p>
      ) : null}
    </Surface>
  );
}

export { Metric };
