// Invariant: catalog route views lead the window toolbar and are the only strip
// content when the selected view has no peer controls.
// Owning layer: Tasks catalog route composition.
// Canonical suite: TasksCatalogLocation component tests.
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { renderWithTopbar } from "@/test/render-with-topbar";

const mocks = vi.hoisted(() => ({
  emptyStateProps: null as null | {
    profileScopeLabel?: string | null;
    workspaceName?: string | null;
  },
  listSurfaceProps: null as null | { profile: unknown },
  navigate: vi.fn(),
  userOpen: vi.fn(),
  openTarget: vi.fn(),
  page: {
    // The listing's profile axis, as the page hook resolves it.
    profile: {
      aggregate: false as boolean,
      destination: "default" as string,
      scopeLabel: "default" as string | null,
      ownerOf: () => ({ id: "00000000000000000000000000", name: "default", archived: false }),
    },
    dashboard: null,
    dashboardError: null,
    dashboardLoading: false,
    effectiveSelectedTaskId: null,
    handleApproveTask: vi.fn(),
    handleArchiveTask: vi.fn(),
    handleDismissTask: vi.fn(),
    handleDrainScheduler: vi.fn(),
    handleInboxPriorityChange: vi.fn(),
    handleInboxStatusChange: vi.fn(),
    handleMarkTaskRead: vi.fn(),
    handleOwnerChange: vi.fn(),
    handlePauseScheduler: vi.fn(),
    handlePriorityChange: vi.fn(),
    handleRejectTask: vi.fn(),
    handleResumeScheduler: vi.fn(),
    handleRetryRun: vi.fn(),
    handleSortChange: vi.fn(),
    handleStatusChange: vi.fn(),
    hasActiveTaskScope: true,
    hasListFilters: false as boolean,
    hasMoreInbox: false,
    hasMoreTasks: false,
    inbox: [],
    inboxError: null,
    inboxLaneFilter: null,
    inboxLoading: false,
    inboxPriorityFilter: null,
    inboxStatusFilter: null,
    inboxUnreadCount: 0,
    inboxUnreadOnly: false,
    isEmpty: false,
    isLoadingMoreInbox: false,
    isLoadingMoreTasks: false,
    isSchedulerDrainPending: false,
    isSchedulerPausePending: false,
    isSchedulerResumePending: false,
    kanbanColumns: [],
    listError: null,
    listLoading: false,
    loadMoreInbox: vi.fn(),
    loadMoreTasks: vi.fn(),
    onSearchChange: vi.fn(),
    ownerFilter: null,
    ownerOptions: [],
    pendingApproveIds: new Set<string>(),
    pendingArchiveIds: new Set<string>(),
    pendingDismissIds: new Set<string>(),
    pendingMarkReadIds: new Set<string>(),
    pendingRejectIds: new Set<string>(),
    pendingRetryIds: new Set<string>(),
    priorityFilter: null,
    retryInbox: vi.fn(),
    retryTasks: vi.fn(),
    schedulerBacklog: null,
    schedulerBacklogError: null,
    schedulerBacklogLoading: false,
    schedulerErrorMessage: null,
    schedulerStatus: null,
    schedulerStatusError: null,
    schedulerStatusLoading: false,
    scopeError: null as Error | null,
    scopeLoading: false,
    searchQuery: "",
    setInboxSearchQuery: vi.fn(),
    setInboxUnreadToggle: vi.fn(),
    setSearchQuery: vi.fn(),
    sortBy: "updated_at",
    statusCounts: {},
    statusFilter: null,
    tasksCount: 3,
    unreadOnly: false,
    visibleTasks: [] as Array<{ id: string; workspace_id?: string }>,
    projectFilter: { visible: false, values: [], setValues: vi.fn(), clear: vi.fn(), field: null },
    projectBadgeOf: undefined,
    clearListFilters: vi.fn(),
  },
}));

vi.mock("@tanstack/react-router", async () => {
  const actual =
    await vi.importActual<typeof import("@tanstack/react-router")>("@tanstack/react-router");
  return {
    ...actual,
    Link: ({
      activeOptions: _activeOptions,
      children,
      search: _search,
      to: _to,
      ...props
    }: React.AnchorHTMLAttributes<HTMLAnchorElement> & {
      activeOptions?: unknown;
      search?: unknown;
      to?: string;
    }) => <a {...props}>{children}</a>,
    useNavigate: () => mocks.navigate,
  };
});

vi.mock("@/systems/os/hooks/use-os-shell", () => ({
  useOsShell: () => ({ coordinator: { userOpen: mocks.userOpen } }),
}));

vi.mock("@/systems/os/hooks/use-open-target", () => ({
  useOpenTarget: () => mocks.openTarget,
}));

vi.mock("@/systems/tasks/components/public-api", () => ({
  TasksDashboardView: () => <div data-testid="tasks-dashboard-view" />,
  TasksEmptyState: (props: {
    onSelectTemplate: (id: string) => void;
    profileScopeLabel?: string | null;
    workspaceName?: string | null;
  }) => {
    mocks.emptyStateProps = props;
    return (
      <div data-testid="tasks-empty-state">
        <button
          data-testid="tasks-empty-blank"
          onClick={() => props.onSelectTemplate("blank")}
          type="button"
        >
          Blank task
        </button>
      </div>
    );
  },
  TasksInboxView: () => <div data-testid="tasks-inbox-view" />,
  TasksKanbanBoard: (props: { onSelectTask: (taskId: string) => void }) => (
    <button
      data-testid="tasks-kanban-view"
      onClick={() => props.onSelectTask("task-beta")}
      type="button"
    >
      Open task
    </button>
  ),
  TasksListSurface: (props: { profile: unknown }) => {
    mocks.listSurfaceProps = props;
    return <div data-testid="tasks-list-surface" />;
  },
  TasksListToolbar: () => <div data-testid="tasks-list-toolbar" />,
}));

vi.mock("@/systems/tasks/hooks/use-tasks-page", () => ({
  useTasksPage: () => mocks.page,
}));

vi.mock("@/systems/tasks/lib/task-location-search", () => ({
  taskCatalogSearchFor: (mode: string) => ({ mode }),
  taskModeSearchFor: (mode: string) => ({ mode }),
  validateTasksSearch: <T,>(search: T) => search,
}));

vi.mock("@/systems/tasks/lib/task-templates", () => ({
  DEFAULT_TASK_TEMPLATE_ID: "default",
}));

import { TasksCatalogLocation } from "../tasks-catalog-location";

function renderCatalog(mode?: "dashboard" | "kanban") {
  return renderWithTopbar(<TasksCatalogLocation search={mode ? { mode } : {}} />);
}

describe("TasksCatalogLocation", () => {
  beforeEach(() => {
    mocks.emptyStateProps = null;
    mocks.listSurfaceProps = null;
    mocks.page.isEmpty = false;
    mocks.page.hasListFilters = false;
    mocks.page.profile.aggregate = false;
    mocks.page.profile.destination = "default";
    mocks.page.profile.scopeLabel = "default";
    mocks.userOpen.mockReset();
    mocks.openTarget.mockReset();
    mocks.page.visibleTasks = [];
    mocks.page.scopeLoading = false;
    mocks.page.scopeError = null;
    mocks.page.hasActiveTaskScope = true;
  });

  it("Should lead the strip with List and Kanban views while keeping them out of the head [UT-130]", () => {
    renderCatalog();

    const toolbar = document.querySelector("[data-slot='os-window-toolbar']");
    const views = screen.getByRole("navigation", { name: "Tasks views" });
    const filters = screen.getByTestId("tasks-list-toolbar");

    expect(screen.getByTestId("tasks-mode-list")).toHaveAttribute("aria-current", "page");
    expect(screen.getByTestId("tasks-mode-kanban")).toBeInTheDocument();
    expect(document.querySelector("[data-slot='topbar-nav']")).toBeNull();
    expect(toolbar).toContainElement(views);
    expect(toolbar).toContainElement(filters);
    expect(document.querySelector("[data-slot='topbar']")).not.toContainElement(views);
    expect(views.compareDocumentPosition(filters) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it("Should render a views-only strip when the selected task view has no peer controls [UT-130]", () => {
    renderCatalog("dashboard");

    const toolbar = document.querySelector("[data-slot='os-window-toolbar']");
    const views = screen.getByRole("navigation", { name: "Tasks views" });

    expect(screen.getByTestId("tasks-mode-dashboard")).toHaveAttribute("aria-current", "page");
    expect(toolbar).toContainElement(views);
    expect(screen.queryByTestId("tasks-list-toolbar")).not.toBeInTheDocument();
    expect(toolbar?.children).toHaveLength(1);
  });

  it("Should keep the strip and the mode's own surface in place while the scope resolves", () => {
    mocks.page.scopeLoading = true;
    mocks.page.hasActiveTaskScope = false;

    renderCatalog();

    // The strip needs no data, so its filters do not pop in when the scope settles; the mode's
    // surface renders its own loading state in its own container instead of a spinner gate.
    expect(screen.getByTestId("tasks-list-toolbar")).toBeInTheDocument();
    expect(screen.getByTestId("tasks-list-surface")).toBeInTheDocument();
    expect(screen.queryByTestId("tasks-scope-loading")).not.toBeInTheDocument();
    expect(screen.getByTestId("tasks-open-create")).toBeDisabled();
  });

  it("Should drop the filters and say so when the scope cannot resolve", () => {
    mocks.page.hasActiveTaskScope = false;
    mocks.page.scopeError = new Error("No active workspace is available for task scope.");

    renderCatalog();

    expect(screen.queryByTestId("tasks-list-toolbar")).not.toBeInTheDocument();
    expect(screen.getByTestId("tasks-scope-error")).toHaveTextContent(
      "No active workspace is available for task scope."
    );
  });

  it("Should swap the body to the new mode while the strip follows at once", async () => {
    const { rerender } = renderCatalog();
    expect(screen.getByTestId("tasks-list-surface")).toBeInTheDocument();

    rerender(<TasksCatalogLocation search={{ mode: "dashboard" }} />);

    // The body renders the new mode as a deferred swap, so it settles rather than lands in the
    // same pass; nothing of the old view stays mounted once it has.
    expect(await screen.findByTestId("tasks-dashboard-view")).toBeInTheDocument();
    expect(screen.queryByTestId("tasks-list-surface")).not.toBeInTheDocument();
    expect(screen.getByTestId("tasks-mode-dashboard")).toHaveAttribute("aria-current", "page");
    expect(screen.getByTestId("tasks-mode-list")).not.toHaveAttribute("aria-current");
  });

  it("Should show the head count only for the unfiltered list", () => {
    const { unmount } = renderCatalog();
    expect(document.querySelector("[data-slot='topbar-count']")).toHaveTextContent("3");
    unmount();

    // Under a filter the count is the filtered total, which the table header
    // already names; the head must not read it as the size of the list.
    mocks.page.hasListFilters = true;
    renderCatalog();
    expect(document.querySelector("[data-slot='topbar-count']")).toBeNull();
  });

  it("Should open the create route with the blank template from zero inventory", async () => {
    const user = userEvent.setup();
    mocks.page.isEmpty = true;

    renderCatalog();
    expect(screen.getByTestId("tasks-list-surface")).toBeInTheDocument();
    await user.click(screen.getByTestId("tasks-empty-blank"));

    expect(mocks.userOpen).toHaveBeenCalledWith({
      app: "tasks",
      route: { pathname: "/tasks/new", search: { mode: "list", template: "blank" } },
    });
  });

  // Open-in-place (BR-4): a task opens through the one item path bound to its own scope, so
  // under All projects it lands on the All projects desktop instead of switching project.
  it("Should open a selected task through the item path bound to its own project", async () => {
    const user = userEvent.setup();
    mocks.page.visibleTasks = [{ id: "task-beta", workspace_id: "ws_beta" }];

    renderCatalog("kanban");
    await user.click(screen.getByTestId("tasks-kanban-view"));

    expect(mocks.openTarget).toHaveBeenCalledWith({
      app: "tasks",
      binding: { kind: "project", workspaceId: "ws_beta" },
      route: { pathname: "/tasks/task-beta", search: {} },
    });
    expect(mocks.userOpen).not.toHaveBeenCalled();
  });

  it("Should pass the resolved default profile into the list surface", () => {
    renderCatalog();

    expect(mocks.listSurfaceProps?.profile).toBe(mocks.page.profile);
  });

  it("Should pass aggregate profile context into the empty state", () => {
    mocks.page.isEmpty = true;
    mocks.page.profile.aggregate = true;
    mocks.page.profile.scopeLabel = null;

    renderCatalog();

    expect(mocks.emptyStateProps).toMatchObject({
      profileScopeLabel: null,
    });
  });
});
