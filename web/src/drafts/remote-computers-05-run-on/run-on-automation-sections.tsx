import { CalendarCheck, Repeat, Timer } from "@compozy/ui/icons";

import {
  Button,
  cn,
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  Field,
  FieldDescription,
  FieldLabel,
  Input,
  PillGroup,
  Textarea,
  Toggle,
  type PillGroupItem,
} from "@compozy/ui";

import { AgentCommandSelect } from "@/systems/agent/components/agent-command-select";
import type { AgentPayload } from "@/systems/agent/types";
import { ScheduleReadout } from "@/systems/automation/components/automation-form/starts/schedule-readout";
import {
  formatClock,
  localInputToDate,
  SCHEDULE_CONSTANTS,
  toRfc3339,
} from "@/systems/automation/lib/cron-engine";
import type { AutomationSchedule, AutomationScheduleMode } from "@/systems/automation/types";

import {
  ALL_DAYS,
  compileDayTime,
  CRON_PARTS_HINT,
  SCHEDULE_EVERY_PICKS,
  SCHEDULE_QUICK_PICKS,
  scheduleDayTime,
} from "./run-on-automation-helpers";

function withIcon(Icon: typeof Repeat, label: string) {
  return (
    <span className="flex items-center gap-1.5">
      <Icon aria-hidden="true" className="size-3" />
      {label}
    </span>
  );
}

const MODE_ITEMS: PillGroupItem<AutomationScheduleMode>[] = [
  { value: "cron", label: withIcon(Repeat, "Repeats"), testId: "automation-schedule-mode-cron" },
  { value: "every", label: withIcon(Timer, "Every…"), testId: "automation-schedule-mode-every" },
  { value: "at", label: withIcon(CalendarCheck, "Once"), testId: "automation-schedule-mode-at" },
];

export interface AutomationScheduleInsetProps {
  schedule: AutomationSchedule;
  daysCleared: boolean;
  expressionOpen: boolean;
  readout: { valid: boolean; text: string } | null;
  targetTimeZone: string;
  onScheduleMode: (mode: AutomationScheduleMode) => void;
  onSchedulePatch: (next: Partial<AutomationSchedule>) => void;
  onDaysClearedChange: (cleared: boolean) => void;
  onExpressionOpenChange: (open: boolean) => void;
}

function WeeklyScheduleControls({
  expr,
  daysCleared,
  targetTimeZone,
  onSchedulePatch,
  onDaysClearedChange,
}: Pick<
  AutomationScheduleInsetProps,
  "daysCleared" | "targetTimeZone" | "onSchedulePatch" | "onDaysClearedChange"
> & { expr: string }) {
  const dayTime = scheduleDayTime(expr);
  const selectedDays = new Set(daysCleared ? [] : (dayTime?.days ?? []));
  const builderApplies = dayTime !== null || daysCleared;
  const clockValue = dayTime ? formatClock(dayTime.hour, dayTime.minute) : "09:00";

  return (
    <div
      className={cn(
        "flex flex-wrap items-center gap-2.5 text-small-body text-muted-foreground",
        !builderApplies && "opacity-50"
      )}
      data-testid="automation-schedule-days-row"
    >
      <span>On</span>
      <fieldset className="flex gap-1" disabled={!builderApplies}>
        <legend className="sr-only">Days</legend>
        {SCHEDULE_CONSTANTS.DOW_LONG.map((day, index) => (
          <Toggle
            aria-label={day}
            disabled={!builderApplies}
            key={day}
            onPressedChange={() => {
              const days = daysCleared ? [] : (dayTime?.days ?? []);
              const nextDays = days.includes(index)
                ? days.filter(value => value !== index)
                : [...days, index];
              if (nextDays.length === 0) {
                onDaysClearedChange(true);
                return;
              }
              onDaysClearedChange(false);
              onSchedulePatch({
                mode: "cron",
                expr: compileDayTime(nextDays, dayTime?.hour ?? 9, dayTime?.minute ?? 0),
              });
            }}
            pressed={selectedDays.has(index)}
            size="sm"
          >
            {SCHEDULE_CONSTANTS.DOW_SHORT[index].slice(0, 2)}
          </Toggle>
        ))}
      </fieldset>
      <span>at</span>
      <span className="inline-flex items-center gap-2">
        <Input
          aria-label="Time"
          className="w-auto min-w-28 font-mono tabular-nums"
          disabled={!builderApplies}
          onChange={event => {
            const match = /^(\d{1,2}):(\d{2})$/.exec(event.target.value);
            if (!match) return;
            const hour = Number(match[1]);
            const minute = Number(match[2]);
            if (hour > 23 || minute > 59) return;
            onSchedulePatch({
              mode: "cron",
              expr: compileDayTime(dayTime?.days ?? [...ALL_DAYS], hour, minute),
            });
          }}
          type="time"
          value={clockValue}
        />
        <span
          className="font-mono text-form-hint text-subtle"
          data-testid="automation-schedule-timezone"
        >
          · {targetTimeZone}
        </span>
      </span>
    </div>
  );
}

/**
 * Mirrors production `ScheduleStart` (`starts/schedule-start.tsx`), adding the target
 * computer's effective automation timezone beside the schedule time (`09:00 · America/Sao_Paulo`).
 */
export function AutomationScheduleInset({
  schedule,
  daysCleared,
  expressionOpen,
  readout,
  targetTimeZone,
  onScheduleMode,
  onSchedulePatch,
  onDaysClearedChange,
  onExpressionOpenChange,
}: AutomationScheduleInsetProps) {
  const expr = schedule.expr ?? "";

  return (
    <div className="rounded-lg bg-sunken p-3.5" data-testid="automation-start-inset-schedule">
      <div className="flex flex-col gap-3">
        <PillGroup
          aria-label="Schedule type"
          items={MODE_ITEMS}
          onChange={onScheduleMode}
          size="sm"
          value={schedule.mode}
        />

        {schedule.mode === "cron" ? (
          <div className="flex flex-col gap-3">
            <div aria-label="Quick picks" className="flex flex-wrap gap-1.5" role="group">
              {SCHEDULE_QUICK_PICKS.map(pick => (
                <Toggle
                  key={pick.expr}
                  onPressedChange={() => {
                    onDaysClearedChange(false);
                    onSchedulePatch({ mode: "cron", expr: pick.expr });
                  }}
                  pressed={!daysCleared && pick.expr === expr.trim()}
                  size="sm"
                  title={pick.expr}
                >
                  {pick.label}
                </Toggle>
              ))}
            </div>

            <WeeklyScheduleControls
              daysCleared={daysCleared}
              expr={expr}
              onDaysClearedChange={onDaysClearedChange}
              onSchedulePatch={onSchedulePatch}
              targetTimeZone={targetTimeZone}
            />

            {readout ? (
              <ScheduleReadout id="automation-schedule-readout" readout={readout} />
            ) : null}

            <Collapsible onOpenChange={onExpressionOpenChange} open={expressionOpen}>
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
                    aria-label="Cron expression"
                    className="max-w-64 font-mono"
                    onChange={event => {
                      onDaysClearedChange(false);
                      onSchedulePatch({ mode: "cron", expr: event.target.value });
                    }}
                    value={expr}
                  />
                  <FieldDescription>{CRON_PARTS_HINT}</FieldDescription>
                </Field>
              </CollapsibleContent>
            </Collapsible>
          </div>
        ) : null}

        {schedule.mode === "every" ? (
          <div className="flex flex-col gap-3">
            <div aria-label="Common intervals" className="flex flex-wrap gap-1.5" role="group">
              {SCHEDULE_EVERY_PICKS.map(pick => (
                <Toggle
                  className="font-mono"
                  key={pick}
                  onPressedChange={() => onSchedulePatch({ interval: pick })}
                  pressed={schedule.interval === pick}
                  size="sm"
                >
                  {pick}
                </Toggle>
              ))}
            </div>
            {readout ? (
              <ScheduleReadout id="automation-schedule-readout" readout={readout} />
            ) : null}
          </div>
        ) : null}

        {schedule.mode === "at" ? (
          <div className="flex flex-wrap items-center gap-2.5">
            <Input
              aria-label="Date and time"
              className="w-auto"
              onChange={event => onSchedulePatch({ time: event.target.value })}
              type="datetime-local"
              value={schedule.time ?? ""}
            />
            <span className="font-mono text-form-hint text-subtle">
              · {targetTimeZone}
              {schedule.time ? ` (${toRfc3339(localInputToDate(schedule.time))})` : ""}
            </span>
          </div>
        ) : null}
      </div>
    </div>
  );
}

/** Mirrors production `AgentTarget` (`does/agent-target.tsx`) with real `AgentCommandSelect`. */
export function AutomationAgentInset({
  agents,
  agentName,
  prompt,
  onAgentChange,
  onPromptChange,
}: {
  agents: AgentPayload[];
  agentName: string;
  prompt: string;
  onAgentChange: (next: string) => void;
  onPromptChange: (next: string) => void;
}) {
  return (
    <div className="rounded-lg bg-sunken p-3.5" data-testid="automation-does-inset">
      <div className="flex flex-col gap-4">
        <Field>
          <FieldLabel htmlFor="automation-agent">Agent</FieldLabel>
          <AgentCommandSelect
            agents={agents}
            onChange={next => onAgentChange(next ?? "")}
            triggerId="automation-agent"
            triggerTestId="automation-agent-input"
            value={agentName || null}
          />
        </Field>
        <Field>
          <FieldLabel htmlFor="automation-prompt">Message</FieldLabel>
          <Textarea
            className="min-h-20"
            data-testid="automation-prompt-input"
            id="automation-prompt"
            onChange={event => onPromptChange(event.target.value)}
            value={prompt}
          />
          <FieldDescription>Sent as written.</FieldDescription>
        </Field>
      </div>
    </div>
  );
}
