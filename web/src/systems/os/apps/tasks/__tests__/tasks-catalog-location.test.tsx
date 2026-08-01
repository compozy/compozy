// Invariant: catalog route views lead the window toolbar and are the only strip
// content when the selected view has no peer controls.
// Owning layer: Tasks catalog route composition.
// Canonical suite: TasksCatalogLocation component tests.
import { screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { renderWithTopbar } from "@/test/render-with-topbar";

const mocks = vi.hoisted(() => ({
  navigate: vi.fn(),
  userOpen: vi.fn(),
  page: {
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
    scopeError: null,
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
    visibleTasks: [],
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

vi.mock("@/systems/tasks", () => ({
  DEFAULT_TASK_TEMPLATE_ID: "default",
  TasksDashboardView: () => <div data-testid="tasks-dashboard-view" />,
  TasksEmptyState: () => <div data-testid="tasks-empty-state" />,
  TasksInboxView: () => <div data-testid="tasks-inbox-view" />,
  TasksKanbanBoard: () => <div data-testid="tasks-kanban-view" />,
  TasksListSurface: () => <div data-testid="tasks-list-surface" />,
  TasksListToolbar: () => <div data-testid="tasks-list-toolbar" />,
  taskCatalogSearchFor: (mode: string) => ({ mode }),
  taskModeSearchFor: (mode: string) => ({ mode }),
  useTasksPage: () => mocks.page,
  validateTasksSearch: <T,>(search: T) => search,
}));

import { TasksCatalogLocation } from "../tasks-catalog-location";

function renderCatalog(mode?: "dashboard" | "kanban") {
  return renderWithTopbar(<TasksCatalogLocation search={mode ? { mode } : {}} />);
}

describe("TasksCatalogLocation", () => {
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
});
