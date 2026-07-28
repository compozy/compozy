import { useState, type SetStateAction } from "react";
import { toast } from "sonner";

import { useUpdateTask } from "./use-task-actions";
import { useTaskExecutionProfile } from "./use-task-profile";
import { useTask } from "./use-tasks";
import {
  buildUpdateTaskRequest,
  EMPTY_TASK_EDITOR_DRAFT,
  taskEditorDraftFromTask,
  type TaskEditorDraft,
} from "@/systems/tasks/lib/task-editor";

/**
 * Edit-form state for one task. `onSaved` runs after the update lands so the
 * host owns dismissal — the hook never encodes a location.
 */
export function useTaskEditState(id: string | undefined, onSaved: () => void) {
  const detailQuery = useTask(id ?? "", { enabled: Boolean(id) });
  const profileQuery = useTaskExecutionProfile(id ?? "", { enabled: Boolean(id) });
  const updateMutation = useUpdateTask();
  const detail = detailQuery.data ?? null;
  const task = detail?.task ?? null;
  const profile = profileQuery.data ?? null;

  const taskKey =
    task && profile ? `${task.id}:${task.updated_at}:${profile.updated_at}` : "pending";
  const sourceDraft =
    task && profile ? taskEditorDraftFromTask(task, profile) : EMPTY_TASK_EDITOR_DRAFT;
  const [draftState, setDraftState] = useState({ draft: sourceDraft, key: taskKey });
  const draft = draftState.key === taskKey ? draftState.draft : sourceDraft;
  const setDraft = (update: SetStateAction<TaskEditorDraft>) => {
    setDraftState(current => {
      const currentDraft = current.key === taskKey ? current.draft : sourceDraft;
      return {
        draft: typeof update === "function" ? update(currentDraft) : update,
        key: taskKey,
      };
    });
  };

  const handleSubmit = async (nextDraft: TaskEditorDraft) => {
    if (!id || !task || !profile) return null;
    try {
      await updateMutation.mutateAsync({ id, data: buildUpdateTaskRequest(nextDraft) });
      toast.success("Task updated.");
      onSaved();
      return true;
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to update task");
      return null;
    }
  };

  return {
    draft,
    handleSubmit,
    isInitialized: task !== null && profile !== null,
    isLoading: (detailQuery.isLoading && !task) || (profileQuery.isLoading && !profile),
    isSubmitting: updateMutation.isPending,
    setDraft,
    task,
    workspaceName: task?.scope === "workspace" ? (task.workspace_id ?? null) : null,
  };
}
