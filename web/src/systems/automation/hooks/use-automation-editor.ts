import { useEffect, useEffectEvent, useLayoutEffect, useState } from "react";
import { useSelector } from "@xstate/store-react";
import { useStoreBinding } from "@/hooks/use-store-binding";

import { useAgents } from "@/systems/agent";

import { AutomationApiError } from "../adapters/automation-api";
import {
  automationJobUpdateFromDraft,
  automationJobToDraft,
  automationTriggerUpdateFromDraft,
  automationTriggerToDraft,
  createAutomationJobDraft,
  createAutomationTriggerDraft,
  createLoopTargetJobDraft,
  createLoopTargetTriggerDraft,
} from "../lib/automation-drafts";
import {
  buildAutomationJobRequest,
  buildAutomationTriggerRequest,
} from "../lib/automation-requests";
import { createAutomationDialogHandle } from "../lib/dialog-handle";
import type { WorkspaceOption } from "../lib/trigger-preview";
import type {
  AutomationJob,
  AutomationTrigger,
  CreateAutomationJobRequest,
  CreateAutomationTriggerRequest,
} from "../types";
import { useCreateAutomationJob, useUpdateAutomationJob } from "./use-automation-actions";
import { useCreateAutomationTrigger, useUpdateAutomationTrigger } from "./use-automation-actions";
import {
  createWorkspaceEditorLogic,
  type WorkspaceEditorFailure,
  type WorkspaceEditorValue,
} from "./automation-editor-store";

type JobEditorState =
  | { draft: CreateAutomationJobRequest; mode: "create" }
  | { draft: CreateAutomationJobRequest; id: string; mode: "edit" };

type TriggerEditorState =
  | { draft: CreateAutomationTriggerRequest; mode: "create" }
  | { draft: CreateAutomationTriggerRequest; id: string; mode: "edit" };

interface JobEditorParams {
  activeWorkspaceId?: string | null;
  onSaved?: (job: AutomationJob) => void;
  workspaces?: ReadonlyArray<WorkspaceOption>;
}

const jobEditorLogic = createWorkspaceEditorLogic<JobEditorState, AutomationJob>();
const triggerEditorLogic = createWorkspaceEditorLogic<TriggerEditorState, AutomationTrigger>();

function useWorkspaceBoundEditor<T extends WorkspaceEditorValue, TResult>(
  logic: ReturnType<typeof createWorkspaceEditorLogic<T, TResult>>,
  workspaceId: string | null | undefined
) {
  const { store } = useStoreBinding(workspaceId, () => logic.createStore({ workspaceId }));
  const editor = useSelector(store, snapshot => snapshot.context.editor);
  const pendingRequest = useSelector(store, snapshot => snapshot.context.pendingRequest);
  const submitError = useSelector(store, snapshot => snapshot.context.submitError);

  useLayoutEffect(
    () => () => {
      store.trigger.lifecycleDisposed();
    },
    [store]
  );

  return { editor, pendingRequest, store, submitError };
}

function agentCatalogErrorMessage(error: unknown): string | null {
  if (!(error instanceof Error)) return error ? "Unable to load agents." : null;
  return error.message.trim() || "Unable to load agents.";
}

/**
 * Modal create/edit controller for automation jobs. Shared by the list route
 * (create + Loop deep-link) and the detail route (edit in place).
 */
export function useAutomationJobEditor({
  activeWorkspaceId,
  onSaved,
  workspaces,
}: JobEditorParams) {
  const { editor, pendingRequest, store } = useWorkspaceBoundEditor(
    jobEditorLogic,
    activeWorkspaceId
  );
  const [handle] = useState(createAutomationDialogHandle);
  // The target selector reads the workspace catalog; a global editor still lists
  // the active workspace's agents because that is what the job can address.
  const agentsQuery = useAgents(activeWorkspaceId);
  const createMutation = useCreateAutomationJob();
  const updateMutation = useUpdateAutomationJob();
  const handleSaveSucceeded = useEffectEvent((result: AutomationJob) => onSaved?.(result));

  useEffect(() => {
    const succeeded = store.on("saveSucceeded", event => handleSaveSucceeded(event.result));
    return () => {
      succeeded.unsubscribe();
    };
  }, [store]);

  const openCreate = () =>
    store.trigger.editorOpened({
      editor: { draft: createAutomationJobDraft(activeWorkspaceId), mode: "create" },
      workspaceId: activeWorkspaceId,
    });
  const openLoopCreate = (loop: string) =>
    store.trigger.editorOpened({
      editor: { draft: createLoopTargetJobDraft(activeWorkspaceId, loop), mode: "create" },
      workspaceId: activeWorkspaceId,
    });
  const openEdit = (job: AutomationJob) =>
    store.trigger.editorOpened({
      editor: { draft: automationJobToDraft(job), id: job.id, mode: "edit" },
      workspaceId: activeWorkspaceId,
    });
  const close = () => store.trigger.editorClosed();

  const handleSubmit = () => {
    const currentEditor = editor;
    if (!currentEditor) return;
    store.trigger.submissionRequested({
      describeFailure: automationJobFailure,
      execute: async () => {
        const payload = buildAutomationJobRequest(currentEditor.draft);
        return currentEditor.mode === "create"
          ? createMutation.mutateAsync(payload)
          : updateMutation.mutateAsync({
              data: automationJobUpdateFromDraft(payload),
              id: currentEditor.id,
            });
      },
      successMessage: (mode, result) =>
        mode === "create" ? `Created job ${result.name}.` : `Updated job ${result.name}.`,
      workspaceId: activeWorkspaceId,
    });
  };

  const editorDialogProps = {
    activeWorkspaceId,
    agents: agentsQuery.data ?? [],
    agentsError: agentCatalogErrorMessage(agentsQuery.error),
    agentsLoading: agentsQuery.isLoading,
    handle,
    workspaces,
    editor: editor
      ? {
          ...editor,
          kind: "jobs" as const,
          isPending: pendingRequest !== null,
          onCancel: close,
          onChange: (draft: CreateAutomationJobRequest) =>
            store.trigger.draftChanged({ draft: { ...editor, draft } }),
          onSubmit: handleSubmit,
        }
      : null,
  };

  return { close, editor, editorDialogProps, openCreate, openEdit, openLoopCreate };
}

interface TriggerEditorParams {
  activeWorkspaceId?: string | null;
  onSaved?: (trigger: AutomationTrigger) => void;
  workspaces?: ReadonlyArray<WorkspaceOption>;
}

/**
 * Modal create/edit controller for automation triggers. Mirrors the job editor
 * but carries the trigger form's inline submit-error surface.
 */
export function useAutomationTriggerEditor({
  activeWorkspaceId,
  onSaved,
  workspaces,
}: TriggerEditorParams) {
  const { editor, pendingRequest, store, submitError } = useWorkspaceBoundEditor(
    triggerEditorLogic,
    activeWorkspaceId
  );
  const [handle] = useState(createAutomationDialogHandle);
  const agentsQuery = useAgents(activeWorkspaceId);
  const createMutation = useCreateAutomationTrigger();
  const updateMutation = useUpdateAutomationTrigger();
  const handleSaveSucceeded = useEffectEvent((result: AutomationTrigger) => onSaved?.(result));

  useEffect(() => {
    const succeeded = store.on("saveSucceeded", event => handleSaveSucceeded(event.result));
    return () => {
      succeeded.unsubscribe();
    };
  }, [store]);

  const openCreate = () => {
    store.trigger.editorOpened({
      editor: { draft: createAutomationTriggerDraft(activeWorkspaceId), mode: "create" },
      workspaceId: activeWorkspaceId,
    });
  };
  const openLoopCreate = (loop: string) => {
    store.trigger.editorOpened({
      editor: { draft: createLoopTargetTriggerDraft(activeWorkspaceId, loop), mode: "create" },
      workspaceId: activeWorkspaceId,
    });
  };
  const openEdit = (trigger: AutomationTrigger) => {
    store.trigger.editorOpened({
      editor: { draft: automationTriggerToDraft(trigger), id: trigger.id, mode: "edit" },
      workspaceId: activeWorkspaceId,
    });
  };
  const close = () => {
    store.trigger.editorClosed();
  };

  const handleSubmit = () => {
    const currentEditor = editor;
    if (!currentEditor) return;
    store.trigger.submissionRequested({
      describeFailure: automationTriggerFailure,
      execute: async () => {
        const payload = buildAutomationTriggerRequest(currentEditor.draft);
        return currentEditor.mode === "create"
          ? createMutation.mutateAsync(payload)
          : updateMutation.mutateAsync({
              data: automationTriggerUpdateFromDraft(payload),
              id: currentEditor.id,
            });
      },
      successMessage: (mode, result) =>
        mode === "create" ? `Created trigger ${result.name}.` : `Updated trigger ${result.name}.`,
      workspaceId: activeWorkspaceId,
    });
  };

  const editorDialogProps = {
    activeWorkspaceId,
    agents: agentsQuery.data ?? [],
    agentsError: agentCatalogErrorMessage(agentsQuery.error),
    agentsLoading: agentsQuery.isLoading,
    handle,
    workspaces,
    editor: editor
      ? {
          ...editor,
          kind: "triggers" as const,
          isPending: pendingRequest !== null,
          onCancel: close,
          onChange: (draft: CreateAutomationTriggerRequest) => {
            store.trigger.draftChanged({ draft: { ...editor, draft } });
          },
          onSubmit: handleSubmit,
          submitError,
        }
      : null,
  };

  return { close, editor, editorDialogProps, openCreate, openEdit, openLoopCreate };
}

function automationJobFailure(error: unknown): WorkspaceEditorFailure {
  const message = error instanceof Error ? error.message : "Failed to save automation job";
  return { submitError: message, toastError: message };
}

function automationTriggerFailure(error: unknown): WorkspaceEditorFailure {
  const detail =
    error instanceof Error && error.message.trim() !== ""
      ? error.message.trim().replace(/\.$/, "")
      : "Failed to save automation trigger";
  const submitError =
    error instanceof AutomationApiError && error.status === 0
      ? "Failed to save automation trigger. Check your connection and try again."
      : `${detail}. Review the target and try again.`;
  return { submitError, toastError: detail };
}
