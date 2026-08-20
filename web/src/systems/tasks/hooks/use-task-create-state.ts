import type { SetStateAction } from "react";
import { useSelector, useStore } from "@xstate/store-react";
import { toast } from "sonner";

import { useStoreBinding } from "@/hooks/use-store-binding";
import { useCreateChildTask, useCreateTask, useEnqueueTaskRun } from "./use-task-actions";
import { taskEditorDraftLogic } from "./task-editor-draft-store";
import {
  requestTaskEditorSubmission,
  taskEditorSubmissionLogic,
} from "./task-editor-submission-store";
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
import { toWorkspaceCommandSelectOptions, useActiveWorkspace } from "@/systems/workspace";

interface TaskCreateLocation {
  pathname: string;
  search: Record<string, unknown>;
}

function createTaskDraftForScope(
  templateId: TaskTemplateId,
  scope: "global" | "workspace",
  workspaceId: string | null
): TaskEditorDraft {
  const draft = createTaskEditorDraft(templateId, workspaceId);
  return scope === "workspace" && workspaceId === null ? { ...draft, scope: "workspace" } : draft;
}

export function useTaskCreateState(
  search: TaskCreateSearch,
  onNavigate: (location: TaskCreateLocation) => void,
  options: { liveDataEnabled?: boolean } = {}
) {
  const liveDataEnabled = options.liveDataEnabled ?? true;
  const { activeWorkspaceId, scope, workspaces } = useActiveWorkspace({
    enabled: liveDataEnabled,
  });
  const createMutation = useCreateTask();
  const createChildMutation = useCreateChildTask();
  const enqueueMutation = useEnqueueTaskRun();
  const submissionStore = useStore(taskEditorSubmissionLogic);

  const templateId = search.template ?? DEFAULT_TASK_TEMPLATE_ID;
  const activeTaskScope = taskScopeForActiveWorkspace(scope, activeWorkspaceId);
  const isScopeResolving = scope === "workspace" && activeTaskScope === null;
  const createDraftWorkspaceId =
    activeTaskScope?.scope === "workspace" ? activeTaskScope.workspace : undefined;
  const workspaceKey = isScopeResolving
    ? "workspace:pending"
    : (createDraftWorkspaceId ?? "global");
  const bindingKey = `${workspaceKey}\u0000${templateId}`;
  const { store } = useStoreBinding(
    bindingKey,
    () =>
      taskEditorDraftLogic.createStore({
        draft: createTaskDraftForScope(templateId, scope, createDraftWorkspaceId ?? null),
        scopeKey: workspaceKey,
      }),
    previous => {
      const previousState = previous.getSnapshot().context;
      return taskEditorDraftLogic.createStore({
        draft:
          previousState.scopeKey === workspaceKey
            ? applyTaskTemplateToEditorDraft(previousState.draft, templateId)
            : createTaskDraftForScope(templateId, scope, createDraftWorkspaceId ?? null),
        scopeKey: workspaceKey,
      });
    }
  );
  const draft = useSelector(store, snapshot => snapshot.context.draft);
  const submissionPhase = useSelector(submissionStore, snapshot => snapshot.context.phase);

  const setDraft = (update: SetStateAction<TaskEditorDraft>) => {
    store.trigger.draftChanged({ update });
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

  const handleSubmit = (nextDraft: TaskEditorDraft, asDraft: boolean) => {
    if (isScopeResolving) return Promise.resolve(null);
    return requestTaskEditorSubmission(
      submissionStore,
      async () => {
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
              console.error("Failed to enqueue the first task run", runError);
              toast.error("Task created, but its first run didn't start.");
            }
          }

          toast.success(
            created.draft ? `Saved draft "${trimmedTitle}".` : `Created task "${trimmedTitle}".`
          );

          if (created.id) {
            onNavigate({ pathname: `/tasks/${encodeURIComponent(created.id)}`, search: {} });
          }

          return created;
        } catch (error) {
          console.error("Failed to create task", error);
          toast.error("Couldn't create the task.");
          return null;
        }
      },
      error => {
        console.error("Failed to create task", error);
        toast.error("Couldn't create the task.");
      }
    );
  };

  return {
    draft,
    handleSubmit,
    handleTemplateChange,
    isScopeResolving,
    isSubmitting:
      submissionPhase === "submitting" ||
      createMutation.isPending ||
      createChildMutation.isPending ||
      enqueueMutation.isPending,
    setDraft,
    template: getTaskTemplate(templateId),
    templateId,
    workspaces: toWorkspaceCommandSelectOptions(workspaces),
  };
}
