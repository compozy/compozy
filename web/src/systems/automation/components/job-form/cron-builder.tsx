import {
  AlertTriangle,
  CalendarDays,
  CalendarRange,
  Check,
  Clock,
  Code2,
  Sun,
  Zap,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";

import { cn, Input, NativeSelect, NativeSelectOption } from "@agh/ui";

import type { CronFrequency, CronModel } from "../../lib/cron-engine";
import { formatClock, SCHEDULE_CONSTANTS } from "../../lib/cron-engine";

type WeekdayPreset = "weekdays" | "weekend" | "all";

interface CronBuilderProps {
  model: CronModel;
  expr: string;
  valid: boolean;
  readout: string;
  onFrequency: (frequency: CronFrequency) => void;
  onExpr: (expr: string) => void;
  onEveryMinutes: (minutes: number) => void;
  onHourlyMinute: (minute: number) => void;
  onDailyTime: (hour: number, minute: number) => void;
  onWeeklyTime: (hour: number, minute: number) => void;
  onMonthlyTime: (hour: number, minute: number) => void;
  onToggleWeekday: (weekday: number) => void;
  onWeekdayPreset: (preset: WeekdayPreset) => void;
  onMonthDay: (day: number) => void;
  onPreset: (expr: string) => void;
}

interface CronPreset {
  label: string;
  expr: string;
}

interface FrequencyOption {
  freq: CronFrequency;
  label: string;
  icon: LucideIcon;
}

const CRON_PRESETS: CronPreset[] = [
  { label: "Weekdays 9am", expr: "0 9 * * 1-5" },
  { label: "Daily 9am", expr: "0 9 * * *" },
  { label: "Hourly", expr: "0 * * * *" },
  { label: "Every 15 min", expr: "*/15 * * * *" },
  { label: "Mondays 8am", expr: "0 8 * * 1" },
  { label: "Midnight", expr: "0 0 * * *" },
];

const FREQUENCIES: FrequencyOption[] = [
  { freq: "minutes", label: "Minutes", icon: Zap },
  { freq: "hourly", label: "Hourly", icon: Clock },
  { freq: "daily", label: "Daily", icon: Sun },
  { freq: "weekly", label: "Weekly", icon: CalendarDays },
  { freq: "monthly", label: "Monthly", icon: CalendarRange },
  { freq: "custom", label: "Custom", icon: Code2 },
];

const EVERY_MINUTE_OPTIONS = [1, 2, 5, 10, 15, 20, 30];
const HOURLY_MINUTE_OPTIONS = [0, 5, 10, 15, 20, 30, 45];
const MONTH_DAY_OPTIONS = Array.from({ length: 31 }, (_, index) => index + 1);
const CRON_LEGEND = ["min", "hour", "day", "month", "weekday"];

const CHIP_BASE =
  "inline-flex items-center gap-1.5 rounded-pill border px-2.5 py-1 text-form-label font-medium transition-colors outline-none focus-visible:shadow-focus-ring";
const CHIP_RESTING = "border-line-soft bg-canvas-tint text-muted hover:bg-elevated hover:text-fg";
const CHIP_SELECTED = "border-accent-dim bg-accent-tint text-accent-strong";

const FREQ_BASE =
  "inline-flex items-center gap-1.5 rounded-md border px-2.5 py-1.5 text-small-body font-medium transition-colors outline-none focus-visible:shadow-focus-ring";
const FREQ_RESTING = "border-line-soft bg-canvas-tint text-muted hover:bg-elevated hover:text-fg";
const FREQ_SELECTED = "bg-accent-tint text-accent-strong ring-1 ring-accent-dim ring-inset";

const ROW_SHELL =
  "mb-3 flex flex-wrap items-center gap-2.5 rounded-md border border-line-soft bg-canvas-tint px-3.5 py-3";
const TIME_INPUT = "h-8 w-auto min-w-[120px] font-mono tabular-nums [color-scheme:dark]";
const SEL_TRIGGER = "[&_select]:h-8 [&_select]:min-w-[72px]";

/** Split an `HH:MM` value into `{ hour, minute }`, or `null` for malformed input. */
function parseTime(value: string): { hour: number; minute: number } | null {
  const match = value.match(/^(\d{1,2}):(\d{2})$/);
  if (!match) {
    return null;
  }
  const hour = Number.parseInt(match[1], 10);
  const minute = Number.parseInt(match[2], 10);
  if (hour < 0 || hour > 23 || minute < 0 || minute > 59) {
    return null;
  }
  return { hour, minute };
}

function handleTime(value: string, commit: (hour: number, minute: number) => void) {
  const parsed = parseTime(value);
  if (parsed) {
    commit(parsed.hour, parsed.minute);
  }
}

/**
 * Visual 5-field cron builder. Pure and presentational: every value and handler
 * arrives via props. The parent owns the {@link CronModel} ↔ raw-string sync;
 * this component only renders controls and forwards intent.
 */
export function CronBuilder({
  model,
  expr,
  valid,
  readout,
  onFrequency,
  onExpr,
  onEveryMinutes,
  onHourlyMinute,
  onDailyTime,
  onWeeklyTime,
  onMonthlyTime,
  onToggleWeekday,
  onWeekdayPreset,
  onMonthDay,
  onPreset,
}: CronBuilderProps) {
  const isCustom = model.frequency === "custom";
  const clock = formatClock(model.hour, model.minute);
  const showMonthWarning = model.frequency === "monthly" && model.monthDay >= 29;
  const selectedWeekdays = new Set(model.weekdays);

  return (
    <div>
      <div className="mb-3 flex flex-wrap gap-1.5" aria-label="Common schedules">
        {CRON_PRESETS.map(preset => {
          const pressed = preset.expr === expr;
          return (
            <button
              key={preset.expr}
              aria-pressed={pressed}
              className={cn(CHIP_BASE, pressed ? CHIP_SELECTED : CHIP_RESTING)}
              onClick={() => onPreset(preset.expr)}
              type="button"
            >
              {preset.label}
              <span
                className={cn(
                  "font-mono text-form-hint",
                  pressed ? "text-accent-strong opacity-75" : "text-subtle"
                )}
              >
                {preset.expr}
              </span>
            </button>
          );
        })}
      </div>

      <div className="eyebrow mb-2 mt-3.5 text-faint">Or build the rhythm</div>

      <fieldset className="mb-3 flex flex-wrap gap-1.5">
        <legend className="sr-only">Frequency</legend>
        {FREQUENCIES.map(({ freq, label, icon: Icon }) => {
          const pressed = model.frequency === freq;
          return (
            <button
              key={freq}
              aria-pressed={pressed}
              className={cn(FREQ_BASE, pressed ? FREQ_SELECTED : FREQ_RESTING)}
              onClick={() => onFrequency(freq)}
              type="button"
            >
              <Icon aria-hidden="true" className="size-4" />
              {label}
            </button>
          );
        })}
      </fieldset>

      {model.frequency === "minutes" ? (
        <div className={ROW_SHELL}>
          <span className="text-small-body font-medium text-fg">Every</span>
          <NativeSelect
            aria-label="Minute interval"
            className={SEL_TRIGGER}
            onChange={event => onEveryMinutes(Number(event.target.value))}
            value={model.everyMinutes}
          >
            {EVERY_MINUTE_OPTIONS.map(value => (
              <NativeSelectOption key={value} value={value}>
                {value}
              </NativeSelectOption>
            ))}
          </NativeSelect>
          <span className="text-small-body font-medium text-muted">minutes</span>
        </div>
      ) : null}

      {model.frequency === "hourly" ? (
        <div className={ROW_SHELL}>
          <span className="text-small-body font-medium text-fg">Every hour at minute</span>
          <NativeSelect
            aria-label="Minute of the hour"
            className={SEL_TRIGGER}
            onChange={event => onHourlyMinute(Number(event.target.value))}
            value={model.hourlyMinute}
          >
            {HOURLY_MINUTE_OPTIONS.map(value => (
              <NativeSelectOption key={value} value={value}>
                {String(value).padStart(2, "0")}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        </div>
      ) : null}

      {model.frequency === "daily" ? (
        <div className={ROW_SHELL}>
          <span className="text-small-body font-medium text-fg">Every day at</span>
          <Input
            aria-label="Daily run time"
            className={TIME_INPUT}
            onChange={event => handleTime(event.target.value, onDailyTime)}
            type="time"
            value={clock}
          />
        </div>
      ) : null}

      {model.frequency === "weekly" ? (
        <div className={cn(ROW_SHELL, "flex-col items-stretch gap-3")}>
          <fieldset className="grid grid-cols-7 gap-1.5">
            <legend className="sr-only">Days of week</legend>
            {SCHEDULE_CONSTANTS.DOW_SHORT.map((label, day) => {
              const pressed = selectedWeekdays.has(day);
              const weekday = SCHEDULE_CONSTANTS.DOW_LONG[day];
              return (
                <button
                  key={weekday}
                  aria-label={weekday}
                  aria-pressed={pressed}
                  className={cn(
                    "h-9 rounded-sm border text-small-body font-semibold transition-colors outline-none focus-visible:shadow-focus-ring",
                    pressed
                      ? "border-accent-dim bg-accent-tint text-accent-strong"
                      : "border-line bg-elevated text-muted hover:bg-canvas-soft hover:text-fg"
                  )}
                  onClick={() => onToggleWeekday(day)}
                  type="button"
                >
                  {label.slice(0, 2)}
                </button>
              );
            })}
          </fieldset>
          <div className="flex flex-wrap items-center justify-between gap-2.5">
            <div className="flex flex-wrap gap-1.5">
              {(
                [
                  { preset: "weekdays", label: "Weekdays" },
                  { preset: "weekend", label: "Weekend" },
                  { preset: "all", label: "Every day" },
                ] satisfies { preset: WeekdayPreset; label: string }[]
              ).map(({ preset, label }) => (
                <button
                  key={preset}
                  className="rounded-pill border border-line-soft px-2.5 py-1 text-form-label font-medium text-subtle transition-colors outline-none hover:bg-elevated hover:text-fg focus-visible:shadow-focus-ring"
                  onClick={() => onWeekdayPreset(preset)}
                  type="button"
                >
                  {label}
                </button>
              ))}
            </div>
            <div className="flex items-center gap-2.5">
              <span className="text-small-body font-medium text-muted">at</span>
              <Input
                aria-label="Weekly run time"
                className={TIME_INPUT}
                onChange={event => handleTime(event.target.value, onWeeklyTime)}
                type="time"
                value={clock}
              />
            </div>
          </div>
        </div>
      ) : null}

      {model.frequency === "monthly" ? (
        <div className={ROW_SHELL}>
          <span className="text-small-body font-medium text-fg">On day</span>
          <NativeSelect
            aria-label="Day of the month"
            className={SEL_TRIGGER}
            onChange={event => onMonthDay(Number(event.target.value))}
            value={model.monthDay}
          >
            {MONTH_DAY_OPTIONS.map(value => (
              <NativeSelectOption key={value} value={value}>
                {value}
              </NativeSelectOption>
            ))}
          </NativeSelect>
          <span className="text-small-body font-medium text-muted">at</span>
          <Input
            aria-label="Monthly run time"
            className={TIME_INPUT}
            onChange={event => handleTime(event.target.value, onMonthlyTime)}
            type="time"
            value={clock}
          />
        </div>
      ) : null}

      <div>
        <div className="mb-1.5 flex items-center justify-between">
          <span className="eyebrow text-faint">Cron expression</span>
          {isCustom ? null : (
            <button
              className="rounded-xs px-1.5 py-0.5 text-form-label font-medium text-accent-strong transition-colors outline-none hover:bg-accent-tint focus-visible:shadow-focus-ring"
              onClick={() => onFrequency("custom")}
              type="button"
            >
              Edit raw →
            </button>
          )}
        </div>
        <Input
          aria-describedby="job-cron-readout"
          aria-invalid={!valid}
          aria-label="Cron expression"
          className={cn("font-mono", isCustom ? "" : "border-dashed bg-canvas-tint text-muted")}
          onChange={event => onExpr(event.target.value)}
          readOnly={!isCustom}
          value={expr}
        />
        <div className="mt-2 grid grid-cols-5 gap-2">
          {CRON_LEGEND.map(cell => (
            <span key={cell} className="eyebrow text-center text-subtle">
              {cell}
            </span>
          ))}
        </div>
      </div>

      <output
        aria-live="polite"
        className={cn(
          "mt-2.5 flex items-center gap-1.5 text-small-body leading-snug",
          valid ? "text-success" : "text-danger"
        )}
        id="job-cron-readout"
      >
        {valid ? (
          <Check aria-hidden="true" className="size-3 shrink-0" />
        ) : (
          <AlertTriangle aria-hidden="true" className="size-3 shrink-0" />
        )}
        <span>{readout}</span>
      </output>

      {showMonthWarning ? (
        <div className="mt-2 flex items-center gap-1.5 text-form-hint leading-snug text-warning">
          <AlertTriangle aria-hidden="true" className="size-3 shrink-0" />
          <span>Months shorter than this day are skipped; the run won&apos;t fire that month.</span>
        </div>
      ) : null}

      <p className="mt-3 text-form-hint leading-snug text-subtle">
        Pick a frequency and we compile the <span className="font-medium text-muted">5-field</span>{" "}
        cron for you. Choose <span className="font-medium text-muted">Custom</span> to write the
        expression directly: no seconds field, no{" "}
        <code className="font-mono text-mono-id">@daily</code> macros.
      </p>
    </div>
  );
}
