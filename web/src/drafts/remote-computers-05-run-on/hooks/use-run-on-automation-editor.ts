import { useMemo, useState } from "react";

import type { AutomationFormModel } from "@/systems/automation/hooks/use-automation-form";
import { retryDraftForStrategy } from "@/systems/automation/lib/automation-drafts";
import type { AutomationFormDraft } from "@/systems/automation/lib/automation-form-draft";
import type {
  AutomationCatchUpPolicy,
  AutomationFireLimit,
  AutomationRetry,
  AutomationSchedule,
  AutomationScheduleMode,
} from "@/systems/automation/types";
import type { RunOnTarget } from "@/systems/computers/components/run-on-picker";
import { COMPUTERS_PRO, LOCAL_COMPUTER } from "@/systems/computers/mocks/remote-computers-fixtures";
import { projectsForTarget } from "@/systems/computers/mocks/remote-computers-fixtures-sessions";

import {
  buildScheduleReadout,
  describeSchedulePhrase,
  type DraftAutomationDoes,
  type DraftAutomationStart,
} from "../run-on-automation-helpers";
import { automationTimeZoneForTarget, RUN_ON_AGENTS } from "../run-on-fixtures";

const FIXED_NOW = Date.parse("2026-11-22T15:00:00Z");

function defaultScheduleForMode(mode: AutomationScheduleMode): AutomationSchedule {
  if (mode === "cron") return { mode: "cron", expr: "0 9 * * 1-5" };
  if (mode === "every") return { mode: "every", interval: "30m" };
  return { mode: "at", time: "2026-11-23T09:00" };
}

function formatOptionsSummary(
  retry: AutomationRetry,
  fireLimit: AutomationFireLimit,
  enabled: boolean
): string {
  const retryPart = retry.strategy === "backoff" ? `retry ${retry.max_retries ?? 3}×` : "no retry";
  return `${retryPart} · ${fireLimit.max}/${fireLimit.window} · ${enabled ? "on" : "off"}`;
}

function isEditorReady(input: {
  isOffline: boolean;
  name: string;
  agentName: string;
  prompt: string;
  readoutValid: boolean;
}): boolean {
  return (
    !input.isOffline &&
    input.name.trim().length > 0 &&
    input.agentName.trim().length > 0 &&
    input.prompt.trim().length > 0 &&
    input.readoutValid
  );
}

function resolveComputerDisplayName(target: RunOnTarget, selectedName: string | undefined): string {
  if (target === "local") return LOCAL_COMPUTER.name;
  return selectedName ?? "devbox";
}

export function useRunOnAutomationEditor(initialTarget: RunOnTarget) {
  const [target, setTarget] = useState<RunOnTarget>(initialTarget);
  const [start, setStart] = useState<DraftAutomationStart>("schedule");
  const [does, setDoes] = useState<DraftAutomationDoes>("agent");
  const [name, setName] = useState("morning-pg-pool-smoke");
  const [agentName, setAgentName] = useState("general");
  const [prompt, setPrompt] = useState(
    "Run go test -race ./internal/pgpool/... and report any flake."
  );
  const [schedule, setSchedule] = useState<AutomationSchedule>(() =>
    defaultScheduleForMode("cron")
  );
  const [daysCleared, setDaysCleared] = useState(false);
  const [expressionOpen, setExpressionOpen] = useState(false);
  const [optionsOpen, setOptionsOpen] = useState(false);
  const [enabled, setEnabled] = useState(true);
  const [retry, setRetry] = useState<AutomationRetry>(() => retryDraftForStrategy("none"));
  const [fireLimit, setFireLimit] = useState<AutomationFireLimit>({ max: 12, window: "1h" });

  const projects = useMemo(() => projectsForTarget(target), [target]);
  const [workspaceId, setWorkspaceId] = useState(projects[0]?.id ?? "ws_devbox_api");
  const currentProject =
    projects.find(project => project.id === workspaceId) ?? projects[0] ?? null;

  const selectedRemote = COMPUTERS_PRO.find(computer => computer.id === target) ?? null;
  const computerDisplayName = resolveComputerDisplayName(target, selectedRemote?.name);
  const targetTimeZone = automationTimeZoneForTarget(target);
  const agentLabel = RUN_ON_AGENTS.find(agent => agent.name === agentName)?.label ?? agentName;

  const readout = useMemo(
    () =>
      start === "schedule"
        ? buildScheduleReadout(schedule, FIXED_NOW, daysCleared, targetTimeZone)
        : null,
    [start, schedule, daysCleared, targetTimeZone]
  );

  const ready = isEditorReady({
    isOffline: selectedRemote?.status === "offline",
    name,
    agentName,
    prompt,
    readoutValid: readout?.valid ?? true,
  });

  const schedulePhrase = describeSchedulePhrase(schedule, targetTimeZone);

  const handleScheduleMode = (mode: AutomationScheduleMode) => {
    setDaysCleared(false);
    setSchedule(defaultScheduleForMode(mode));
  };

  const handleTargetChange = (next: RunOnTarget) => {
    setTarget(next);
    const nextProjects = projectsForTarget(next);
    if (nextProjects[0]) setWorkspaceId(nextProjects[0].id);
  };

  const handleSchedulePatch = (patch: Partial<AutomationSchedule>) => {
    setSchedule(prev => ({ ...prev, ...patch }));
  };

  const optionsDraft = {
    start,
    schedule,
    enabled,
    retry,
    fire_limit: fireLimit,
  } as AutomationFormDraft;

  const optionsForm = {
    does,
    retry: retryDraftForStrategy(retry.strategy ?? "none", retry),
    optionsSummary: formatOptionsSummary(retry, fireLimit, enabled),
    onRetryChange: next => {
      if (does !== "task") setRetry(next);
    },
    onFireLimitChange: setFireLimit,
    onEnabledChange: setEnabled,
    onCatchUpPolicyChange: (catch_up_policy: AutomationCatchUpPolicy | undefined) =>
      setSchedule(prev => ({ ...prev, catch_up_policy })),
    onMisfireGraceChange: (misfire_grace_seconds: number | undefined) =>
      setSchedule(prev => ({ ...prev, misfire_grace_seconds })),
  } as AutomationFormModel;

  return {
    target,
    start,
    setStart,
    does,
    setDoes,
    name,
    setName,
    agentName,
    setAgentName,
    prompt,
    setPrompt,
    schedule,
    daysCleared,
    setDaysCleared,
    expressionOpen,
    setExpressionOpen,
    optionsOpen,
    setOptionsOpen,
    projects,
    setWorkspaceId,
    currentProject,
    computerDisplayName,
    targetTimeZone,
    agentLabel,
    readout,
    ready,
    schedulePhrase,
    optionsDraft,
    optionsForm,
    handleScheduleMode,
    handleTargetChange,
    handleSchedulePatch,
  };
}
