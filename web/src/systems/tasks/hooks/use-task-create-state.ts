import { useRef, useState, type SetStateAction } from "react";
import { toast } from "sonner";

import { useCreateChildTask, useCreateTask, useEnqueueTaskRun } from "./use-task-actions";
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
import {
  toWorkspaceCommandSelectOptions,
  useActiveWorkspace,
  useUserHomeDir,
} from "@/systems/workspace";
import { taskScopeForActiveWorkspace } from "../lib/workspace-scope";

interface TaskCreateLocation {
  pathname: string;
  search: Record<string, unknown>;
}

interface TaskCreateDraftState {
  draft: TaskEditorDraft;
  templateId: TaskTemplateId;
  workspaceKey: string;
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
  const submitInFlightRef = useRef(false);

  const templateId = search.template ?? DEFAULT_TASK_TEMPLATE_ID;
  const activeTaskScope = taskScopeForActiveWorkspace(activeWorkspace, userHomeDir);
  const createDraftWorkspaceId =
    activeTaskScope?.scope === "workspace" ? activeTaskScope.workspace : undefined;
  const workspaceKey = createDraftWorkspaceId ?? "global";
  const [draftState, setDraftState] = useState<TaskCreateDraftState>(() => ({
    draft: createTaskEditorDraft(templateId, createDraftWorkspaceId),
    templateId,
    workspaceKey,
  }));
  const currentDraftState =
    draftState.workspaceKey === workspaceKey && draftState.templateId === templateId
      ? draftState
      : {
          draft:
            draftState.workspaceKey !== workspaceKey
              ? createTaskEditorDraft(templateId, createDraftWorkspaceId)
              : applyTaskTemplateToEditorDraft(draftState.draft, templateId),
          templateId,
          workspaceKey,
        };
  if (currentDraftState !== draftState) {
    setDraftState(currentDraftState);
  }
  const draft = currentDraftState.draft;

  const setDraft = (update: SetStateAction<TaskEditorDraft>) =>
    setDraftState(current => {
      const currentDraft =
        current.workspaceKey === workspaceKey && current.templateId === templateId
          ? current.draft
          : draft;
      return {
        draft: typeof update === "function" ? update(currentDraft) : update,
        templateId,
        workspaceKey,
      };
    });

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
    if (submitInFlightRef.current) {
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

    submitInFlightRef.current = true;
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

      submitInFlightRef.current = false;
      return created;
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to create task");
      submitInFlightRef.current = false;
      return null;
    }
  };

  return {
    draft,
    handleSubmit,
    handleTemplateChange,
    isSubmitting:
      createMutation.isPending || createChildMutation.isPending || enqueueMutation.isPending,
    setDraft,
    template: getTaskTemplate(templateId),
    templateId,
    userHomeDir,
    workspaces: toWorkspaceCommandSelectOptions(workspaces),
  };
}
