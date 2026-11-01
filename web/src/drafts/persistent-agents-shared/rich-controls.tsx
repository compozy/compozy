/** The draft's rich controls, composed on `@compozy/ui` and token utilities. */
import type { CSSProperties, ReactNode } from "react";

import { Slider, cn } from "@compozy/ui";

const HATCH: CSSProperties = {
  backgroundImage:
    "repeating-linear-gradient(-45deg, var(--color-line-strong) 0 1.5px, transparent 1.5px 5px)",
};

export interface StopSliderProps {
  label: string;
  stops: readonly string[];
  value: number;
  /** First stop the control can reach; everything before it reads as hatched. */
  floor: number;
  floorLabel?: string;
  onChange: (next: number) => void;
}

/** A stepped slider that stops at a configured floor and shows the floor as a hatched range. */
export function StopSlider({ label, stops, value, floor, floorLabel, onChange }: StopSliderProps) {
  const last = stops.length - 1;
  return (
    <div className="flex flex-col gap-2">
      <div className="relative py-1">
        <Slider<number>
          aria-label={label}
          max={last}
          min={0}
          onValueChange={next => onChange(Math.max(Number(next), floor))}
          step={1}
          value={value}
        />
        {floor > 0 ? (
          <span
            aria-hidden="true"
            className="pointer-events-none absolute top-1/2 left-0 h-2.5 -translate-y-1/2 rounded-l-pill bg-sunken"
            style={{ ...HATCH, width: `${(floor / last) * 100}%` }}
          />
        ) : null}
      </div>
      <div className="relative h-4">
        {stops.map((stop, index) => (
          <span
            className={cn(
              "absolute -translate-x-1/2 font-mono text-micro",
              index === 0 && "translate-x-0",
              index === last && "-translate-x-full",
              index < floor ? "text-faint" : index === value ? "text-fg" : "text-subtle"
            )}
            key={stop}
            style={{ left: `${(index / last) * 100}%` }}
          >
            {stop}
          </span>
        ))}
      </div>
      {floorLabel ? <span className="text-eyebrow text-subtle">{floorLabel}</span> : null}
    </div>
  );
}

/** Discrete cells, filled up to `value`: quiet streaks, failure streaks. */
export function SegmentMeter({
  value,
  total,
  tone = "fg",
  label,
}: {
  value: number;
  total: number;
  tone?: "fg" | "danger" | "warning";
  label: string;
}) {
  const fill = { fg: "bg-fg-2", danger: "bg-danger", warning: "bg-warning" }[tone];
  return (
    <span aria-label={label} className="flex items-center gap-1" role="img">
      {Array.from({ length: total }, (_, index) => (
        <span
          className={cn("h-4 flex-1 rounded-xxs", index < value ? fill : "bg-surface-2")}
          key={index}
        />
      ))}
    </span>
  );
}

/** Used of a budget, with a warning fill near the end. */
export function BudgetBar({
  label,
  value,
  max,
  unit,
}: {
  label: string;
  value: number;
  max: number;
  unit: string;
}) {
  const share = Math.min(1, value / max);
  const near = share >= 0.9;
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-1.5">
      <div className="flex items-baseline justify-between gap-2">
        <span className="text-small-body text-muted">{label}</span>
        <span className="font-mono text-mono-id text-fg-2 tabular-nums">
          <span className={near ? "text-warning" : "text-fg"}>{value}</span> / {max} {unit}
        </span>
      </div>
      <span className="block h-1.5 overflow-hidden rounded-pill bg-surface-2">
        <span
          className={cn("block h-full rounded-pill", near ? "bg-warning" : "bg-fg-2")}
          style={{ width: `${share * 100}%` }}
        />
      </span>
    </div>
  );
}

export function SettingRow({
  title,
  value,
  children,
}: {
  title: string;
  value?: ReactNode;
  children?: ReactNode;
}) {
  return (
    <section className="flex flex-col gap-3">
      <div className="flex items-baseline justify-between gap-3">
        <h3 className="text-small-body font-medium text-fg">{title}</h3>
        {value ? <span className="text-small-body text-muted">{value}</span> : null}
      </div>
      {children}
    </section>
  );
}

export function EditorCard({ className, children }: { className?: string; children: ReactNode }) {
  return (
    <div className={cn("flex flex-col gap-6 rounded-lg bg-card p-5 shadow-card", className)}>
      {children}
    </div>
  );
}
