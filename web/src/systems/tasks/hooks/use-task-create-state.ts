import type { SetStateAction } from "react";
import { useSelector, useStore } from "@xstate/store-react";
import { toast } from "sonner";

import { useStoreBinding } from "@/hooks/use-store-binding";
import { useGatewayCapabilities } from "@/systems/gateway";
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
import { createdInProfileToast, useProfileReadScope } from "@/systems/profiles";

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
  const profile = useProfileReadScope();
  const createMutation = useCreateTask();
  const createChildMutation = useCreateChildTask();
  const enqueueMutation = useEnqueueTaskRun();
  // Origin gate for the submit path below: task create (POST /api/tasks and
  // the child-task variant) and the first-run enqueue register only on the
  // local surface set (`routes.go` `includeTaskMutations`), with no 403 code
  // for the truthful loopback-only state to render (BR-3). The create dialog
  // goes absent at the composition layer (BR-1); this gate is the backstop
  // that keeps a wiring regression from firing a doomed POST on a remote tier
  // (mirrors the task-detail lifecycle verbs).
  const { localTaskLifecycle } = useGatewayCapabilities();
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
    if (!localTaskLifecycle || isScopeResolving) return Promise.resolve(null);
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

          // Under the aggregate the destination is not the context on screen, so
          // the confirmation names the owner. It names the one the daemon
          // returned rather than the one we asked for: a toast that echoed the
          // request would keep saying "default" even if the filing went
          // elsewhere, which is exactly the misfile it exists to surface
          // (US-012.AC-2, US-012.EC-1).
          const createdLine = created.draft
            ? `Saved draft "${trimmedTitle}".`
            : `Created task "${trimmedTitle}".`;
          toast.success(
            profile.aggregate
              ? `${createdLine} ${createdInProfileToast(created.profile_name)}`
              : createdLine
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
    profileDestination: profile.aggregate ? profile.destination : null,
    setDraft,
    template: getTaskTemplate(templateId),
    templateId,
    workspaces: toWorkspaceCommandSelectOptions(workspaces),
  };
}
