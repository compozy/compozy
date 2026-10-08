import { CalendarCheck, Repeat, Timer } from "lucide-react";
import { useState } from "react";

import {
  Button,
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  Field,
  FieldDescription,
  Input,
  PillGroup,
  Toggle,
  cn,
  type PillGroupItem,
} from "@compozy/ui";

import type { AutomationFormModel } from "../../../hooks/use-automation-form";
import type { AutomationFormDraft } from "../../../lib/automation-form-draft";
import {
  CRON_PARTS_HINT,
  SCHEDULE_EVERY_PICKS,
  SCHEDULE_QUICK_PICKS,
  scheduleClock,
  scheduleDayTime,
} from "../../../lib/automation-form-schedule";
import { localInputToDate, SCHEDULE_CONSTANTS, toRfc3339 } from "../../../lib/cron-engine";
import type { AutomationScheduleMode } from "../../../types";
import { ChoiceChip } from "../choice-chip";
import { ScheduleReadout } from "./schedule-readout";

const MODE_ITEMS: PillGroupItem<AutomationScheduleMode>[] = [
  { value: "cron", label: withIcon(Repeat, "Repeats"), testId: "automation-schedule-mode-cron" },
  { value: "every", label: withIcon(Timer, "Every…"), testId: "automation-schedule-mode-every" },
  { value: "at", label: withIcon(CalendarCheck, "Once"), testId: "automation-schedule-mode-at" },
];

function withIcon(Icon: typeof Repeat, label: string) {
  return (
    <span className="flex items-center gap-1.5">
      <Icon aria-hidden="true" className="size-3" />
      {label}
    </span>
  );
}

const READOUT_ID = "automation-schedule-readout";

interface ScheduleStartProps {
  draft: AutomationFormDraft;
  form: AutomationFormModel;
}

/** Schedule settings: Repeats (quick picks, days and a time, cron one level deeper), Every…, Once. */
export function ScheduleStart({ draft, form }: ScheduleStartProps) {
  const schedule = draft.schedule;
  return (
    <div className="flex flex-col gap-3">
      <PillGroup
        aria-label="Schedule type"
        items={MODE_ITEMS}
        onChange={form.onScheduleMode}
        size="sm"
        value={schedule.mode}
      />
      {schedule.mode === "cron" ? <RepeatsBuilder expr={schedule.expr ?? ""} form={form} /> : null}
      {schedule.mode === "every" ? (
        <div className="flex flex-col gap-3">
          <div aria-label="Common intervals" className="flex flex-wrap gap-1.5" role="group">
            {SCHEDULE_EVERY_PICKS.map(pick => (
              <ChoiceChip
                className="font-mono"
                key={pick}
                onClick={() => form.onEveryInterval(pick)}
                pressed={schedule.interval === pick}
              >
                {pick}
              </ChoiceChip>
            ))}
          </div>
          <Input
            aria-describedby={READOUT_ID}
            aria-invalid={form.readout?.valid === false}
            aria-label="Interval"
            className="max-w-48 font-mono"
            onChange={event => form.onEveryInterval(event.target.value)}
            placeholder="30m"
            value={schedule.interval ?? ""}
          />
        </div>
      ) : null}
      {schedule.mode === "at" ? (
        <div className="flex flex-wrap items-center gap-2.5">
          <Input
            aria-describedby={READOUT_ID}
            aria-invalid={form.readout?.valid === false}
            aria-label="Date and time"
            className="w-auto"
            onChange={event => form.onAtTime(event.target.value)}
            type="datetime-local"
            value={schedule.time ?? ""}
          />
          <span className="font-mono text-form-hint text-subtle">
            {toRfc3339(localInputToDate(schedule.time ?? ""))}
          </span>
        </div>
      ) : null}
      {form.readout ? <ScheduleReadout id={READOUT_ID} readout={form.readout} /> : null}
    </div>
  );
}

function RepeatsBuilder({ expr, form }: { expr: string; form: AutomationFormModel }) {
  const dayTime = scheduleDayTime(expr);
  const selectedDays = new Set(form.daysCleared ? [] : (dayTime?.days ?? []));
  // A shape the day picker can't show (every hour, a custom cron) keeps it quiet.
  const builderApplies = dayTime !== null || form.daysCleared;
  const [expressionOpen, setExpressionOpen] = useState(!builderApplies && !isQuickPick(expr));

  return (
    <div className="flex flex-col gap-3">
      <div aria-label="Quick picks" className="flex flex-wrap gap-1.5" role="group">
        {SCHEDULE_QUICK_PICKS.map(pick => (
          <ChoiceChip
            key={pick.expr}
            onClick={() => form.onQuickPick(pick.expr)}
            pressed={!form.daysCleared && pick.expr === expr.trim()}
            title={pick.expr}
          >
            {pick.label}
          </ChoiceChip>
        ))}
      </div>
      <div
        className={cn(
          "flex flex-wrap items-center gap-2.5 text-small-body text-muted",
          !builderApplies && "opacity-50"
        )}
        data-testid="automation-schedule-days-row"
      >
        <span>On</span>
        <fieldset className="flex gap-1">
          <legend className="sr-only">Days</legend>
          {SCHEDULE_CONSTANTS.DOW_LONG.map((day, index) => (
            <Toggle
              aria-label={day}
              key={day}
              onPressedChange={() => form.onToggleDay(index)}
              pressed={selectedDays.has(index)}
              size="sm"
            >
              {SCHEDULE_CONSTANTS.DOW_SHORT[index].slice(0, 2)}
            </Toggle>
          ))}
        </fieldset>
        <span>at</span>
        <Input
          aria-label="Time"
          className="w-auto min-w-28 font-mono tabular-nums"
          onChange={event => form.onTime(event.target.value)}
          type="time"
          value={scheduleClock(expr)}
        />
      </div>
      <Collapsible onOpenChange={setExpressionOpen} open={expressionOpen}>
        <CollapsibleTrigger
          render={
            <Button className="h-auto px-0" size="sm" type="button" variant="link">
              Edit expression
            </Button>
          }
        />
        <CollapsibleContent className="pt-2">
          <Field>
            <Input
              aria-describedby={READOUT_ID}
              aria-invalid={form.readout?.valid === false && !form.daysCleared}
              aria-label="Cron expression"
              className="max-w-64 font-mono"
              onChange={event => form.onCronExpr(event.target.value)}
              value={expr}
            />
            <FieldDescription>{CRON_PARTS_HINT}</FieldDescription>
          </Field>
        </CollapsibleContent>
      </Collapsible>
    </div>
  );
}

function isQuickPick(expr: string): boolean {
  return SCHEDULE_QUICK_PICKS.some(pick => pick.expr === expr.trim());
}
