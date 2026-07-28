import type {
  AutomationJob,
  AutomationRetry,
  AutomationTrigger,
  CreateAutomationJobRequest,
  CreateAutomationTriggerRequest,
  UpdateAutomationJobRequest,
  UpdateAutomationTriggerRequest,
} from "../types";

const DEFAULT_RETRY_NONE = {
  strategy: "none" as const,
  max_retries: 0,
  base_delay: "",
};

const DEFAULT_RETRY_BACKOFF = {
  strategy: "backoff" as const,
  max_retries: 3,
  base_delay: "2s",
};

const DEFAULT_FIRE_LIMIT = {
  max: 12,
  window: "1h",
};

export function retryDraftForStrategy(
  strategy: AutomationRetry["strategy"],
  retry?: AutomationRetry
): AutomationRetry {
  if (strategy === "backoff") {
    return {
      strategy: "backoff",
      max_retries:
        retry?.strategy === "backoff" && retry.max_retries > 0
          ? retry.max_retries
          : DEFAULT_RETRY_BACKOFF.max_retries,
      base_delay:
        retry?.strategy === "backoff" && retry.base_delay.trim() !== ""
          ? retry.base_delay
          : DEFAULT_RETRY_BACKOFF.base_delay,
    };
  }

  return { ...DEFAULT_RETRY_NONE };
}

export function normalizeAutomationRetry(retry?: AutomationRetry): AutomationRetry {
  return retryDraftForStrategy(retry?.strategy ?? "none", retry);
}

export function createAutomationJobDraft(
  activeWorkspaceId?: string | null
): CreateAutomationJobRequest {
  const workspaceId = activeWorkspaceId ?? undefined;

  return {
    name: "",
    agent_name: "",
    prompt: "",
    schedule: {
      mode: "cron",
      expr: "0 9 * * *",
    },
    scope: workspaceId ? "workspace" : "global",
    target_kind: "agent",
    workspace_id: workspaceId,
    enabled: true,
    retry: normalizeAutomationRetry(),
    fire_limit: { ...DEFAULT_FIRE_LIMIT },
  };
}

export type JobOutputMode = "agent" | "task";

/** Derive the output mode from the draft: a task object means task mode. */
export function jobOutputMode(draft: CreateAutomationJobRequest): JobOutputMode {
  return draft.task ? "task" : "agent";
}

/** A blank task object for entering task mode. */
export function emptyJobTask(): NonNullable<CreateAutomationJobRequest["task"]> {
  return {
    title: "",
    description: "",
    owner: null,
    network_participation: { mode: "local" },
  };
}

/**
 * Switch the draft between agent and task output modes. The runtime requires
 * `retry.strategy === "none"` whenever a task is materialized, so task mode also
 * forces the retry policy to none.
 */
export function setJobOutputMode(
  draft: CreateAutomationJobRequest,
  mode: JobOutputMode
): CreateAutomationJobRequest {
  if (mode === "task") {
    return {
      ...draft,
      task: draft.task ?? emptyJobTask(),
      retry: retryDraftForStrategy("none"),
    };
  }
  return { ...draft, task: undefined };
}

export function automationJobToDraft(job: AutomationJob): CreateAutomationJobRequest {
  return {
    name: job.name,
    agent_name: job.agent_name,
    prompt: job.prompt,
    schedule: job.schedule ?? {
      mode: "cron",
      expr: "0 9 * * *",
    },
    scope: job.scope,
    target_kind: job.target_kind,
    loop_target: job.loop_target ?? undefined,
    workspace_id: job.workspace_id,
    task: job.task,
    enabled: job.enabled,
    retry: normalizeAutomationRetry(job.retry),
    fire_limit: {
      max: job.fire_limit.max,
      window: job.fire_limit.window,
    },
  };
}

/** Project an editable create-shaped draft onto the mutable job update contract. */
export function automationJobUpdateFromDraft(
  draft: CreateAutomationJobRequest
): UpdateAutomationJobRequest {
  const {
    agent_name: _agentName,
    scope: _scope,
    target_kind: _targetKind,
    workspace_id: _workspaceId,
    ...request
  } = draft;
  return request;
}

export function createAutomationTriggerDraft(
  activeWorkspaceId?: string | null
): CreateAutomationTriggerRequest {
  const workspaceId = activeWorkspaceId ?? undefined;

  return {
    name: "",
    agent_name: "",
    prompt: "",
    event: "session.stopped",
    filter: {},
    scope: workspaceId ? "workspace" : "global",
    target_kind: "agent",
    workspace_id: workspaceId,
    enabled: true,
    retry: normalizeAutomationRetry(),
    fire_limit: { ...DEFAULT_FIRE_LIMIT },
  };
}

/** The `target_kind` discriminator value marking a loop-target automation. */
export const LOOP_TARGET_KIND = "loop";
const AGENT_TARGET_KIND = "agent";

export type AutomationTargetMode = "agent" | "loop";
export type AutomationLoopTarget = NonNullable<CreateAutomationTriggerRequest["loop_target"]>;

/** The persisted discriminator is the sole authority for the target union branch. */
export function automationTargetMode(
  draft: Pick<CreateAutomationTriggerRequest, "target_kind">
): AutomationTargetMode {
  return draft.target_kind === LOOP_TARGET_KIND ? "loop" : "agent";
}

/** A blank loop target bound to the automation's own workspace. */
export function emptyLoopTarget(workspaceId?: string | null, loopName = ""): AutomationLoopTarget {
  return {
    loop_name: loopName,
    workspace_id: workspaceId ?? "",
    inputs: {},
    input_mapping: {},
    network_participation: { mode: "local" },
  };
}

/** Resolves the concrete workspace that owns an Automation Loop target. */
export function loopTargetWorkspaceId(
  draft: Pick<CreateAutomationTriggerRequest, "loop_target" | "scope" | "workspace_id">,
  fallbackWorkspaceId?: string | null
): string {
  if (draft.scope === "workspace") {
    return draft.workspace_id ?? fallbackWorkspaceId ?? "";
  }

  return draft.loop_target?.workspace_id || fallbackWorkspaceId || "";
}

/** Keeps a Trigger's nested Loop target binding synchronized without creating a target branch. */
export function bindLoopTargetWorkspace(
  draft: CreateAutomationTriggerRequest,
  workspaceId: string
): CreateAutomationTriggerRequest;
/** Keeps a Job's nested Loop target binding synchronized without creating a target branch. */
export function bindLoopTargetWorkspace(
  draft: CreateAutomationJobRequest,
  workspaceId: string
): CreateAutomationJobRequest;
export function bindLoopTargetWorkspace(
  draft: CreateAutomationTriggerRequest | CreateAutomationJobRequest,
  workspaceId: string
): CreateAutomationTriggerRequest | CreateAutomationJobRequest {
  if (automationTargetMode(draft) !== "loop" || !draft.loop_target) {
    return draft;
  }

  return {
    ...draft,
    loop_target: { ...draft.loop_target, workspace_id: workspaceId },
  };
}

function applyTargetMode(
  draft: CreateAutomationTriggerRequest,
  mode: AutomationTargetMode,
  targetWorkspaceId?: string
): CreateAutomationTriggerRequest;
function applyTargetMode(
  draft: CreateAutomationJobRequest,
  mode: AutomationTargetMode,
  targetWorkspaceId?: string
): CreateAutomationJobRequest;
function applyTargetMode(
  draft: CreateAutomationTriggerRequest | CreateAutomationJobRequest,
  mode: AutomationTargetMode,
  targetWorkspaceId: string = draft.workspace_id ?? ""
): CreateAutomationTriggerRequest | CreateAutomationJobRequest {
  if (mode === "loop") {
    return {
      ...draft,
      target_kind: LOOP_TARGET_KIND,
      loop_target: draft.loop_target ?? emptyLoopTarget(targetWorkspaceId),
    };
  }
  return { ...draft, target_kind: AGENT_TARGET_KIND, loop_target: undefined };
}

/**
 * Switches a trigger draft between running an agent and running a Loop. Loop mode
 * pins the `target_kind` discriminator and seeds an empty loop target scoped to
 * the automation's workspace; agent mode clears the loop target.
 */
export function setTriggerTargetMode(
  draft: CreateAutomationTriggerRequest,
  mode: AutomationTargetMode,
  targetWorkspaceId: string = draft.workspace_id ?? ""
): CreateAutomationTriggerRequest {
  return applyTargetMode(draft, mode, targetWorkspaceId);
}

/** Switches a job draft between agent and Loop targets (see setTriggerTargetMode). */
export function setJobTargetMode(
  draft: CreateAutomationJobRequest,
  mode: AutomationTargetMode,
  targetWorkspaceId: string = draft.workspace_id ?? ""
): CreateAutomationJobRequest {
  return applyTargetMode(draft, mode, targetWorkspaceId);
}

/** A trigger create draft pre-targeted at one Loop (the detail "Add trigger" CTA). */
export function createLoopTargetTriggerDraft(
  workspaceId: string | null | undefined,
  loopName: string
): CreateAutomationTriggerRequest {
  const draft = createAutomationTriggerDraft(workspaceId);
  return {
    ...draft,
    scope: "workspace",
    workspace_id: workspaceId ?? undefined,
    target_kind: LOOP_TARGET_KIND,
    loop_target: emptyLoopTarget(workspaceId, loopName),
  };
}

/** A job create draft pre-targeted at one Loop (the detail "Add schedule" CTA). */
export function createLoopTargetJobDraft(
  workspaceId: string | null | undefined,
  loopName: string
): CreateAutomationJobRequest {
  const draft = createAutomationJobDraft(workspaceId);
  return {
    ...draft,
    scope: "workspace",
    workspace_id: workspaceId ?? undefined,
    target_kind: LOOP_TARGET_KIND,
    loop_target: emptyLoopTarget(workspaceId, loopName),
  };
}

export function automationTriggerToDraft(
  trigger: AutomationTrigger
): CreateAutomationTriggerRequest {
  return {
    name: trigger.name,
    agent_name: trigger.agent_name,
    prompt: trigger.prompt,
    event: trigger.event,
    filter: trigger.filter ?? {},
    scope: trigger.scope,
    target_kind: trigger.target_kind,
    loop_target: trigger.loop_target ?? undefined,
    workspace_id: trigger.workspace_id,
    enabled: trigger.enabled,
    retry: normalizeAutomationRetry(trigger.retry),
    fire_limit: {
      max: trigger.fire_limit.max,
      window: trigger.fire_limit.window,
    },
    endpoint_slug: trigger.endpoint_slug,
    webhook_id: trigger.webhook_id,
  };
}

/** Project an editable create-shaped draft onto the mutable trigger update contract. */
export function automationTriggerUpdateFromDraft(
  draft: CreateAutomationTriggerRequest
): UpdateAutomationTriggerRequest {
  const {
    agent_name: _agentName,
    scope: _scope,
    target_kind: _targetKind,
    workspace_id: _workspaceId,
    ...request
  } = draft;
  return request;
}
