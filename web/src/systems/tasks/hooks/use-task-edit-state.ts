import type { SetStateAction } from "react";
import { useSelector } from "@xstate/store-react";
import { toast } from "sonner";

import { useStoreBinding } from "@/hooks/use-store-binding";
import { useUpdateTask } from "./use-task-actions";
import { useTaskExecutionProfile } from "./use-task-profile";
import { useTask } from "./use-tasks";
import {
  buildUpdateTaskRequest,
  EMPTY_TASK_EDITOR_DRAFT,
  taskEditorDraftFromTask,
  type TaskEditorDraft,
} from "@/systems/tasks/lib/task-editor";
import { taskEditorDraftLogic } from "./task-editor-draft-store";

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
  const { store } = useStoreBinding(
    taskKey,
    () =>
      taskEditorDraftLogic.createStore({
        draft: sourceDraft,
        scopeKey: taskKey,
        variantKey: "edit",
      }),
    () =>
      taskEditorDraftLogic.createStore({
        draft: sourceDraft,
        scopeKey: taskKey,
        variantKey: "edit",
      }),
    (current, nextKey) => current.key !== nextKey
  );
  const draft = useSelector(store, snapshot => snapshot.context.draft);
  const setDraft = (update: SetStateAction<TaskEditorDraft>) => {
    const currentDraft = store.getSnapshot().context.draft;
    store.trigger.draftChanged({
      draft: typeof update === "function" ? update(currentDraft) : update,
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
