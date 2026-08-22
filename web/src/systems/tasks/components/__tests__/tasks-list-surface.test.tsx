import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { UIProvider } from "@compozy/ui";

vi.mock("@tanstack/react-router", async importOriginal => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    Link: ({ to, params, children, ...props }: Record<string, unknown>) => (
      <a
        href={typeof to === "string" ? to : "#"}
        data-params={JSON.stringify(params)}
        {...(props as Record<string, unknown>)}
      >
        {children as React.ReactNode}
      </a>
    ),
  };
});

const { TasksListSurface } = await import("../tasks-list-surface");
const { TasksListToolbar } = await import("../tasks-list-toolbar");
type TaskListItem = import("../../types").TaskListItem;
const { countTasksByStatus } = await import("../../lib/task-formatters");
type TaskRecordsFilter = import("../../types").TaskRecordsFilter;

function buildTask(overrides: Partial<TaskListItem> = {}): TaskListItem {
  return {
    id: "task_001",
    title: "Generate API client",
    identifier: "TASK-1",
    status: "ready",
    scope: "workspace",
    origin: { kind: "web", ref: "op" },
    created_at: "2026-04-11T09:00:00Z",
    updated_at: "2026-04-11T09:00:00Z",
    created_by: { kind: "human", ref: "op" },
    ...overrides,
  } as TaskListItem;
}

interface RenderOptions {
  tasks?: TaskListItem[];
  isLoading?: boolean;
  errorMessage?: string | null;
  statusFilter?: TaskListItem["status"] | null;
  hasMore?: boolean;
  isLoadingMore?: boolean;
  onLoadMore?: () => void;
  onRetryLoad?: () => void;
  searchQuery?: string;
  statusCounts?: ReturnType<typeof countTasksByStatus>;
  recordsFilter?: TaskRecordsFilter;
  onShowWorkItems?: () => void;
  onOpenLoopRun?: () => void;
}

function renderSurface(options: RenderOptions = {}) {
  const tasks = options.tasks ?? [];
  return render(
    <UIProvider reducedMotion="never" skipAnimations>
      <TasksListSurface
        errorMessage={options.errorMessage ?? null}
        isLoading={options.isLoading}
        isLoadingMore={options.isLoadingMore}
        onLoadMore={options.onLoadMore}
        onRetryLoad={options.onRetryLoad}
        onOpenLoopRun={options.onOpenLoopRun}
        searchQuery={options.searchQuery ?? ""}
        filterState={options.statusFilter ? "active" : "inactive"}
        onShowWorkItems={options.onShowWorkItems}
        recordsFilter={options.recordsFilter}
        statusCounts={options.statusCounts ?? countTasksByStatus(tasks)}
        tasks={tasks}
        hasMore={options.hasMore}
      />
    </UIProvider>
  );
}

describe("TasksListSurface", () => {
  it("Should partition tasks into the canonical status group sections", () => {
    const tasks = [
      buildTask({ id: "a", title: "Active task", status: "in_progress" }),
      buildTask({ id: "b", title: "Blocked task", status: "blocked" }),
      buildTask({ id: "n", title: "Escalated task", status: "needs_attention" }),
      buildTask({ id: "c", title: "Queued task", status: "ready" }),
      buildTask({ id: "d", title: "Done task", status: "completed" }),
      buildTask({ id: "e", title: "Failed task", status: "failed" }),
    ];

    const { container } = renderSurface({ tasks });

    expect(screen.getByTestId("task-group-active-label")).toHaveTextContent(/active/i);
    expect(screen.getByTestId("task-group-active-count")).toHaveTextContent("1");
    expect(screen.getByTestId("task-group-blocked")).toBeInTheDocument();
    expect(screen.getByTestId("task-group-needs_attention")).toBeInTheDocument();
    expect(screen.getByTestId("task-group-queued")).toBeInTheDocument();
    expect(screen.getByTestId("task-group-done")).toBeInTheDocument();
    expect(screen.getByTestId("task-group-failed")).toBeInTheDocument();

    expect(screen.getByTestId("task-group-dot-active")).toHaveAttribute("data-tone", "accent");
    expect(screen.getByTestId("task-group-dot-active")).toHaveAttribute("data-variant", "ring");
    expect(screen.getByTestId("task-group-dot-blocked")).toHaveAttribute("data-tone", "danger");
    expect(screen.getByTestId("task-group-dot-needs_attention")).toHaveAttribute(
      "data-tone",
      "warning"
    );
    expect(screen.getByTestId("task-group-dot-queued")).toHaveAttribute("data-tone", "faint");
    expect(screen.getByTestId("task-group-dot-queued")).toHaveAttribute("data-variant", "ring");
    expect(screen.getByTestId("task-group-dot-done")).toHaveAttribute("data-tone", "faint");
    expect(screen.getByTestId("task-group-dot-done")).toHaveAttribute("data-variant", "solid");
    expect(screen.getByTestId("task-group-dot-failed")).toHaveAttribute("data-tone", "danger");

    const rowDots = container.querySelectorAll(
      '[data-slot="tasks-list-row"] [data-slot="status-dot"]'
    );
    expect(rowDots).toHaveLength(0);
  });

  it("Should link each list row to /tasks/$id", () => {
    renderSurface({
      tasks: [buildTask({ id: "task_777", title: "Linked task" })],
    });

    const row = screen.getByTestId("task-card-task_777");
    const link = row.querySelector("a");
    expect(link).not.toBeNull();
    expect(link).toHaveAttribute("href", "/tasks/$id");
    expect(link).toHaveAttribute("data-params", JSON.stringify({ id: "task_777" }));
  });

  it("Should reset the ephemeral reveal before opening a Loop run", () => {
    const onOpenLoopRun = vi.fn();
    renderSurface({
      onOpenLoopRun,
      recordsFilter: "loop",
      tasks: [
        buildTask({
          id: "loop.looprun-001.g1.node.review.0",
          loop: {
            generation: 1,
            loop_name: "review-loop",
            node_id: "review",
            role: "cell",
            run_id: "looprun-001",
          },
        }),
      ],
    });

    fireEvent.click(screen.getByRole("link", { name: /Open run for/ }));
    expect(onOpenLoopRun).toHaveBeenCalledTimes(1);
  });

  it("Should render search and forward list query changes", () => {
    const handleSearchQueryChange = vi.fn();
    const handleRecordsFilterChange = vi.fn();
    render(
      <UIProvider reducedMotion="never" skipAnimations>
        <TasksListToolbar
          onOwnerChange={() => {}}
          onPriorityChange={() => {}}
          onRecordsFilterChange={handleRecordsFilterChange}
          onSearchQueryChange={handleSearchQueryChange}
          onSortChange={() => {}}
          onStatusChange={() => {}}
          ownerFilter={null}
          ownerOptions={[]}
          priorityFilter={null}
          recordsFilter="work"
          searchQuery="api"
          sortBy="recent"
          statusFilter={null}
        />
      </UIProvider>
    );

    const search = screen.getByTestId("tasks-list-search-input");
    expect(search).toHaveValue("api");
    fireEvent.change(search, { target: { value: "deploy" } });
    expect(handleSearchQueryChange).toHaveBeenCalledWith("deploy");
    expect(screen.queryByTestId("tasks-view-nav")).toBeNull();

    // The reveal is off by default and opting in is one explicit act.
    expect(screen.getByTestId("tasks-records-filter-work")).toHaveAttribute("aria-pressed", "true");
    fireEvent.click(screen.getByTestId("tasks-records-filter-loop"));
    expect(handleRecordsFilterChange).toHaveBeenCalledWith("loop");
  });

  it("Should render the empty state when the list is empty", () => {
    renderSurface({ tasks: [] });
    expect(screen.getByTestId("tasks-list-surface-empty")).toBeInTheDocument();
  });

  // UT-041
  it("Should scope the empty message to the reveal filter instead of the generic empty", () => {
    const onShowWorkItems = vi.fn();
    renderSurface({ tasks: [], recordsFilter: "loop", onShowWorkItems });

    const empty = screen.getByTestId("tasks-list-surface-loop-empty");
    expect(empty).toHaveTextContent("No loop records in this workspace");
    expect(empty).toHaveTextContent("Turn the filter back to work items to see your tasks.");
    expect(screen.queryByTestId("tasks-list-surface-empty")).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Show work items" }));
    expect(onShowWorkItems).toHaveBeenCalledTimes(1);
  });

  it("Should keep the narrower filtered-empty message when a filter rides on top of the reveal", () => {
    renderSurface({ tasks: [], recordsFilter: "loop", searchQuery: "missing" });
    expect(screen.queryByTestId("tasks-list-surface-loop-empty")).toBeNull();
    expect(screen.getByTestId("tasks-list-surface-empty")).toHaveTextContent(
      "No tasks match the current filters"
    );
  });

  it("Should describe an empty server search as filtered", () => {
    renderSurface({ searchQuery: "missing", tasks: [] });
    expect(screen.getByTestId("tasks-list-surface-empty")).toHaveTextContent(
      "No tasks match the current filters"
    );
  });

  it("Should render the loading skeleton when isLoading and no tasks", () => {
    renderSurface({ tasks: [], isLoading: true });
    expect(screen.getByTestId("tasks-list-surface-loading")).toBeInTheDocument();
  });

  it("Should render the error state when errorMessage is set and no tasks", () => {
    renderSurface({ tasks: [], errorMessage: "Unable to reach the daemon" });
    expect(screen.getByTestId("tasks-list-surface-error")).toBeInTheDocument();
    expect(screen.getByText(/unable to reach the daemon/i)).toBeInTheDocument();
  });

  it("Should render ready task cards without body identity or toolbar chrome", () => {
    renderSurface({
      tasks: [buildTask({ id: "a", status: "ready" })],
    });

    expect(screen.queryByTestId("tasks-list-page-title")).toBeNull();
    expect(document.querySelector('[data-slot="page-head"]')).toBeNull();
    expect(document.querySelector('[data-slot="listing-toolbar"]')).toBeNull();
    expect(screen.getByTestId("tasks-list-surface")).toBeInTheDocument();
    expect(screen.getByText("Generate API client")).toBeInTheDocument();
  });

  it("Should expose an accessible continuation without hiding loaded rows", () => {
    const onLoadMore = vi.fn();
    renderSurface({
      tasks: [buildTask()],
      hasMore: true,
      onLoadMore,
    });

    fireEvent.click(screen.getByRole("button", { name: "Load more tasks" }));
    expect(onLoadMore).toHaveBeenCalledTimes(1);
    expect(screen.getByTestId("task-card-task_001")).toBeInTheDocument();
  });

  it("Should disable the continuation while the next page is loading", () => {
    renderSurface({
      tasks: [buildTask()],
      hasMore: true,
      isLoadingMore: true,
      onLoadMore: vi.fn(),
    });

    expect(screen.getByRole("button", { name: "Loading more tasks" })).toBeDisabled();
  });

  it("Should retry the failed operation without reusing the load-more callback", () => {
    const onLoadMore = vi.fn();
    const onRetry = vi.fn();
    renderSurface({
      errorMessage: "Next page unavailable",
      tasks: [buildTask()],
      hasMore: true,
      onLoadMore,
      onRetryLoad: onRetry,
    });

    fireEvent.click(screen.getByRole("button", { name: "Retry loading tasks" }));
    expect(onRetry).toHaveBeenCalledTimes(1);
    expect(onLoadMore).not.toHaveBeenCalled();
    expect(screen.getByTestId("task-card-task_001")).toBeInTheDocument();
  });

  it("Should label partial status groups as loaded of the exact facet total", () => {
    const task = buildTask({ status: "ready" });
    const statusCounts = countTasksByStatus([]);
    statusCounts.ready = 10;
    renderSurface({ statusCounts, tasks: [task] });

    expect(screen.getByTestId("task-group-queued-count")).toHaveTextContent("1 of 10");
  });
});
