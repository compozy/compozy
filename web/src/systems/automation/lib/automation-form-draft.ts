/**
 * One editor draft for every automation. The Starts choice decides the daemon
 * entity out of sight: a schedule saves a job, an event or a link saves a
 * trigger. The draft carries the shared fields once plus each entity's own
 * fields, and projects onto the exact job or trigger request at save time.
 */

import type {
  AutomationJob,
  AutomationTrigger,
  CreateAutomationJobRequest,
  CreateAutomationTriggerRequest,
  UpdateAutomationJobRequest,
  UpdateAutomationTriggerRequest,
} from "../types";
import {
  automationJobToDraft,
  automationJobUpdateFromDraft,
  automationTargetMode,
  automationTriggerToDraft,
  automationTriggerUpdateFromDraft,
  createAutomationJobDraft,
  emptyJobTask,
  emptyLoopTarget,
  LOOP_TARGET_KIND,
  retryDraftForStrategy,
} from "./automation-drafts";
import {
  buildAutomationJobRequest,
  buildAutomationTriggerRequest,
  projectAutomationJobRequest,
  projectAutomationTriggerRequest,
  type AutomationEditorMode,
  type AutomationRequestProjection,
} from "./automation-requests";
import type { AutomationDoes, AutomationDraft, AutomationStart } from "./automation-sentence";
import { getEventDef } from "./trigger-catalog";
import { parseEventSelection } from "./trigger-event-id";
import { retainValidFilters } from "./trigger-preview";

/** The editor section a deep link can open at. */
export type AutomationEditorSection = "options";

/** The event a new event automation listens for until the operator picks one. */
export const DEFAULT_EVENT = "session.stopped";
const WEBHOOK_EVENT = "webhook";

type TriggerOnlyFields = Pick<
  CreateAutomationTriggerRequest,
  "endpoint_slug" | "filter" | "webhook_id" | "webhook_secret_value"
>;

/**
 * Shared fields once (name, target, location, reliability), the job's
 * `schedule`/`task`, and the trigger's event, conditions and link identity.
 * `event` is the event start's runtime id and is never `webhook`; a link start
 * is its own Starts choice. `scope`/`workspace_id` keep the home location even
 * while a link start forces Global, so switching back restores it.
 */
export type AutomationFormDraft = CreateAutomationJobRequest &
  TriggerOnlyFields & {
    start: AutomationStart;
    event: string;
  };

export interface CreateAutomationFormDraftOptions {
  start?: AutomationStart;
  /** Preselects Does = Start a Loop with this Loop (Loop page "Automate"). */
  loop?: string;
}

/** A new automation in the active project, schedule-first unless a start is preselected. */
export function createAutomationFormDraft(
  activeWorkspaceId?: string | null,
  { start = "schedule", loop }: CreateAutomationFormDraftOptions = {}
): AutomationFormDraft {
  const job = createAutomationJobDraft(activeWorkspaceId);
  const draft: AutomationFormDraft = {
    ...job,
    start: "schedule",
    event: DEFAULT_EVENT,
    filter: {},
  };
  const seeded = loop
    ? {
        ...draft,
        scope: "workspace" as const,
        workspace_id: activeWorkspaceId ?? undefined,
        target_kind: LOOP_TARGET_KIND,
        loop_target: emptyLoopTarget(activeWorkspaceId, loop),
      }
    : draft;
  return setAutomationFormStart(seeded, start);
}

/** Edit draft for a persisted job. */
export function automationJobToFormDraft(job: AutomationJob): AutomationFormDraft {
  return { ...automationJobToDraft(job), start: "schedule", event: DEFAULT_EVENT, filter: {} };
}

/** Edit draft for a persisted trigger; a webhook trigger is a link start. */
export function automationTriggerToFormDraft(trigger: AutomationTrigger): AutomationFormDraft {
  const { event, filter, endpoint_slug, webhook_id, ...shared } = automationTriggerToDraft(trigger);
  const isWebhook = event.trim() === WEBHOOK_EVENT;
  return {
    ...shared,
    schedule: createAutomationJobDraft().schedule,
    start: isWebhook ? "webhook" : "event",
    event: isWebhook ? DEFAULT_EVENT : event,
    filter: filter ?? {},
    endpoint_slug,
    webhook_id,
  };
}

/** The daemon entity a draft saves as. */
export function automationFormEntity(draft: Pick<AutomationFormDraft, "start">): "job" | "trigger" {
  return draft.start === "schedule" ? "job" : "trigger";
}

/** What the automation does: a task body wins on schedules, then the Loop discriminator. */
export function automationFormDoes(
  draft: Pick<AutomationFormDraft, "start" | "target_kind" | "task">
): AutomationDoes {
  if (draft.start === "schedule" && draft.task) return "task";
  return automationTargetMode(draft) === "loop" ? "loop" : "agent";
}

/** The trigger event a draft would save: `webhook` for a link start. */
export function automationFormEvent(draft: Pick<AutomationFormDraft, "start" | "event">): string {
  return draft.start === "webhook" ? WEBHOOK_EVENT : draft.event;
}

/**
 * Switches Starts. Task targets exist only on schedules, so leaving a schedule
 * moves a task selection to Ask an agent; conditions that no longer fit the new
 * event are dropped, as the trigger form always did.
 */
export function setAutomationFormStart(
  draft: AutomationFormDraft,
  start: AutomationStart
): AutomationFormDraft {
  const next: AutomationFormDraft = { ...draft, start };
  if (start !== "schedule" && next.task) {
    next.task = undefined;
  }
  if (start !== "schedule") {
    const def = getEventDef(parseEventSelection(automationFormEvent(next)).catalogId);
    next.filter = retainValidFilters(next.filter ?? {}, def);
  }
  return next;
}

/** Switches Does. A task forces retries off: the task owns its own retries. */
export function setAutomationFormDoes(
  draft: AutomationFormDraft,
  does: AutomationDoes,
  loopWorkspaceId: string
): AutomationFormDraft {
  if (does === "loop") {
    return {
      ...draft,
      task: undefined,
      target_kind: LOOP_TARGET_KIND,
      loop_target: draft.loop_target ?? emptyLoopTarget(loopWorkspaceId),
    };
  }
  const agentDraft: AutomationFormDraft = {
    ...draft,
    target_kind: "agent",
    loop_target: undefined,
  };
  if (does === "task" && draft.start === "schedule") {
    return {
      ...agentDraft,
      task: draft.task ?? emptyJobTask(),
      retry: retryDraftForStrategy("none"),
    };
  }
  return { ...agentDraft, task: undefined };
}

/** The job create draft, only meaningful for schedule starts. */
export function automationFormJobDraft(draft: AutomationFormDraft): CreateAutomationJobRequest {
  return {
    name: draft.name,
    agent_name: draft.agent_name,
    prompt: draft.prompt,
    schedule: draft.schedule,
    scope: draft.scope,
    target_kind: draft.target_kind,
    loop_target: draft.loop_target,
    workspace_id: draft.workspace_id,
    task: draft.task,
    enabled: draft.enabled,
    retry: draft.retry,
    fire_limit: draft.fire_limit,
  };
}

/** The trigger create draft; a link start is always Global and carries its identity. */
export function automationFormTriggerDraft(
  draft: AutomationFormDraft
): CreateAutomationTriggerRequest {
  const isWebhook = draft.start === "webhook";
  return {
    name: draft.name,
    agent_name: draft.agent_name,
    prompt: draft.prompt,
    event: automationFormEvent(draft),
    filter: draft.filter ?? {},
    scope: isWebhook ? "global" : draft.scope,
    target_kind: draft.target_kind,
    loop_target: draft.loop_target,
    workspace_id: isWebhook ? undefined : draft.workspace_id,
    enabled: draft.enabled,
    retry: draft.retry,
    fire_limit: draft.fire_limit,
    ...(isWebhook
      ? {
          endpoint_slug: draft.endpoint_slug,
          webhook_id: draft.webhook_id,
          webhook_secret_value: draft.webhook_secret_value,
        }
      : {}),
  };
}

export type AutomationFormRequest =
  | { entity: "job"; create: CreateAutomationJobRequest; update: UpdateAutomationJobRequest }
  | {
      entity: "trigger";
      create: CreateAutomationTriggerRequest;
      update: UpdateAutomationTriggerRequest;
    };

/** The exact request the save sends, normalized by the shared request builders. */
export function buildAutomationFormRequest(draft: AutomationFormDraft): AutomationFormRequest {
  if (automationFormEntity(draft) === "job") {
    const create = buildAutomationJobRequest(automationFormJobDraft(draft));
    return { entity: "job", create, update: automationJobUpdateFromDraft(create) };
  }
  const create = buildAutomationTriggerRequest(automationFormTriggerDraft(draft));
  return { entity: "trigger", create, update: automationTriggerUpdateFromDraft(create) };
}

/** The displayed request (route + redacted-at-render payload) for the preview. */
export function projectAutomationFormRequest(
  draft: AutomationFormDraft,
  mode: AutomationEditorMode
): AutomationRequestProjection<unknown> {
  return automationFormEntity(draft) === "job"
    ? projectAutomationJobRequest(automationFormJobDraft(draft), mode)
    : projectAutomationTriggerRequest(automationFormTriggerDraft(draft), mode);
}

/** The draft shape `describeAutomation` reads; `days: []` marks a cleared day picker. */
export function automationFormSentenceDraft(
  draft: AutomationFormDraft,
  { daysCleared = false }: { daysCleared?: boolean } = {}
): AutomationDraft {
  const does = automationFormDoes(draft);
  return {
    start: draft.start,
    schedule:
      draft.start === "schedule"
        ? { ...draft.schedule, ...(daysCleared ? { days: [] } : {}) }
        : undefined,
    event: draft.start === "event" ? draft.event : undefined,
    filter: draft.filter ?? undefined,
    webhook: { slug: draft.endpoint_slug, webhookId: draft.webhook_id },
    target: {
      kind: does,
      agentName: draft.agent_name,
      prompt: draft.prompt,
      loopName: draft.loop_target?.loop_name,
      loopInputs: draft.loop_target?.inputs,
      // The daemon titles an untitled task after the automation.
      taskTitle: draft.task?.title?.trim() ? draft.task.title : draft.name,
      taskOwner: draft.task?.owner?.ref,
    },
    workspaceId: draft.scope === "workspace" ? draft.workspace_id : undefined,
  };
}
