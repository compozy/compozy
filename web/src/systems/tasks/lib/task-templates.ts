import type { PillTone } from "@agh/ui";

import type { CreateChildTaskRequest, CreateTaskRequest, TaskPriority } from "../types";

export type TaskTemplateId =
  | "one_shot"
  | "recurring"
  | "epic"
  | "remote_peer"
  | "human_in_loop"
  | "blank";

export interface TaskTemplate {
  id: TaskTemplateId;
  /** Advanced-mode display name. */
  label: string;
  /** Advanced-mode description. */
  description: string;
  /** Whether the template surfaces in Simple mode. */
  simple: boolean;
  /** Simple-mode display name. */
  simpleLabel: string;
  /** Simple-mode description (empty when the template is advanced-only). */
  simpleDescription: string;
  defaults: TaskTemplateDefaults;
  badges: TaskTemplateBadge[];
  preview: TaskTemplatePreview;
}

/**
 * Badge tones consume the 6-tone `PillTone` vocabulary The
 * empty-state template grid renders only `accent | info | warning | neutral`.
 */
export type TaskTemplateBadgeTone = Extract<PillTone, "accent" | "info" | "warning" | "neutral">;

export interface TaskTemplateBadge {
  label: string;
  tone: TaskTemplateBadgeTone;
}

export interface TaskTemplateDefaults {
  draft: boolean;
  priority?: TaskPriority;
  max_attempts?: number | null;
  approval_policy?: "none" | "manual";
}

export interface TaskTemplatePreview {
  enqueueOnSubmit: boolean;
  notice?: string;
}

const ONE_SHOT_TEMPLATE: TaskTemplate = {
  id: "one_shot",
  label: "One-shot",
  description: "A single task with one run. Good default for ad-hoc work.",
  simple: true,
  simpleLabel: "Run once",
  simpleDescription: "Do it now, once. Best for most things.",
  defaults: { draft: false, priority: "medium", max_attempts: 1 },
  badges: [{ label: "1 run", tone: "neutral" }],
  preview: { enqueueOnSubmit: true, notice: "Will enqueue 1 run immediately on submit." },
};

const RECURRING_TEMPLATE: TaskTemplate = {
  id: "recurring",
  label: "Recurring via automation",
  description:
    "Bind a cron or schedule from Automation — re-enqueues a run every tick. Configure the schedule from the Automation area after the draft is saved.",
  simple: false,
  simpleLabel: "Recurring",
  simpleDescription: "",
  defaults: { draft: true, priority: "medium" },
  badges: [{ label: "Automation", tone: "info" }],
  preview: {
    enqueueOnSubmit: false,
    notice: "Saves as a draft so Automation can attach the schedule later.",
  },
};

const EPIC_TEMPLATE: TaskTemplate = {
  id: "epic",
  label: "Epic with children",
  description:
    "A parent task that decomposes into child tasks. Reconciled via dependencies; child tasks can be added after creation.",
  simple: true,
  simpleLabel: "Break into steps",
  simpleDescription: "A big goal — add sub-tasks under it afterward.",
  defaults: { draft: false, priority: "high" },
  badges: [{ label: "Epic", tone: "warning" }],
  preview: { enqueueOnSubmit: true },
};

const REMOTE_PEER_TEMPLATE: TaskTemplate = {
  id: "remote_peer",
  label: "Remote from peer",
  description:
    "Ingress from a network channel. Remote peers enqueue; local owner claims when ready.",
  simple: false,
  simpleLabel: "Remote",
  simpleDescription: "",
  defaults: { draft: false, priority: "medium" },
  badges: [{ label: "Network", tone: "info" }],
  preview: { enqueueOnSubmit: true },
};

const HUMAN_IN_LOOP_TEMPLATE: TaskTemplate = {
  id: "human_in_loop",
  label: "Human-in-the-loop",
  description: "Agent proposes, human approves. Pauses on blocked until approved in Inbox.",
  simple: true,
  simpleLabel: "Needs approval",
  simpleDescription: "The agent waits for your sign-off before acting.",
  defaults: { draft: false, priority: "high", approval_policy: "manual" },
  badges: [{ label: "Approvals", tone: "warning" }],
  preview: {
    enqueueOnSubmit: true,
    notice: "First run will wait for approval in the Inbox before claiming.",
  },
};

const BLANK_TEMPLATE: TaskTemplate = {
  id: "blank",
  label: "Blank task",
  description: "Start with an empty form. Full control over owner, origin, and metadata.",
  simple: false,
  simpleLabel: "Blank",
  simpleDescription: "",
  defaults: { draft: false },
  badges: [{ label: "Custom", tone: "neutral" }],
  preview: { enqueueOnSubmit: true },
};

const TEMPLATE_BY_ID: Record<TaskTemplateId, TaskTemplate> = {
  one_shot: ONE_SHOT_TEMPLATE,
  recurring: RECURRING_TEMPLATE,
  epic: EPIC_TEMPLATE,
  remote_peer: REMOTE_PEER_TEMPLATE,
  human_in_loop: HUMAN_IN_LOOP_TEMPLATE,
  blank: BLANK_TEMPLATE,
};

export const TASK_TEMPLATES: TaskTemplate[] = [
  ONE_SHOT_TEMPLATE,
  RECURRING_TEMPLATE,
  EPIC_TEMPLATE,
  REMOTE_PEER_TEMPLATE,
  HUMAN_IN_LOOP_TEMPLATE,
  BLANK_TEMPLATE,
];

export const DEFAULT_TASK_TEMPLATE_ID: TaskTemplateId = "one_shot";

/** Templates surfaced in the Create-Task modal's Simple mode, in display order. */
export const SIMPLE_TASK_TEMPLATE_IDS: TaskTemplateId[] = ["one_shot", "human_in_loop", "epic"];

export function getTaskTemplate(id: TaskTemplateId): TaskTemplate {
  return TEMPLATE_BY_ID[id] ?? ONE_SHOT_TEMPLATE;
}

type TemplateAwareTaskCreationPayload = CreateTaskRequest | CreateChildTaskRequest;

export function applyTemplateToCreatePayload(
  base: CreateTaskRequest,
  templateId: TaskTemplateId
): CreateTaskRequest;
export function applyTemplateToCreatePayload(
  base: CreateChildTaskRequest,
  templateId: TaskTemplateId
): CreateChildTaskRequest;
export function applyTemplateToCreatePayload(
  base: TemplateAwareTaskCreationPayload,
  templateId: TaskTemplateId
): TemplateAwareTaskCreationPayload {
  const template = getTaskTemplate(templateId);
  return {
    ...base,
    draft: base.draft ?? template.defaults.draft,
    priority: base.priority ?? template.defaults.priority,
    max_attempts: base.max_attempts ?? template.defaults.max_attempts,
    approval_policy: base.approval_policy ?? template.defaults.approval_policy,
  };
}
