import type { AgentPayload } from "@/systems/agent";
import type { FormEvent } from "react";
import { useEffect, useState } from "react";

import {
  loopTargetAvailabilityMessage,
  useLoopTargetCatalog,
  type LoopTargetDraft,
} from "@/systems/loops";
import {
  isNetworkParticipationDraftValid,
  networkParticipationDraftFromPayload,
  serializeNetworkParticipation,
  type NetworkParticipationDraft,
} from "@/systems/network";

import {
  automationTargetMode,
  bindLoopTargetWorkspace,
  jobOutputMode,
  loopTargetWorkspaceId,
  retryDraftForStrategy,
  setJobOutputMode,
  setJobTargetMode,
  type JobOutputMode,
} from "../lib/automation-drafts";
import {
  compileCron,
  decodeCron,
  defaultAtLocal,
  fullCronModel,
  localInputToDate,
  parseCron,
  parseDuration,
  type CronFrequency,
  type CronModel,
} from "../lib/cron-engine";
import { buildJobPreview } from "../lib/job-preview";
import type { WorkspaceOption } from "../lib/trigger-preview";
import type {
  AutomationCatchUpPolicy,
  AutomationFireLimit,
  AutomationRetry,
  AutomationScheduleMode,
  AutomationScope,
  CreateAutomationJobRequest,
} from "../types";

type JobTask = NonNullable<CreateAutomationJobRequest["task"]>;
type JobOwner = NonNullable<JobTask["owner"]>;
type JobOwnerKind = JobOwner["kind"];

const DEFAULT_CRON_EXPR = "0 9 * * *";
const DEFAULT_EVERY_INTERVAL = "30m";
const SECOND_MS = 1_000;
const MINUTE_MS = 60_000;

/** Weekday presets for the visual cron builder (0=Sun … 6=Sat). */
const WEEKDAY_PRESETS = {
  weekdays: [1, 2, 3, 4, 5],
  weekend: [0, 6],
  all: [0, 1, 2, 3, 4, 5, 6],
} as const;

export type WeekdayPreset = keyof typeof WEEKDAY_PRESETS;

export interface UseAutomationJobFormParams {
  activeWorkspaceId?: string | null;
  draft: CreateAutomationJobRequest;
  isPending: boolean;
  mode: "create" | "edit";
  onChange: (draft: CreateAutomationJobRequest) => void;
  onSubmit: () => void;
  workspaces?: ReadonlyArray<WorkspaceOption>;
  agents?: AgentPayload[];
}

/** Read the current cron expression off the draft schedule (cron mode only). */
function scheduleExpr(draft: CreateAutomationJobRequest): string {
  return draft.schedule.expr ?? "";
}

/** True when the draft's active schedule is valid for its own mode. */
function scheduleValid(draft: CreateAutomationJobRequest, now: number): boolean {
  const schedule = draft.schedule;
  if (schedule.mode === "cron") {
    return parseCron(schedule.expr ?? "") !== null;
  }
  if (schedule.mode === "every") {
    return parseDuration(schedule.interval ?? "") !== null;
  }
  const date = localInputToDate(schedule.time ?? "");
  return date !== null && date.getTime() > now;
}

function delayToBoundary(now: number, boundary: number): number {
  const remainder = now % boundary;
  return remainder === 0 ? boundary : boundary - remainder;
}

function scheduleClockDelay(
  schedule: CreateAutomationJobRequest["schedule"],
  now: number
): number | null {
  if (schedule.mode === "cron") {
    return parseCron(schedule.expr ?? "") ? delayToBoundary(now, MINUTE_MS) : null;
  }
  if (schedule.mode === "every") {
    return parseDuration(schedule.interval ?? "") ? delayToBoundary(now, SECOND_MS) : null;
  }
  const date = localInputToDate(schedule.time ?? "");
  return date && date.getTime() > now ? delayToBoundary(now, SECOND_MS) : null;
}

/** A job runs an agent, a durable task, or a Loop (§9.14). */
export type JobTargetMode = "agent" | "task" | "loop";

function jobTargetMode(draft: CreateAutomationJobRequest): JobTargetMode {
  if (automationTargetMode(draft) === "loop") return "loop";
  return jobOutputMode(draft);
}

function computeCanSubmit(
  draft: CreateAutomationJobRequest,
  now: number,
  loopTargetCompatible: boolean
): boolean {
  const baseValid =
    draft.name.trim() !== "" && (draft.scope === "global" || Boolean(draft.workspace_id));
  if (!baseValid) return false;
  if (!scheduleValid(draft, now)) return false;
  const target = jobTargetMode(draft);
  if (target === "loop") {
    const loopWorkspaceId = draft.loop_target?.workspace_id ?? "";
    const loopWorkspaceValid =
      loopWorkspaceId !== "" &&
      (draft.scope === "global" || loopWorkspaceId === (draft.workspace_id ?? ""));
    const participationValid = isNetworkParticipationDraftValid(
      networkParticipationDraftFromPayload(draft.loop_target?.network_participation),
      ["named", "loop_run"]
    );
    return (
      Boolean(draft.loop_target?.loop_name.trim()) &&
      loopTargetCompatible &&
      loopWorkspaceValid &&
      participationValid
    );
  }
  if (target === "task") {
    // Task mode: owner is optional, title/description fall back to the job — nothing more required.
    return isNetworkParticipationDraftValid(
      networkParticipationDraftFromPayload(draft.task?.network_participation),
      ["named", "run"]
    );
  }
  return draft.agent_name.trim() !== "" && draft.prompt.trim() !== "";
}

/** View-model for the Job form: derives preview/schedule model and owns every patch handler. */
export function useAutomationJobForm({
  activeWorkspaceId,
  draft,
  isPending,
  mode,
  onChange,
  onSubmit,
  workspaces,
  agents,
}: UseAutomationJobFormParams) {
  const output: JobOutputMode = jobOutputMode(draft);
  const retry = retryDraftForStrategy(draft.retry?.strategy ?? "none", draft.retry ?? undefined);
  const [cronFrequencyOverride, setCronFrequencyOverride] = useState<CronFrequency | null>(null);
  const [now, setNow] = useState(Date.now);
  const schedule = draft.schedule;
  useEffect(() => {
    const clockDelay = scheduleClockDelay(schedule, Date.now());
    if (clockDelay === null) return;
    const timer = window.setTimeout(() => setNow(Date.now()), clockDelay);
    return () => window.clearTimeout(timer);
  }, [schedule, now]);

  const decodedCronModel = decodeCron(scheduleExpr(draft)) ?? { frequency: "custom" as const };
  const cronModel: CronModel = fullCronModel({
    ...decodedCronModel,
    frequency:
      draft.schedule.mode === "cron" && cronFrequencyOverride
        ? cronFrequencyOverride
        : decodedCronModel.frequency,
  });

  const resolvedWorkspaces: WorkspaceOption[] =
    workspaces && workspaces.length > 0
      ? [...workspaces]
      : activeWorkspaceId
        ? [{ id: activeWorkspaceId, name: activeWorkspaceId }]
        : [];

  const loopTarget: LoopTargetDraft = draft.loop_target ?? {
    loop_name: "",
    inputs: {},
    input_mapping: {},
  };
  const loopWorkspaceId = loopTargetWorkspaceId(draft, activeWorkspaceId);
  const loopCatalog = useLoopTargetCatalog(loopWorkspaceId, loopTarget.loop_name, "schedule");
  const loopTargetIssue = loopTargetAvailabilityMessage(loopCatalog, mode);
  const preview = buildJobPreview(draft, now, mode, loopTargetIssue);
  const canSubmit = computeCanSubmit(draft, now, loopCatalog.status === "compatible");

  const patch = (next: Partial<CreateAutomationJobRequest>) => onChange({ ...draft, ...next });

  /** Patch the schedule object, preserving fields that belong to other modes. */
  const patchSchedule = (next: Partial<CreateAutomationJobRequest["schedule"]>) => {
    setNow(Date.now());
    patch({ schedule: { ...draft.schedule, ...next } });
  };

  /** Patch the task object (only meaningful in task mode). */
  const patchTask = (next: Partial<JobTask>) => {
    const current = draft.task ?? {};
    patch({ task: { ...current, ...next } });
  };

  /** Compile the builder model back into the draft's cron expression. */
  const applyCronModel = (model: CronModel) => {
    const compiled = compileCron(model);
    if (model.frequency !== "custom") {
      setCronFrequencyOverride(null);
    }
    // `custom` keeps the raw user-entered expression; everything else recompiles.
    patchSchedule({ mode: "cron", expr: compiled ?? scheduleExpr(draft) });
  };

  const handleScopeChange = (scope: AutomationScope) => {
    if (scope === "global") {
      patch({ scope: "global", workspace_id: undefined });
      return;
    }
    const fallback = draft.workspace_id ?? activeWorkspaceId ?? resolvedWorkspaces[0]?.id;
    const workspaceId = fallback ?? "";
    onChange(
      bindLoopTargetWorkspace(
        { ...draft, scope: "workspace", workspace_id: workspaceId || undefined },
        workspaceId
      )
    );
  };

  const handleTargetChange = (next: JobTargetMode) => {
    if (next === "loop") {
      // Loop mode owns the target; a durable task cannot co-exist with a loop target.
      onChange(setJobTargetMode({ ...draft, task: undefined }, "loop", loopWorkspaceId));
      return;
    }
    onChange(setJobOutputMode(setJobTargetMode(draft, "agent"), next));
  };

  const handleLoopTargetChange = (next: LoopTargetDraft) => {
    patch({ loop_target: { ...next, workspace_id: loopWorkspaceId } });
  };

  const handleOwnerKind = (kind: JobOwnerKind | "") => {
    if (kind === "") {
      patchTask({ owner: null });
      return;
    }
    const ref = draft.task?.owner?.ref ?? "";
    patchTask({ owner: { kind, ref } });
  };

  const handleOwnerRef = (ref: string) => {
    const owner = draft.task?.owner;
    if (!owner) return;
    patchTask({ owner: { ...owner, ref } });
  };

  const handleTaskNetworkParticipation = (next: NetworkParticipationDraft) => {
    patchTask({ network_participation: serializeNetworkParticipation(next) });
  };

  const handleScheduleMode = (next: AutomationScheduleMode) => {
    setNow(Date.now());
    setCronFrequencyOverride(null);
    // Carry the recurring-reliability fields across every switch so a
    // cron → at → cron round-trip preserves what the operator entered; the
    // request normalizer drops them for one-shot `at`.
    const { catch_up_policy, misfire_grace_seconds } = draft.schedule;
    const recurringReliability = { catch_up_policy, misfire_grace_seconds };
    if (next === "cron") {
      const expr = scheduleExpr(draft) || DEFAULT_CRON_EXPR;
      patch({ schedule: { mode: "cron", expr, ...recurringReliability } });
      return;
    }
    if (next === "every") {
      const interval = draft.schedule.interval ?? DEFAULT_EVERY_INTERVAL;
      patch({ schedule: { mode: "every", interval, ...recurringReliability } });
      return;
    }
    const time = draft.schedule.time ?? defaultAtLocal();
    patch({ schedule: { mode: "at", time, ...recurringReliability } });
  };

  const handleCronFrequency = (frequency: CronFrequency) => {
    if (frequency === "custom") {
      setCronFrequencyOverride("custom");
      patchSchedule({ mode: "cron", expr: scheduleExpr(draft) || DEFAULT_CRON_EXPR });
      return;
    }
    setCronFrequencyOverride(null);
    applyCronModel({ ...cronModel, frequency });
  };

  const handleToggleWeekday = (weekday: number) => {
    const current = cronModel.weekdays;
    const has = current.includes(weekday);
    // Never let the weekly set drop to empty — keep at least one day selected.
    const next = has
      ? current.length > 1
        ? current.filter(day => day !== weekday)
        : current
      : [...current, weekday].sort((a, b) => a - b);
    applyCronModel({ ...cronModel, frequency: "weekly", weekdays: next });
  };

  const handleRetryChange = (next: AutomationRetry) => {
    // Task mode locks retry to none — the materialized task owns its own retries.
    if (output === "task") return;
    patch({ retry: next });
  };

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!canSubmit || isPending) return;
    onSubmit();
  };

  return {
    output,
    targetMode: jobTargetMode(draft),
    loopCatalog,
    loopTarget,
    retry,
    cronModel,
    resolvedWorkspaces,
    agents: agents ?? [],
    preview,
    canSubmit,
    scheduleMode: draft.schedule.mode,
    reliabilityDefaultOpen:
      mode === "edit" ||
      retry.strategy === "backoff" ||
      draft.enabled === false ||
      output === "task",

    // identity & scope
    onName: (name: string) => patch({ name }),
    onScopeChange: handleScopeChange,
    onWorkspaceChange: (workspace_id: string) =>
      onChange(bindLoopTargetWorkspace({ ...draft, workspace_id }, workspace_id)),

    // output / agent / task / loop
    onTargetChange: handleTargetChange,
    onLoopTargetChange: handleLoopTargetChange,
    onAgentChange: (agent_name: string) => patch({ agent_name }),
    onPromptChange: (prompt: string) => patch({ prompt }),
    onTaskTitle: (title: string) => patchTask({ title }),
    onTaskDescription: (description: string) => patchTask({ description }),
    onOwnerKind: handleOwnerKind,
    onOwnerRef: handleOwnerRef,
    onTaskNetworkParticipation: handleTaskNetworkParticipation,

    // schedule mode
    onScheduleMode: handleScheduleMode,

    // cron builder
    onCronFrequency: handleCronFrequency,
    onCronExpr: (expr: string) => {
      setCronFrequencyOverride("custom");
      patchSchedule({ mode: "cron", expr });
    },
    onEveryMinutes: (everyMinutes: number) =>
      applyCronModel({ ...cronModel, frequency: "minutes", everyMinutes }),
    onHourlyMinute: (hourlyMinute: number) =>
      applyCronModel({ ...cronModel, frequency: "hourly", hourlyMinute }),
    onDailyTime: (hour: number, minute: number) =>
      applyCronModel({ ...cronModel, frequency: "daily", hour, minute }),
    onWeeklyTime: (hour: number, minute: number) =>
      applyCronModel({ ...cronModel, frequency: "weekly", hour, minute }),
    onMonthlyTime: (hour: number, minute: number) =>
      applyCronModel({ ...cronModel, frequency: "monthly", hour, minute }),
    onToggleWeekday: handleToggleWeekday,
    onWeekdayPreset: (preset: WeekdayPreset) =>
      applyCronModel({
        ...cronModel,
        frequency: "weekly",
        weekdays: [...WEEKDAY_PRESETS[preset]],
      }),
    onMonthDay: (monthDay: number) =>
      applyCronModel({ ...cronModel, frequency: "monthly", monthDay }),
    onCronPreset: (expr: string) => {
      setCronFrequencyOverride(null);
      patchSchedule({ mode: "cron", expr });
    },

    // every / at
    onEveryInterval: (interval: string) => patchSchedule({ mode: "every", interval }),
    onEveryPreset: (interval: string) => patchSchedule({ mode: "every", interval }),
    onAtTime: (time: string) => patchSchedule({ mode: "at", time }),

    // schedule reliability (recurring-only: catch-up policy + misfire grace)
    recurring: draft.schedule.mode !== "at",
    catchUpPolicy: draft.schedule.catch_up_policy,
    misfireGraceSeconds: draft.schedule.misfire_grace_seconds,
    onCatchUpPolicyChange: (policy: AutomationCatchUpPolicy | undefined) =>
      patchSchedule({ catch_up_policy: policy }),
    onMisfireGraceChange: (seconds: number | undefined) =>
      patchSchedule({ misfire_grace_seconds: seconds }),

    // reliability
    onRetryChange: handleRetryChange,
    onFireLimitChange: (next: AutomationFireLimit) => patch({ fire_limit: next }),
    onEnabledChange: (enabled: boolean) => patch({ enabled }),

    handleSubmit,
  };
}
