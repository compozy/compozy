import type { SetStateAction } from "react";
import { useSelector } from "@xstate/store-react";
import { toast } from "sonner";

import { useStoreBinding } from "@/hooks/use-store-binding";
import { useCreateChildTask, useCreateTask, useEnqueueTaskRun } from "./use-task-actions";
import { taskEditorDraftLogic } from "./task-editor-draft-store";
import {
  applyTaskTemplateToEditorDraft,
  buildCreateChildTaskRequest,
  buildCreateTaskRequest,
  createTaskEditorDraft,
  type TaskEditorDraft,
} from "@/systems/tasks/lib/task-editor";
import type { TaskCreateSearch } from "@/systems/tasks/lib/task-location-search";
import {
  DEFAULT_TASK_TEMPLATE_ID,
  getTaskTemplate,
  type TaskTemplateId,
} from "@/systems/tasks/lib/task-templates";

import { taskScopeForActiveWorkspace } from "../lib/workspace-scope";
import {
  toWorkspaceCommandSelectOptions,
  useActiveWorkspace,
  useUserHomeDir,
} from "@/systems/workspace";

interface TaskCreateLocation {
  pathname: string;
  search: Record<string, unknown>;
}

export function useTaskCreateState(
  search: TaskCreateSearch,
  onNavigate: (location: TaskCreateLocation) => void
) {
  const { activeWorkspace, workspaces } = useActiveWorkspace();
  const userHomeDir = useUserHomeDir();
  const createMutation = useCreateTask();
  const createChildMutation = useCreateChildTask();
  const enqueueMutation = useEnqueueTaskRun();

  const templateId = search.template ?? DEFAULT_TASK_TEMPLATE_ID;
  const activeTaskScope = taskScopeForActiveWorkspace(activeWorkspace, userHomeDir);
  const createDraftWorkspaceId =
    activeTaskScope?.scope === "workspace" ? activeTaskScope.workspace : undefined;
  const workspaceKey = createDraftWorkspaceId ?? "global";
  const bindingKey = `${workspaceKey}\u0000${templateId}`;
  const { store } = useStoreBinding(
    bindingKey,
    () =>
      taskEditorDraftLogic.createStore({
        draft: createTaskEditorDraft(templateId, createDraftWorkspaceId),
        scopeKey: workspaceKey,
        variantKey: templateId,
      }),
    previous => {
      const previousState = previous.getSnapshot().context;
      return taskEditorDraftLogic.createStore({
        draft:
          previousState.scopeKey === workspaceKey
            ? applyTaskTemplateToEditorDraft(previousState.draft, templateId)
            : createTaskEditorDraft(templateId, createDraftWorkspaceId),
        scopeKey: workspaceKey,
        variantKey: templateId,
      });
    },
    (current, nextKey) => current.key !== nextKey
  );
  const draft = useSelector(store, snapshot => snapshot.context.draft);
  const submissionPhase = useSelector(store, snapshot => snapshot.context.submissionPhase);

  const setDraft = (update: SetStateAction<TaskEditorDraft>) => {
    const currentDraft = store.getSnapshot().context.draft;
    store.trigger.draftChanged({
      draft: typeof update === "function" ? update(currentDraft) : update,
    });
  };

  const handleTemplateChange = (nextTemplateId: TaskTemplateId) => {
    const { template: _template, ...catalogSearch } = search;
    onNavigate({
      pathname: "/tasks/new",
      search: {
        ...catalogSearch,
        ...(nextTemplateId === DEFAULT_TASK_TEMPLATE_ID ? {} : { template: nextTemplateId }),
      },
    });
  };

  const handleSubmit = async (nextDraft: TaskEditorDraft, asDraft: boolean) => {
    if (store.getSnapshot().context.submissionPhase === "submitting") {
      return null;
    }

    const trimmedTitle = nextDraft.title.trim();
    if (!trimmedTitle) {
      toast.error("Provide a title before creating the task.");
      return null;
    }

    if (nextDraft.scope === "workspace" && !nextDraft.workspaceId) {
      toast.error("Select a workspace before creating a workspace task.");
      return null;
    }

    const parentTaskId = nextDraft.parentTaskId.trim();
    const isChildTask = parentTaskId.length > 0;

    store.trigger.submissionStarted();
    try {
      const created = isChildTask
        ? await createChildMutation.mutateAsync({
            parentId: parentTaskId,
            data: buildCreateChildTaskRequest(nextDraft, {
              asDraft,
              templateId,
            }),
          })
        : await createMutation.mutateAsync(
            buildCreateTaskRequest(nextDraft, {
              asDraft,
              templateId,
            })
          );
      const wantsImmediateRun =
        !created.draft && getTaskTemplate(templateId).preview.enqueueOnSubmit;
      if (wantsImmediateRun && created.id) {
        try {
          await enqueueMutation.mutateAsync({ id: created.id });
        } catch (runError) {
          const message =
            runError instanceof Error ? runError.message : "Failed to enqueue first run";
          toast.error(`Task created, but enqueue failed: ${message}`);
        }
      }

      toast.success(
        created.draft ? `Saved draft "${trimmedTitle}".` : `Created task "${trimmedTitle}".`
      );

      if (created.id) {
        onNavigate({ pathname: `/tasks/${encodeURIComponent(created.id)}`, search: {} });
      }

      store.trigger.submissionFinished();
      return created;
    } catch (error) {
      store.trigger.submissionFinished();
      toast.error(error instanceof Error ? error.message : "Failed to create task");
      return null;
    }
  };

  return {
    draft,
    handleSubmit,
    handleTemplateChange,
    isSubmitting:
      submissionPhase === "submitting" ||
      createMutation.isPending ||
      createChildMutation.isPending ||
      enqueueMutation.isPending,
    setDraft,
    template: getTaskTemplate(templateId),
    templateId,
    userHomeDir,
    workspaces: toWorkspaceCommandSelectOptions(workspaces),
  };
}
