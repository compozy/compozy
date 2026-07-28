import type { UpdateTaskRequest } from "../types";
import {
  networkParticipationDraftFromPayload,
  networkParticipationDraftFromValues,
  serializeNetworkParticipation,
  type NetworkParticipationStrategy,
} from "@/systems/network";
import type { TaskTemplateId } from "./task-templates";
import { applyTemplateToCreatePayload, getTaskTemplate } from "./task-templates";
import type {
  CreateChildTaskRequest,
  CreateTaskRequest,
  TaskExecutionProfile,
  TaskOwnerKind,
  TaskPriority,
  TaskRecord,
  TaskScope,
} from "../types";

export interface TaskEditorDraft {
  title: string;
  description: string;
  scope: TaskScope;
  workspaceId: string | null;
  priority: TaskPriority;
  ownerKind: TaskOwnerKind | "";
  ownerRef: string;
  parentTaskId: string;
  maxAttempts: number | null;
  approvalPolicy: "none" | "manual";
  identifier: string;
  autoEnqueueOnReady: boolean;
  saveAsDraft: boolean;
  networkParticipationMode: "local" | "live";
  networkChannelId: string;
  networkChannelStrategy: NetworkParticipationStrategy | "";
}

export const EMPTY_TASK_EDITOR_DRAFT: TaskEditorDraft = {
  title: "",
  description: "",
  scope: "workspace",
  workspaceId: null,
  priority: "medium",
  ownerKind: "",
  ownerRef: "",
  parentTaskId: "",
  maxAttempts: 1,
  approvalPolicy: "none",
  identifier: "",
  autoEnqueueOnReady: false,
  saveAsDraft: false,
  networkParticipationMode: "local",
  networkChannelId: "",
  networkChannelStrategy: "",
};

type TaskTemplateDraftDefaults = Pick<
  TaskEditorDraft,
  "priority" | "maxAttempts" | "approvalPolicy" | "saveAsDraft"
>;

function taskTemplateDraftDefaults(templateId: TaskTemplateId): TaskTemplateDraftDefaults {
  const template = getTaskTemplate(templateId);

  return {
    priority: template.defaults.priority ?? "medium",
    maxAttempts:
      typeof template.defaults.max_attempts === "number" ? template.defaults.max_attempts : 1,
    approvalPolicy: template.defaults.approval_policy ?? "none",
    saveAsDraft: template.defaults.draft,
  };
}

export function createTaskEditorDraft(
  templateId: TaskTemplateId,
  activeWorkspaceId?: string | null
): TaskEditorDraft {
  return {
    ...EMPTY_TASK_EDITOR_DRAFT,
    scope: activeWorkspaceId ? "workspace" : "global",
    workspaceId: activeWorkspaceId ?? null,
    ...taskTemplateDraftDefaults(templateId),
  };
}

export function applyTaskTemplateToEditorDraft(
  draft: TaskEditorDraft,
  templateId: TaskTemplateId
): TaskEditorDraft {
  return {
    ...draft,
    ...taskTemplateDraftDefaults(templateId),
  };
}

export function taskEditorDraftFromTask(
  task: TaskRecord,
  profile: TaskExecutionProfile
): TaskEditorDraft {
  const participationDraft = networkParticipationDraftFromPayload(profile.network_participation);
  return {
    title: task.title,
    description: task.description ?? "",
    scope: task.scope,
    workspaceId: task.workspace_id ?? null,
    priority: task.priority ?? "medium",
    ownerKind: task.owner?.kind ?? "",
    ownerRef: task.owner?.ref ?? "",
    parentTaskId: task.parent_task_id ?? "",
    maxAttempts: task.max_attempts ?? null,
    approvalPolicy: task.approval_policy === "manual" ? "manual" : "none",
    identifier: task.identifier ?? "",
    autoEnqueueOnReady: task.auto_enqueue_on_ready ?? false,
    saveAsDraft: task.draft ?? false,
    networkParticipationMode: participationDraft.mode,
    networkChannelId: participationDraft.channelId,
    networkChannelStrategy: participationDraft.channelStrategy,
  };
}

function resolveOwnerInput(draft: TaskEditorDraft) {
  const ownerKind = draft.ownerKind || undefined;
  const ownerRef = draft.ownerRef.trim();

  if (!ownerKind) {
    return { owner: undefined, ownerIsEmpty: true };
  }

  return {
    owner: ownerRef ? { kind: ownerKind, ref: ownerRef } : null,
    ownerIsEmpty: false,
  };
}

export function buildCreateTaskRequest(
  draft: TaskEditorDraft,
  options: {
    templateId: TaskTemplateId;
    asDraft: boolean;
  }
): CreateTaskRequest {
  const { owner } = resolveOwnerInput(draft);

  const basePayload: CreateTaskRequest = {
    title: draft.title.trim(),
    description: draft.description.trim() || undefined,
    scope: draft.scope,
    workspace: draft.scope === "workspace" ? (draft.workspaceId ?? undefined) : undefined,
    priority: draft.priority,
    max_attempts: draft.maxAttempts ?? undefined,
    draft: options.asDraft || draft.saveAsDraft || options.templateId === "recurring",
    auto_enqueue_on_ready: draft.autoEnqueueOnReady || undefined,
    owner,
    approval_policy: draft.approvalPolicy === "manual" ? "manual" : undefined,
    identifier: draft.identifier.trim() || undefined,
    network_participation: serializeNetworkParticipation(
      networkParticipationDraftFromValues(
        draft.networkParticipationMode,
        draft.networkChannelId,
        draft.networkChannelStrategy
      )
    ),
  };

  return applyTemplateToCreatePayload(basePayload, options.templateId);
}

export function buildCreateChildTaskRequest(
  draft: TaskEditorDraft,
  options: {
    templateId: TaskTemplateId;
    asDraft: boolean;
  }
): CreateChildTaskRequest {
  const { owner } = resolveOwnerInput(draft);

  const basePayload: CreateChildTaskRequest = {
    title: draft.title.trim(),
    description: draft.description.trim() || undefined,
    scope: draft.scope,
    workspace: draft.scope === "workspace" ? (draft.workspaceId ?? undefined) : undefined,
    priority: draft.priority,
    max_attempts: draft.maxAttempts ?? undefined,
    draft: options.asDraft || draft.saveAsDraft || options.templateId === "recurring",
    auto_enqueue_on_ready: draft.autoEnqueueOnReady || undefined,
    owner,
    approval_policy: draft.approvalPolicy === "manual" ? "manual" : undefined,
    identifier: draft.identifier.trim() || undefined,
    network_participation: serializeNetworkParticipation(
      networkParticipationDraftFromValues(
        draft.networkParticipationMode,
        draft.networkChannelId,
        draft.networkChannelStrategy
      )
    ),
  };

  return applyTemplateToCreatePayload(basePayload, options.templateId);
}

export function buildUpdateTaskRequest(draft: TaskEditorDraft): UpdateTaskRequest {
  const { owner, ownerIsEmpty } = resolveOwnerInput(draft);

  return {
    title: draft.title.trim() || undefined,
    description: draft.description.trim() || null,
    priority: draft.priority,
    ...(ownerIsEmpty ? { clear_owner: true } : { owner }),
    max_attempts: draft.maxAttempts ?? null,
    approval_policy: draft.approvalPolicy === "manual" ? "manual" : "none",
    auto_enqueue_on_ready: draft.autoEnqueueOnReady,
    network_participation: serializeNetworkParticipation(
      networkParticipationDraftFromValues(
        draft.networkParticipationMode,
        draft.networkChannelId,
        draft.networkChannelStrategy
      )
    ),
  };
}
