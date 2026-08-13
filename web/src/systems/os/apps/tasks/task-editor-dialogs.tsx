import { useTasksNavigation } from "./hooks/use-tasks-navigation";
import {
  type ResolvedTaskDetailSearch,
  taskCatalogSearchFor,
  type TaskCreateSearch,
  TaskEditorModal,
  type TaskEditorModalStatus,
  type TaskViewMode,
  useTaskCreateState,
  useTaskEditState,
} from "@/systems/tasks";
import { useCurrentWindowLiveDataEnabled } from "../../hooks/use-window-live-data-enabled";
import { toWorkspaceCommandSelectOptions, useActiveWorkspace } from "@/systems/workspace";

/**
 * Task creation is a focused single-entity form on the `md` host, so it stays a
 * window-scoped dialog layered over the catalog (`MODAL-STANDARD.md` § Hosts).
 * The `/tasks/new` location keeps it deep-linkable and template-addressable.
 */
export function TaskCreateDialog({
  catalogMode,
  search,
}: {
  catalogMode: TaskViewMode;
  search: TaskCreateSearch;
}) {
  const navigate = useTasksNavigation();
  const liveDataEnabled = useCurrentWindowLiveDataEnabled();
  const backToCatalog = () =>
    navigate({ pathname: "/tasks", search: taskCatalogSearchFor(catalogMode, search) });
  const page = useTaskCreateState(search, navigate, { liveDataEnabled });

  return (
    <TaskEditorModal
      canSubmit={
        page.draft.title.trim().length > 0 &&
        (page.draft.scope === "global" || Boolean(page.draft.workspaceId))
      }
      draft={page.draft}
      isSubmitting={page.isSubmitting}
      mode="new"
      onDraftChange={page.setDraft}
      onOpenChange={open => {
        if (!open) backToCatalog();
      }}
      onSubmit={page.handleSubmit}
      onTemplateChange={page.handleTemplateChange}
      open
      templateId={page.templateId}
      workspaces={page.workspaces}
    />
  );
}

/** Task editing layers the same dialog over the task-detail location it returns to. */
export function TaskEditDialog({
  search,
  taskId,
}: {
  search: ResolvedTaskDetailSearch;
  taskId: string;
}) {
  const navigate = useTasksNavigation();
  const liveDataEnabled = useCurrentWindowLiveDataEnabled();
  // The edit chip echoes the entity's own scope; the list only resolves its name.
  const { workspaces } = useActiveWorkspace();
  const backToTask = () =>
    navigate({ pathname: `/tasks/${encodeURIComponent(taskId)}`, search: { ...search } });
  const page = useTaskEditState(taskId, backToTask, { liveDataEnabled });
  const status: TaskEditorModalStatus = page.isLoading
    ? "loading"
    : page.isInitialized
      ? "ready"
      : "unavailable";

  return (
    <TaskEditorModal
      canSubmit={page.draft.title.trim().length > 0}
      draft={page.draft}
      isSubmitting={page.isSubmitting}
      mode="edit"
      onDraftChange={page.setDraft}
      onOpenChange={open => {
        if (!open) backToTask();
      }}
      onSubmit={page.handleSubmit}
      open
      status={status}
      workspaces={toWorkspaceCommandSelectOptions(workspaces)}
    />
  );
}
