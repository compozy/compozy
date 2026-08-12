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
  const backToCatalog = () =>
    navigate({ pathname: "/tasks", search: taskCatalogSearchFor(catalogMode, search) });
  const page = useTaskCreateState(search, navigate);

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
      userHomeDir={page.userHomeDir}
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
  const backToTask = () =>
    navigate({ pathname: `/tasks/${encodeURIComponent(taskId)}`, search: { ...search } });
  const page = useTaskEditState(taskId, backToTask);
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
    />
  );
}
