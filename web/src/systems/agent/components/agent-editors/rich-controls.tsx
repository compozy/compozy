/** The editors' rich controls, composed on `@compozy/ui` and token utilities. */
import type { LucideIcon } from "@compozy/ui/icons";
import type { CSSProperties, ReactNode } from "react";

import { Slider, Switch, cn } from "@compozy/ui";

import type { DayCell } from "../../lib/agent-check-in-activity";
import { formatClock } from "../../lib/agent-check-in-form";

const THUMB = 16;
const BIG_SLIDER = cn(
  "[&_[data-slot=slider-track]]:h-1.5",
  "[&_[data-slot=slider-thumb]]:size-4 [&_[data-slot=slider-thumb]]:border-0 [&_[data-slot=slider-thumb]]:bg-fg-strong"
);

/** The slider keeps its thumb inside the track, so stops sit inside a thumb-wide inset. */
function stopLeft(ratio: number) {
  return `calc(${THUMB / 2}px + (100% - ${THUMB}px) * ${ratio})`;
}

function hatch(color: string): CSSProperties {
  return {
    backgroundColor: "var(--color-sunken)",
    backgroundImage: `repeating-linear-gradient(-45deg, ${color} 0 1.5px, transparent 1.5px 5px)`,
  };
}

function Ticks({ count, major }: { count: number; major: number }) {
  return (
    <span aria-hidden="true" className="relative block h-1.5">
      {Array.from({ length: count + 1 }, (_, index) => (
        <span
          className={cn(
            "absolute top-0 w-px -translate-x-1/2",
            index % major === 0 ? "h-1.5 bg-line-strong" : "h-1 bg-line"
          )}
          key={index}
          style={{ left: stopLeft(index / count) }}
        />
      ))}
    </span>
  );
}

function ScaleLabel({
  children,
  ratio,
  className,
}: {
  children: ReactNode;
  ratio: number;
  className?: string;
}) {
  const edge = ratio === 0 ? { left: 0 } : ratio === 1 ? { right: 0 } : { left: stopLeft(ratio) };
  return (
    <span
      className={cn(
        "absolute whitespace-nowrap text-eyebrow tabular-nums",
        ratio > 0 && ratio < 1 && "-translate-x-1/2",
        className
      )}
      style={edge}
    >
      {children}
    </span>
  );
}

export interface StopSliderProps {
  label: string;
  stops: readonly string[];
  value: number;
  /** First stop the control can reach; everything before it reads as hatched. */
  floor: number;
  /** The word that sits beside the floor's stop, the way a cap is named on a scale. */
  floorWord?: string;
  /** The person pushed past the floor: the limit lights up. */
  atLimit?: boolean;
  disabled?: boolean;
  onChange: (next: number) => void;
  /** Fires when a move would have crossed the floor. */
  onLimit?: () => void;
}

/** A stepped slider with ticks that stops at a floor and shows what lies past it as hatched. */
export function StopSlider({
  label,
  stops,
  value,
  floor,
  floorWord,
  atLimit = false,
  disabled = false,
  onChange,
  onLimit,
}: StopSliderProps) {
  const last = stops.length - 1;
  return (
    <div className="flex flex-col gap-2">
      <div className="relative">
        <Slider<number>
          aria-label={label}
          className={BIG_SLIDER}
          disabled={disabled}
          max={last}
          min={0}
          onValueChange={next => {
            const wanted = Number(next);
            onChange(Math.max(wanted, floor));
            if (wanted < floor) onLimit?.();
          }}
          step={1}
          value={value}
        />
        {floor > 0 ? (
          <span
            aria-hidden="true"
            className="pointer-events-none absolute top-1/2 left-0 h-2.5 -translate-y-1/2 rounded-l-pill"
            style={{
              ...hatch(atLimit ? "var(--color-warning)" : "var(--color-line-strong)"),
              width: `calc(${stopLeft(floor / last)} - ${THUMB / 2 + 2}px)`,
            }}
          />
        ) : null}
      </div>
      <Ticks count={last * 4} major={4} />
      <div aria-hidden="true" className="relative h-4">
        {stops.map((stop, index) => (
          <ScaleLabel
            className={
              index < floor
                ? "text-faint"
                : index === floor && atLimit
                  ? "font-medium text-warning"
                  : index === value
                    ? "text-fg"
                    : "text-subtle"
            }
            key={stop}
            ratio={index / last}
          >
            {index === floor && floorWord ? `${stop} ${floorWord}` : stop}
          </ScaleLabel>
        ))}
      </div>
    </div>
  );
}

export interface DayRangeSliderProps {
  label: string;
  value: readonly [number, number];
  /** Hours inside the range that stay skipped, drawn hatched. */
  gap?: readonly [number, number];
  /** Hour of the day to mark on the scale. */
  now?: number;
  disabled?: boolean;
  onChange: (next: readonly [number, number]) => void;
}

/** A day from midnight to midnight with a draggable start and end. */
export function DayRangeSlider({
  label,
  value,
  gap,
  now,
  disabled = false,
  onChange,
}: DayRangeSliderProps) {
  const inside = gap && gap[0] >= value[0] && gap[1] <= value[1];
  return (
    <div className="flex flex-col gap-2">
      <div className="relative">
        <Slider<readonly number[]>
          aria-label={label}
          className={BIG_SLIDER}
          disabled={disabled}
          getAriaLabel={index => (index === 0 ? "Starts at" : "Ends at")}
          max={24}
          min={0}
          minStepsBetweenValues={2}
          onValueChange={next => {
            const [start, end] = next as readonly number[];
            onChange([start ?? value[0], end ?? value[1]]);
          }}
          step={0.5}
          value={value}
        />
        {inside ? (
          <span
            aria-hidden="true"
            className="pointer-events-none absolute top-1/2 h-2.5 -translate-y-1/2"
            style={{
              ...hatch("var(--color-line-strong)"),
              left: stopLeft(gap[0] / 24),
              width: `calc((100% - ${THUMB}px) * ${(gap[1] - gap[0]) / 24})`,
            }}
          />
        ) : null}
      </div>
      <div aria-hidden="true" className="relative">
        <Ticks count={24} major={6} />
        {now === undefined ? null : (
          <span
            className="absolute top-0 h-1.5 w-0.5 -translate-x-1/2 rounded-pill bg-brand"
            style={{ left: stopLeft(now / 24) }}
          />
        )}
      </div>
      <div aria-hidden="true" className="relative h-4">
        {[0, 6, 12, 18, 24].map(hour => (
          <ScaleLabel className="text-subtle" key={hour} ratio={hour / 24}>
            {formatClock(hour)}
          </ScaleLabel>
        ))}
      </div>
    </div>
  );
}

const CELL: Record<DayCell, string> = {
  quiet: "bg-faint",
  told: "bg-fg-strong",
  failed: "bg-destructive",
  next: "bg-brand",
  later: "bg-surface-2",
};

/** One cell per moment of a day, or per attempt: what happened and what is still ahead. */
export function DayStrip({ cells, label }: { cells: readonly DayCell[]; label: string }) {
  return (
    <span aria-label={label} className="flex h-6 w-full gap-0.5" role="img">
      {cells.map((cell, index) => (
        <span className={cn("min-w-px flex-1 rounded-xxs", CELL[cell])} key={index} />
      ))}
    </span>
  );
}

export function DayStripKey({ cell, children }: { cell: DayCell; children: ReactNode }) {
  return (
    <span className="flex items-center gap-1.5 text-eyebrow text-muted-foreground">
      <span aria-hidden="true" className={cn("size-2 rounded-xxs", CELL[cell])} />
      {children}
    </span>
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
        <h3 className="text-item-title font-medium text-fg">{title}</h3>
        {value ? (
          <span className="text-small-body text-muted-foreground tabular-nums">{value}</span>
        ) : null}
      </div>
      {children}
    </section>
  );
}

export function ToggleRow({
  icon: Icon,
  label,
  checked,
  disabled = false,
  onChange,
}: {
  icon: LucideIcon;
  label: string;
  checked: boolean;
  disabled?: boolean;
  onChange: (next: boolean) => void;
}) {
  return (
    <label
      className={cn(
        "flex min-h-10 items-center gap-3 py-1.5",
        disabled ? "opacity-50" : "cursor-pointer"
      )}
    >
      <Icon aria-hidden="true" className="size-4 shrink-0 text-muted-foreground" />
      <span
        className={cn(
          "min-w-0 flex-1 text-small-body",
          checked ? "text-fg" : "text-muted-foreground"
        )}
      >
        {label}
      </span>
      <Switch checked={checked} disabled={disabled} onCheckedChange={onChange} />
    </label>
  );
}

export function EditorCard({ className, children }: { className?: string; children: ReactNode }) {
  return (
    <div className={cn("flex flex-col gap-6 rounded-lg bg-card p-5 shadow-card", className)}>
      {children}
    </div>
  );
}
