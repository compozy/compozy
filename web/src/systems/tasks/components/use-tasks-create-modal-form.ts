import type { ChangeEvent, FormEvent } from "react";

import type { TaskEditorDraft as CreateTaskDraftInput } from "../lib/task-editor";
import type { TaskOwnerKind, TaskPriority, TaskScope } from "../types";

interface UseTasksCreateModalFormParams {
  onDraftChange: (
    next: CreateTaskDraftInput | ((current: CreateTaskDraftInput) => CreateTaskDraftInput)
  ) => void;
  onSubmit: (draft: CreateTaskDraftInput, asDraft: boolean) => Promise<unknown> | void;
  draft: CreateTaskDraftInput;
}

export function useTasksCreateModalForm({
  draft,
  onDraftChange,
  onSubmit,
}: UseTasksCreateModalFormParams) {
  const updateText =
    (field: keyof CreateTaskDraftInput) =>
    (event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>) => {
      const value = event.target.value;
      onDraftChange(current => ({ ...current, [field]: value }));
    };

  const updateScope = (scope: TaskScope) => onDraftChange(current => ({ ...current, scope }));

  const updateWorkspace = (workspaceId: string) =>
    onDraftChange(current => ({ ...current, workspaceId }));

  const updatePriority = (priority: TaskPriority) =>
    onDraftChange(current => ({ ...current, priority }));

  const updateOwnerKind = (ownerKind: TaskOwnerKind | "") =>
    onDraftChange(current => ({
      ...current,
      ownerKind,
      ownerRef: current.ownerKind === ownerKind ? current.ownerRef : "",
    }));

  const updateMaxAttempts = (maxAttempts: number | null) =>
    onDraftChange(current => ({ ...current, maxAttempts }));

  const updateApprovalPolicy = (approvalPolicy: "none" | "manual") =>
    onDraftChange(current => ({ ...current, approvalPolicy }));

  const updateAutoEnqueue = (next: boolean) =>
    onDraftChange(current => ({ ...current, autoEnqueueOnReady: next }));

  const updateSaveAsDraft = (next: boolean) =>
    onDraftChange(current => ({ ...current, saveAsDraft: next }));

  const submitForm = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    void onSubmit(draft, draft.saveAsDraft);
  };

  const submitDraft = () => {
    void onSubmit(draft, true);
  };

  return {
    submitDraft,
    submitForm,
    updateApprovalPolicy,
    updateAutoEnqueue,
    updateMaxAttempts,
    updateOwnerKind,
    updatePriority,
    updateSaveAsDraft,
    updateScope,
    updateText,
    updateWorkspace,
  };
}
