import { useRef, useState, type SetStateAction } from "react";
import { toast } from "sonner";

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

interface WorkspaceEditorState<T> {
  editor: T | null;
  workspaceId: string | null | undefined;
}

function useWorkspaceBoundEditor<T>(workspaceId: string | null | undefined) {
  const [state, setState] = useState<WorkspaceEditorState<T>>({
    editor: null,
    workspaceId,
  });
  const contextMatches = state.workspaceId === workspaceId;
  if (!contextMatches) {
    setState({ editor: null, workspaceId });
  }

  const setEditor = (action: SetStateAction<T | null>) => {
    setState(current => {
      const currentEditor = current.workspaceId === workspaceId ? current.editor : null;
      const editor =
        typeof action === "function"
          ? (action as (value: T | null) => T | null)(currentEditor)
          : action;
      return { editor, workspaceId };
    });
  };

  return { editor: contextMatches ? state.editor : null, setEditor };
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
  const { editor, setEditor } = useWorkspaceBoundEditor<JobEditorState>(activeWorkspaceId);
  const inFlightRef = useRef(false);
  const [handle] = useState(createAutomationDialogHandle);
  // The target selector reads the workspace catalog; a global editor still lists
  // the active workspace's agents because that is what the job can address.
  const agentsQuery = useAgents(activeWorkspaceId);
  const createMutation = useCreateAutomationJob();
  const updateMutation = useUpdateAutomationJob();

  const openCreate = () =>
    setEditor({ draft: createAutomationJobDraft(activeWorkspaceId), mode: "create" });
  const openLoopCreate = (loop: string) =>
    setEditor({ draft: createLoopTargetJobDraft(activeWorkspaceId, loop), mode: "create" });
  const openEdit = (job: AutomationJob) =>
    setEditor({ draft: automationJobToDraft(job), id: job.id, mode: "edit" });
  const close = () => setEditor(null);

  const handleSubmit = async () => {
    if (!editor || inFlightRef.current) return;
    inFlightRef.current = true;
    try {
      const payload = buildAutomationJobRequest(editor.draft);
      const job =
        editor.mode === "create"
          ? await createMutation.mutateAsync(payload)
          : await updateMutation.mutateAsync({
              data: automationJobUpdateFromDraft(payload),
              id: editor.id,
            });
      setEditor(null);
      toast.success(
        editor.mode === "create" ? `Created job ${job.name}.` : `Updated job ${job.name}.`
      );
      onSaved?.(job);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to save automation job");
    }
    inFlightRef.current = false;
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
          isPending: createMutation.isPending || updateMutation.isPending,
          onCancel: close,
          onChange: (draft: CreateAutomationJobRequest) =>
            setEditor(current => (current ? { ...current, draft } : current)),
          onSubmit: () => {
            void handleSubmit();
          },
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
  const { editor, setEditor } = useWorkspaceBoundEditor<TriggerEditorState>(activeWorkspaceId);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const inFlightRef = useRef(false);
  const [handle] = useState(createAutomationDialogHandle);
  const agentsQuery = useAgents(activeWorkspaceId);
  const createMutation = useCreateAutomationTrigger();
  const updateMutation = useUpdateAutomationTrigger();

  const openCreate = () => {
    setSubmitError(null);
    setEditor({ draft: createAutomationTriggerDraft(activeWorkspaceId), mode: "create" });
  };
  const openLoopCreate = (loop: string) => {
    setSubmitError(null);
    setEditor({ draft: createLoopTargetTriggerDraft(activeWorkspaceId, loop), mode: "create" });
  };
  const openEdit = (trigger: AutomationTrigger) => {
    setSubmitError(null);
    setEditor({ draft: automationTriggerToDraft(trigger), id: trigger.id, mode: "edit" });
  };
  const close = () => {
    setSubmitError(null);
    setEditor(null);
  };

  const handleSubmit = async () => {
    if (!editor || inFlightRef.current) return;
    inFlightRef.current = true;
    setSubmitError(null);
    try {
      const payload = buildAutomationTriggerRequest(editor.draft);
      const trigger =
        editor.mode === "create"
          ? await createMutation.mutateAsync(payload)
          : await updateMutation.mutateAsync({
              data: automationTriggerUpdateFromDraft(payload),
              id: editor.id,
            });
      setEditor(null);
      toast.success(
        editor.mode === "create"
          ? `Created trigger ${trigger.name}.`
          : `Updated trigger ${trigger.name}.`
      );
      onSaved?.(trigger);
    } catch (error) {
      const detail =
        error instanceof Error && error.message.trim() !== ""
          ? error.message.trim().replace(/\.$/, "")
          : "Failed to save automation trigger";
      const message =
        error instanceof AutomationApiError && error.status === 0
          ? "Failed to save automation trigger. Check your connection and try again."
          : `${detail}. Review the target and try again.`;
      setSubmitError(message);
      toast.error(detail);
    }
    inFlightRef.current = false;
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
          isPending: createMutation.isPending || updateMutation.isPending,
          onCancel: close,
          onChange: (draft: CreateAutomationTriggerRequest) => {
            setSubmitError(null);
            setEditor(current => (current ? { ...current, draft } : current));
          },
          onSubmit: () => {
            void handleSubmit();
          },
          submitError,
        }
      : null,
  };

  return { close, editor, editorDialogProps, openCreate, openEdit, openLoopCreate };
}
