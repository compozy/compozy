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
import { useGatewayCapabilities } from "@/systems/gateway";
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
  // Task create registers only on the local surface set (`routes.go`
  // `includeTaskMutations`) — POST /api/tasks (and the first-run enqueue), with
  // no 403 code for the truthful loopback-only state to render (BR-3). The
  // modal goes absent on remote tiers (BR-1 — absent, never disabled), so the
  // deep link lands on the catalog read view beneath, whose remote branch
  // already renders the remote-note empty state instead of the local
  // affordance.
  const { localTaskLifecycle } = useGatewayCapabilities();
  const navigate = useTasksNavigation();
  const liveDataEnabled = useCurrentWindowLiveDataEnabled();
  const backToCatalog = () =>
    navigate({ pathname: "/tasks", search: taskCatalogSearchFor(catalogMode, search) });
  const page = useTaskCreateState(search, navigate, { liveDataEnabled });

  if (!localTaskLifecycle) return null;

  return (
    <TaskEditorModal
      canSubmit={
        !page.isScopeResolving &&
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
      profileDestination={page.profileDestination}
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
  // Task PATCH (`UpdateTask`) registers only on the local surface set
  // (`routes.go` `includeTaskMutations`), with no 403 code for the truthful
  // loopback-only state to render (BR-3). The modal goes absent on remote
  // tiers (BR-1 — absent, never disabled), so the deep link lands on the task
  // detail read view beneath. The gate is deliberately presence-only (no
  // redirect): firing a navigation while the tier is still unlatched would
  // bounce a local operator off the deep link before `/api/status` resolves.
  const { localTaskLifecycle } = useGatewayCapabilities();
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

  if (!localTaskLifecycle) return null;

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
