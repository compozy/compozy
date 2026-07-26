import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";

import { CenteredSurface } from "@/storybook/story-layout";
import {
  compileCron,
  decodeCron,
  fullCronModel,
  humanCron,
  isValidCron,
} from "@/systems/automation/lib/cron-engine";
import type { CronFrequency, CronModel } from "@/systems/automation/lib/cron-engine";

import { CronBuilder } from "../../job-form/cron-builder";

const meta: Meta<typeof CronBuilder> = {
  title: "systems/automation/components/job-form/CronBuilder",
  component: CronBuilder,
  parameters: {
    layout: "fullscreen",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Derive the builder model from a raw expression, falling back to `custom`. */
function modelForExpr(expr: string): CronModel {
  const decoded = decodeCron(expr);
  return decoded ? fullCronModel(decoded) : fullCronModel({ frequency: "custom" });
}

/** Plain-language readout matching what the live preview shows in-form. */
function readoutForExpr(expr: string): string {
  if (!isValidCron(expr)) {
    return "Invalid cron — need 5 fields in range.";
  }
  const human = humanCron(expr);
  return `Runs ${human ?? "on this custom schedule"} · UTC`;
}

function CronBuilderHarness({ initialExpr }: { initialExpr: string }) {
  const [model, setModel] = useState<CronModel>(() => modelForExpr(initialExpr));
  const [expr, setExpr] = useState(initialExpr);

  const valid = isValidCron(expr);
  const readout = readoutForExpr(expr);

  /** Recompile the raw expression from a mutated model (except `custom`). */
  const syncFromModel = (next: CronModel) => {
    setModel(next);
    const compiled = compileCron(next);
    if (compiled !== null) {
      setExpr(compiled);
    }
  };

  const handleFrequency = (frequency: CronFrequency) => {
    syncFromModel({ ...model, frequency });
  };

  const handleExpr = (raw: string) => {
    setExpr(raw);
    const decoded = decodeCron(raw);
    if (decoded) {
      setModel(fullCronModel(decoded));
    } else {
      setModel(current => ({ ...current, frequency: "custom" }));
    }
  };

  const handleEveryMinutes = (minutes: number) => {
    syncFromModel({ ...model, frequency: "minutes", everyMinutes: minutes });
  };

  const handleHourlyMinute = (minute: number) => {
    syncFromModel({ ...model, frequency: "hourly", hourlyMinute: minute });
  };

  const handleDailyTime = (hour: number, minute: number) => {
    syncFromModel({ ...model, frequency: "daily", hour, minute });
  };

  const handleWeeklyTime = (hour: number, minute: number) => {
    syncFromModel({ ...model, frequency: "weekly", hour, minute });
  };

  const handleMonthlyTime = (hour: number, minute: number) => {
    syncFromModel({ ...model, frequency: "monthly", hour, minute });
  };

  const handleToggleWeekday = (weekday: number) => {
    const has = model.weekdays.includes(weekday);
    const weekdays = has
      ? model.weekdays.filter(day => day !== weekday)
      : [...model.weekdays, weekday].sort((a, b) => a - b);
    syncFromModel({ ...model, frequency: "weekly", weekdays });
  };

  const handleWeekdayPreset = (preset: "weekdays" | "weekend" | "all") => {
    const weekdays =
      preset === "weekdays"
        ? [1, 2, 3, 4, 5]
        : preset === "weekend"
          ? [0, 6]
          : [0, 1, 2, 3, 4, 5, 6];
    syncFromModel({ ...model, frequency: "weekly", weekdays });
  };

  const handleMonthDay = (day: number) => {
    syncFromModel({ ...model, frequency: "monthly", monthDay: day });
  };

  const handlePreset = (presetExpr: string) => {
    handleExpr(presetExpr);
  };

  return (
    <CenteredSurface className="items-start justify-center p-6">
      <div className="w-full max-w-(--width-modal-md) rounded-xl border border-line bg-canvas-soft p-5">
        <CronBuilder
          expr={expr}
          model={model}
          onDailyTime={handleDailyTime}
          onEveryMinutes={handleEveryMinutes}
          onExpr={handleExpr}
          onFrequency={handleFrequency}
          onHourlyMinute={handleHourlyMinute}
          onMonthDay={handleMonthDay}
          onMonthlyTime={handleMonthlyTime}
          onPreset={handlePreset}
          onToggleWeekday={handleToggleWeekday}
          onWeekdayPreset={handleWeekdayPreset}
          onWeeklyTime={handleWeeklyTime}
          readout={readout}
          valid={valid}
        />
      </div>
    </CenteredSurface>
  );
}

export const Weekly: Story = {
  args: {},
  render: () => <CronBuilderHarness initialExpr="0 9 * * 1-5" />,
};

export const EveryNMinutes: Story = {
  args: {},
  render: () => <CronBuilderHarness initialExpr="*/15 * * * *" />,
};

export const Custom: Story = {
  args: {},
  render: () => <CronBuilderHarness initialExpr="30 6 1,15 * *" />,
};
