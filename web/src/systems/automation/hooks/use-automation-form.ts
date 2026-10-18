import type { FormEvent } from "react";
import { useEffect, useState } from "react";

import { loopTargetWorkspaceId, retryDraftForStrategy } from "../lib/automation-drafts";
import { useAutomationTimeZone } from "./use-automation-time-zone";
import {
  automationFormDoes,
  automationFormEvent,
  automationFormSentenceDraft,
  setAutomationFormDoes,
  setAutomationFormEvent,
  setAutomationFormStart,
  type AutomationCondition,
  type AutomationFormDraft,
} from "../lib/automation-form-draft";
import {
  editorEventDef,
  editorEventIdFor,
  type EditorEventCardId,
} from "../lib/automation-form-events";
import {
  automationFormReady,
  automationOptionsSummary,
  incompleteConditionRows,
} from "../lib/automation-form-readiness";
import {
  automationFormDestination,
  canSwitchDoes,
  canSwitchStart,
  optionsOpenByDefault,
  resolveWorkspaceOptions,
  savedOneShotTime,
  scheduleForMode,
  taskOwnerFor,
} from "../lib/automation-form-edits";
import {
  scheduleReadout,
  setScheduleTime,
  toggleScheduleDay,
} from "../lib/automation-form-schedule";
import {
  automationSentenceIsIncomplete,
  describeAutomation,
  type AutomationDoes,
  type AutomationStart,
} from "../lib/automation-sentence";
import { localInputToDate, parseCron, parseDuration } from "../lib/cron-engine";
import { composeEventId, parseEventSelection } from "../lib/trigger-event-id";
import type { WorkspaceOption } from "../lib/trigger-preview";
import type {
  AutomationCatchUpPolicy,
  AutomationFireLimit,
  AutomationRetry,
  AutomationScheduleMode,
} from "../types";
import {
  type LoopAutomationStartKind,
  type LoopTargetDraft,
  useLoopTargetCatalog,
} from "@/systems/loops";

type JobTask = NonNullable<AutomationFormDraft["task"]>;
type JobOwnerKind = NonNullable<JobTask["owner"]>["kind"];

const SECOND_MS = 1_000;
const MINUTE_MS = 60_000;

const EMPTY_LOOP_TARGET: LoopTargetDraft = { loop_name: "", inputs: {}, input_mapping: {} };

const LOOP_START_KIND: Record<AutomationStart, LoopAutomationStartKind> = {
  schedule: "schedule",
  event: "trigger",
  webhook: "webhook",
};

export interface UseAutomationFormParams {
  activeWorkspaceId?: string | null;
  draft: AutomationFormDraft;
  isPending: boolean;
  /** Set when a Loop page seeded the editor: Does stays on that Loop. */
  lockedLoop?: string;
  mode: "create" | "edit";
  onChange: (draft: AutomationFormDraft) => void;
  onSubmit: () => void;
  workspaces?: ReadonlyArray<WorkspaceOption>;
}

function delayToBoundary(now: number, boundary: number): number {
  const remainder = now % boundary;
  return remainder === 0 ? boundary : boundary - remainder;
}

/** When the relative labels ("next in 14h") next change, or `null` when nothing ticks. */
function scheduleClockDelay(draft: AutomationFormDraft, now: number): number | null {
  if (draft.start !== "schedule") return null;
  const schedule = draft.schedule;
  if (schedule.mode === "cron") {
    return parseCron(schedule.expr ?? "") ? delayToBoundary(now, MINUTE_MS) : null;
  }
  if (schedule.mode === "every") {
    return parseDuration(schedule.interval ?? "") ? delayToBoundary(now, SECOND_MS) : null;
  }
  const date = localInputToDate(schedule.time ?? "");
  return date && date.getTime() > now ? delayToBoundary(now, SECOND_MS) : null;
}

/** The clock relative labels read, refreshed when "next in 14h" would next change. */
function useScheduleClock(draft: AutomationFormDraft) {
  const [now, setNow] = useState(Date.now);
  useEffect(() => {
    const delay = scheduleClockDelay(draft, Date.now());
    if (delay === null) return;
    const timer = window.setTimeout(() => setNow(Date.now()), delay);
    return () => window.clearTimeout(timer);
  }, [draft, now]);
  return { now, touch: () => setNow(Date.now()) };
}

/** The Loop target, its workspace and whether that Loop allows this start. */
function useFormLoopTarget(
  draft: AutomationFormDraft,
  activeWorkspaceId: string | null | undefined,
  does: AutomationDoes
) {
  const effectiveScope = draft.start === "webhook" ? "global" : draft.scope;
  const loopWorkspaceId = loopTargetWorkspaceId(
    { ...draft, scope: effectiveScope },
    activeWorkspaceId
  );
  const loopTarget: LoopTargetDraft = draft.loop_target ?? EMPTY_LOOP_TARGET;
  const loopCatalog = useLoopTargetCatalog(
    loopWorkspaceId,
    does === "loop" ? loopTarget.loop_name : "",
    LOOP_START_KIND[draft.start]
  );
  return { effectiveScope, loopCatalog, loopTarget, loopWorkspaceId };
}

/** View-model for the one automation form: derived sentence, readiness and every patch. */
export function useAutomationForm({
  activeWorkspaceId,
  draft,
  isPending,
  lockedLoop,
  mode,
  onChange,
  onSubmit,
  workspaces,
}: UseAutomationFormParams) {
  const [daysCleared, setDaysCleared] = useState(false);
  const [agentTouched, setAgentTouched] = useState(false);
  // An edit may keep a one-shot time that has already passed; only a changed time must be ahead.
  const [savedAtTime] = useState(() => savedOneShotTime(draft, mode));
  const clock = useScheduleClock(draft);
  const now = clock.now;

  const resolvedWorkspaces = resolveWorkspaceOptions(workspaces, activeWorkspaceId);
  const workspaceName = (id: string) => resolvedWorkspaces.find(item => item.id === id)?.name;

  const timeZone = useAutomationTimeZone();
  const does = automationFormDoes(draft);
  const isWebhook = draft.start === "webhook";
  const { effectiveScope, loopCatalog, loopTarget, loopWorkspaceId } = useFormLoopTarget(
    draft,
    activeWorkspaceId,
    does
  );

  const sentence = describeAutomation(automationFormSentenceDraft(draft, { daysCleared }), {
    timeZone,
    workspaceName,
  });
  const ready =
    !automationSentenceIsIncomplete(sentence) &&
    automationFormReady(draft, {
      daysCleared,
      loopCompatible: loopCatalog.status === "compatible",
      mode,
      now,
      savedAtTime,
    });
  const readout =
    draft.start === "schedule"
      ? scheduleReadout(draft.schedule, now, { daysCleared, timeZone })
      : null;
  const eventDef = editorEventDef(automationFormEvent(draft));
  const selection = parseEventSelection(draft.event);

  const patch = (next: Partial<AutomationFormDraft>) => onChange({ ...draft, ...next });
  const patchSchedule = (next: Partial<AutomationFormDraft["schedule"]>) => {
    clock.touch();
    patch({ schedule: { ...draft.schedule, ...next } });
  };
  const patchTask = (next: Partial<JobTask>) => patch({ task: { ...draft.task, ...next } });

  const handleStart = (start: AutomationStart) => {
    if (canSwitchStart(mode, draft.start, start)) onChange(setAutomationFormStart(draft, start));
  };

  const handleDoes = (next: AutomationDoes) => {
    if (canSwitchDoes(draft, { mode, lockedLoop, current: does }, next)) {
      onChange(setAutomationFormDoes(draft, next, loopWorkspaceId));
    }
  };

  const handleScheduleMode = (next: AutomationScheduleMode) => {
    clock.touch();
    patch({ schedule: scheduleForMode(draft.schedule, next) });
  };

  const handleCronExpr = (expr: string) => {
    setDaysCleared(false);
    patchSchedule({ mode: "cron", expr });
  };

  const handleToggleDay = (day: number) => {
    const next = toggleScheduleDay(draft.schedule.expr ?? "", day, daysCleared);
    setDaysCleared(next.cleared);
    patchSchedule({ mode: "cron", expr: next.expr });
  };

  const handleTime = (value: string) => {
    const expr = setScheduleTime(draft.schedule.expr ?? "", value);
    if (expr !== null) patchSchedule({ mode: "cron", expr });
  };

  const handleEventCard = (card: EditorEventCardId) => {
    onChange(setAutomationFormEvent(draft, editorEventIdFor(card, selection)));
  };

  const handleOwnerKind = (kind: JobOwnerKind | "") => {
    patchTask({ owner: taskOwnerFor(kind, draft.task?.owner?.ref ?? "") });
  };

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!ready || isPending) return;
    onSubmit();
  };

  const retry = retryDraftForStrategy(draft.retry?.strategy ?? "none", draft.retry ?? undefined);

  return {
    now,
    /** The global automation time zone every phrase in the editor reads in. */
    timeZone,
    does,
    isWebhook,
    effectiveScope,
    /** Project the automation lives in, or `null` for Global. */
    destination: automationFormDestination(effectiveScope, draft.workspace_id, workspaceName),
    sentence,
    ready,
    readout,
    daysCleared,
    eventDef,
    selection,
    loopCatalog,
    loopTarget,
    loopWorkspaceId,
    resolvedWorkspaces,
    retry,
    optionsSummary: automationOptionsSummary(draft),
    conditionProblems: incompleteConditionRows(draft.conditions),
    agentMissing: agentTouched && draft.agent_name.trim() === "",
    optionsDefaultOpen: optionsOpenByDefault(draft, mode, retry.strategy),

    onName: (name: string) => patch({ name }),
    onStart: handleStart,
    onDoes: handleDoes,

    onScheduleMode: handleScheduleMode,
    onQuickPick: handleCronExpr,
    onCronExpr: handleCronExpr,
    onToggleDay: handleToggleDay,
    onTime: handleTime,
    onEveryInterval: (interval: string) => patchSchedule({ mode: "every", interval }),
    onAtTime: (time: string) => patchSchedule({ mode: "at", time }),

    onEventCard: handleEventCard,
    onHookName: (hookName: string) =>
      patch({ event: composeEventId({ family: "hook", hookName }) }),
    onExtension: (extExt: string, extEvent: string) =>
      patch({ event: composeEventId({ family: "ext", extExt, extEvent }) }),
    onEndpointSlug: (endpoint_slug: string) => patch({ endpoint_slug }),
    onWebhookId: (webhook_id: string) => patch({ webhook_id }),
    onWebhookSecret: (webhook_secret_value: string) => patch({ webhook_secret_value }),
    onConditionsChange: (conditions: AutomationCondition[]) => patch({ conditions }),

    onAgentChange: (agent_name: string) => {
      setAgentTouched(true);
      patch({ agent_name });
    },
    onPromptChange: (prompt: string) => patch({ prompt }),
    onLoopTargetChange: (next: LoopTargetDraft) =>
      patch({ loop_target: { ...next, workspace_id: loopWorkspaceId } }),
    onTaskTitle: (title: string) => patchTask({ title }),
    onTaskDescription: (description: string) => patchTask({ description }),
    onOwnerKind: handleOwnerKind,
    onOwnerRef: (ref: string) => {
      const owner = draft.task?.owner;
      if (owner) patchTask({ owner: { ...owner, ref } });
    },

    onRetryChange: (next: AutomationRetry) => {
      // A task owns its retries; the automation's own retry stays off.
      if (does !== "task") patch({ retry: next });
    },
    onFireLimitChange: (next: AutomationFireLimit) => patch({ fire_limit: next }),
    onEnabledChange: (enabled: boolean) => patch({ enabled }),
    onCatchUpPolicyChange: (policy: AutomationCatchUpPolicy | undefined) =>
      patchSchedule({ catch_up_policy: policy }),
    onMisfireGraceChange: (seconds: number | undefined) =>
      patchSchedule({ misfire_grace_seconds: seconds }),

    handleSubmit,
  };
}

export type AutomationFormModel = ReturnType<typeof useAutomationForm>;
