import type { FormEvent } from "react";
import { useEffect, useState } from "react";

import { loopTargetWorkspaceId, retryDraftForStrategy } from "../lib/automation-drafts";
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
import { defaultAtLocal, localInputToDate, parseCron, parseDuration } from "../lib/cron-engine";
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

const DEFAULT_CRON_EXPR = "0 9 * * *";
const DEFAULT_EVERY_INTERVAL = "30m";
const SECOND_MS = 1_000;
const MINUTE_MS = 60_000;

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
  const [savedAtTime] = useState(() =>
    mode === "edit" && draft.schedule.mode === "at" ? draft.schedule.time : undefined
  );
  const [now, setNow] = useState(Date.now);
  useEffect(() => {
    const delay = scheduleClockDelay(draft, Date.now());
    if (delay === null) return;
    const timer = window.setTimeout(() => setNow(Date.now()), delay);
    return () => window.clearTimeout(timer);
  }, [draft, now]);

  const resolvedWorkspaces: WorkspaceOption[] =
    workspaces && workspaces.length > 0
      ? [...workspaces]
      : activeWorkspaceId
        ? [{ id: activeWorkspaceId, name: activeWorkspaceId }]
        : [];
  const workspaceName = (id: string) => resolvedWorkspaces.find(item => item.id === id)?.name;

  const does = automationFormDoes(draft);
  const isWebhook = draft.start === "webhook";
  const effectiveScope = isWebhook ? "global" : draft.scope;
  const loopWorkspaceId = loopTargetWorkspaceId(
    { ...draft, scope: effectiveScope },
    activeWorkspaceId
  );
  const loopTarget: LoopTargetDraft = draft.loop_target ?? {
    loop_name: "",
    inputs: {},
    input_mapping: {},
  };
  const loopCatalog = useLoopTargetCatalog(
    loopWorkspaceId,
    does === "loop" ? loopTarget.loop_name : "",
    LOOP_START_KIND[draft.start]
  );

  const sentence = describeAutomation(automationFormSentenceDraft(draft, { daysCleared }), {
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
    draft.start === "schedule" ? scheduleReadout(draft.schedule, now, { daysCleared }) : null;
  const eventDef = editorEventDef(automationFormEvent(draft));
  const selection = parseEventSelection(draft.event);

  const patch = (next: Partial<AutomationFormDraft>) => onChange({ ...draft, ...next });
  const patchSchedule = (next: Partial<AutomationFormDraft["schedule"]>) => {
    setNow(Date.now());
    patch({ schedule: { ...draft.schedule, ...next } });
  };
  const patchTask = (next: Partial<JobTask>) => patch({ task: { ...draft.task, ...next } });

  const handleStart = (start: AutomationStart) => {
    if (mode === "edit" || start === draft.start) return;
    onChange(setAutomationFormStart(draft, start));
  };

  const handleDoes = (next: AutomationDoes) => {
    if (mode === "edit" || lockedLoop || next === does) return;
    if (next === "task" && draft.start !== "schedule") return;
    onChange(setAutomationFormDoes(draft, next, loopWorkspaceId));
  };

  const handleScheduleMode = (next: AutomationScheduleMode) => {
    setNow(Date.now());
    // Carry the recurring reliability fields across every switch so a round
    // trip keeps them; the request normalizer drops them for one-shot `at`.
    const { catch_up_policy, misfire_grace_seconds } = draft.schedule;
    const recurring = { catch_up_policy, misfire_grace_seconds };
    if (next === "cron") {
      patch({
        schedule: { mode: "cron", expr: draft.schedule.expr || DEFAULT_CRON_EXPR, ...recurring },
      });
    } else if (next === "every") {
      const interval = draft.schedule.interval ?? DEFAULT_EVERY_INTERVAL;
      patch({ schedule: { mode: "every", interval, ...recurring } });
    } else {
      patch({
        schedule: { mode: "at", time: draft.schedule.time ?? defaultAtLocal(), ...recurring },
      });
    }
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
    patchTask({ owner: kind === "" ? null : { kind, ref: draft.task?.owner?.ref ?? "" } });
  };

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!ready || isPending) return;
    onSubmit();
  };

  const retry = retryDraftForStrategy(draft.retry?.strategy ?? "none", draft.retry ?? undefined);

  return {
    now,
    does,
    isWebhook,
    effectiveScope,
    /** Project the automation lives in, or `null` for Global. */
    destination:
      effectiveScope === "global"
        ? null
        : (workspaceName(draft.workspace_id ?? "") ?? draft.workspace_id ?? "project"),
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
    optionsDefaultOpen: mode === "edit" || retry.strategy === "backoff" || draft.enabled === false,

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
