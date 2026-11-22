import {
  Bot,
  CalendarCheck,
  Clock3,
  Radio,
  Repeat,
  Repeat2,
  SquareCheck,
  Timer,
  Webhook,
} from "@compozy/ui/icons";
import type { ComponentProps } from "react";

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
import type { AutomationChoice } from "@/systems/automation/components/automation-form/automation-choice-cards";
import { ScheduleReadout } from "@/systems/automation/components/automation-form/starts/schedule-readout";
import {
  compileCron,
  cronNext,
  decodeCron,
  formatClock,
  formatRelative,
  localInputToDate,
  parseCron,
  parseDuration,
  SCHEDULE_CONSTANTS,
  toRfc3339,
} from "@/systems/automation/lib/cron-engine";
import { humanCron } from "@/systems/automation/lib/cron-engine-presentation";
import type { AutomationSchedule, AutomationScheduleMode } from "@/systems/automation/types";

export type DraftAutomationStart = "schedule" | "event" | "webhook";
export type DraftAutomationDoes = "agent" | "always-on" | "loop" | "task";

export const SCHEDULE_QUICK_PICKS = [
  { label: "Weekdays 9am", expr: "0 9 * * 1-5" },
  { label: "Every day 9am", expr: "0 9 * * *" },
  { label: "Mondays 8am", expr: "0 8 * * 1" },
  { label: "Every hour", expr: "0 * * * *" },
  { label: "Midnight", expr: "0 0 * * *" },
] as const;

export const SCHEDULE_EVERY_PICKS = ["5m", "15m", "30m", "1h", "4h", "24h"] as const;
const CRON_PARTS_HINT = "min · hour · day · month · weekday";
const ALL_DAYS = [0, 1, 2, 3, 4, 5, 6] as const;

function RadarIcon(props: ComponentProps<typeof Radio>) {
  return <Radio {...props} />;
}

export const AUTOMATION_START_CHOICES: readonly AutomationChoice<DraftAutomationStart>[] = [
  {
    value: "schedule",
    icon: Clock3,
    title: "On a schedule",
    description: "Every morning, every hour, or once at a set time.",
  },
  {
    value: "event",
    icon: Radio,
    title: "When something happens",
    description: "A session starts or stops, a hook finishes, an extension sends an event.",
  },
  {
    value: "webhook",
    icon: Webhook,
    title: "When another app calls a link",
    description: "GitHub, CI or any service sends a signed request.",
  },
];

export const AUTOMATION_DOES_CHOICES: readonly AutomationChoice<DraftAutomationDoes>[] = [
  {
    value: "agent",
    icon: Bot,
    title: "Ask an agent",
    description: "Send a message to an agent in a new session.",
  },
  {
    value: "always-on",
    icon: RadarIcon,
    title: "Tell an always-on agent",
    description: "Its next check reads the event and proposes what to do.",
    lockedReason: "Only events and links can reach an always-on agent",
  },
  {
    value: "loop",
    icon: Repeat2,
    title: "Start a Loop",
    description: "Run one of your Loops with set inputs.",
  },
  {
    value: "task",
    icon: SquareCheck,
    title: "Create a task",
    description: "Add a task for an agent pool or a person.",
  },
];

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

export function scheduleDayTime(expr: string) {
  const model = decodeCron(expr);
  if (!model || model.hour === undefined || model.minute === undefined) return null;
  if (model.frequency === "daily") {
    return { days: [...ALL_DAYS], hour: model.hour, minute: model.minute };
  }
  if (model.frequency === "weekly" && model.weekdays) {
    return { days: [...model.weekdays], hour: model.hour, minute: model.minute };
  }
  return null;
}

function compileDayTime(days: readonly number[], hour: number, minute: number): string {
  const sorted = [...new Set(days)].sort((a, b) => a - b);
  return (
    compileCron({
      frequency: sorted.length === 7 ? "daily" : "weekly",
      everyMinutes: 15,
      hourlyMinute: 0,
      hour,
      minute,
      weekdays: sorted,
      monthDay: 1,
    }) ?? ""
  );
}

export function describeSchedulePhrase(schedule: AutomationSchedule, timeZone: string): string {
  if (schedule.mode === "cron") {
    const human = schedule.expr ? humanCron(schedule.expr) : null;
    if (!human) return "On a custom schedule";
    const cap = human.charAt(0).toUpperCase() + human.slice(1);
    return /\d{2}:\d{2}|midnight/.test(human) ? `${cap} ${timeZone}` : cap;
  }
  if (schedule.mode === "every") {
    return schedule.interval ? `Every ${schedule.interval}` : "Every interval";
  }
  return schedule.time ? `Once at ${schedule.time} ${timeZone}` : "Once at a set time";
}

export function buildScheduleReadout(
  schedule: AutomationSchedule,
  now: number,
  daysCleared: boolean,
  timeZone: string
): { valid: boolean; text: string } {
  if (schedule.mode === "cron") {
    if (daysCleared) return { valid: false, text: "Pick at least one day." };
    const expr = schedule.expr ?? "";
    const parts = expr.trim().split(/\s+/).filter(Boolean);
    if (parts.length !== 5 || !parseCron(expr)) {
      return { valid: false, text: `Needs 5 valid parts: ${CRON_PARTS_HINT}.` };
    }
    const phrase = describeSchedulePhrase(schedule, timeZone);
    const next = cronNext(expr, 1, now)?.[0];
    return {
      valid: true,
      text: [phrase, ...(next ? [`next ${formatRelative(next, now)}`] : []), expr.trim()].join(
        " · "
      ),
    };
  }
  if (schedule.mode === "every") {
    const interval = schedule.interval?.trim() ?? "";
    return parseDuration(interval)
      ? { valid: true, text: `Runs every ${interval}, starting right after you save.` }
      : { valid: false, text: "Use a duration like 30m, 1h or 2h30m." };
  }
  const date = localInputToDate(schedule.time ?? "");
  if (!date) return { valid: false, text: "Pick a date and time." };
  return date.getTime() <= now
    ? { valid: false, text: "That time is in the past. It would never run." }
    : { valid: true, text: `Runs once (${schedule.time} ${timeZone}), then stops.` };
}

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
  const dayTime = scheduleDayTime(expr);
  const selectedDays = new Set(daysCleared ? [] : (dayTime?.days ?? []));
  const builderApplies = dayTime !== null || daysCleared;
  const clockValue = dayTime ? formatClock(dayTime.hour, dayTime.minute) : "09:00";

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

            {readout ? <ScheduleReadout id="automation-schedule-readout" readout={readout} /> : null}

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
            {readout ? <ScheduleReadout id="automation-schedule-readout" readout={readout} /> : null}
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
