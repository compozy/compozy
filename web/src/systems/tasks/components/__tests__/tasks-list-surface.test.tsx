import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { UIProvider } from "@compozy/ui";
import { ownerFromRow } from "@/systems/profiles";
import { aggregateProfileTaskFixture } from "../../mocks/profile-fixtures";

vi.mock("@tanstack/react-router", async importOriginal => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    Link: ({ to, params, search, children, ...props }: Record<string, unknown>) => (
      <a
        href={typeof to === "string" ? to : "#"}
        data-params={JSON.stringify(params)}
        data-search={search === undefined ? undefined : JSON.stringify(search)}
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
type TaskStatus = import("../../types").TaskStatus;

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

const STORY_PROFILE_SCOPE = {
  aggregate: false,
  destination: "default",
  scopeLabel: "default",
  ownerOf: () => ({ id: "00000000000000000000000000", name: "default", archived: false }),
};

function renderSurface(options: RenderOptions = {}) {
  const tasks = options.tasks ?? [];
  return render(
    <UIProvider reducedMotion="never" skipAnimations>
      <TasksListSurface
        profile={STORY_PROFILE_SCOPE}
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

    const groupGlyph = (id: string) => screen.getByTestId(`task-group-dot-${id}`);
    expect(groupGlyph("active")).toHaveAttribute("data-state", "running");
    // Both escalation buckets wait on a person; their labels keep them distinct.
    expect(groupGlyph("blocked")).toHaveAttribute("data-state", "attention");
    expect(groupGlyph("needs_attention")).toHaveAttribute("data-state", "attention");
    expect(groupGlyph("queued")).toHaveAttribute("data-state", "queued");
    expect(groupGlyph("done")).toHaveAttribute("data-state", "done");
    expect(groupGlyph("failed")).toHaveAttribute("data-state", "failed");

    // Every row carries its own exact status beside the glyph.
    const rowStatuses = [...container.querySelectorAll('[data-slot="tasks-list-row-status"]')].map(
      node => node.textContent
    );
    expect(rowStatuses).toEqual([
      "In progress",
      "Blocked",
      "Needs attention",
      "Ready",
      "Completed",
      "Failed",
    ]);
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
          statusCounts={countTasksByStatus([])}
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

  it("Should hold each status count's place while the facets load, then show the numbers", () => {
    const statusCounts = countTasksByStatus([]);
    statusCounts.in_progress = 3;
    const toolbar = (counts: typeof statusCounts, loading: boolean) => (
      <UIProvider reducedMotion="never" skipAnimations>
        <TasksListToolbar
          onOwnerChange={() => {}}
          onPriorityChange={() => {}}
          onRecordsFilterChange={() => {}}
          onSearchQueryChange={() => {}}
          onSortChange={() => {}}
          onStatusChange={() => {}}
          ownerFilter={null}
          ownerOptions={[]}
          priorityFilter={null}
          recordsFilter="work"
          searchQuery=""
          sortBy="recent"
          statusCounts={counts}
          statusCountsLoading={loading}
          statusFilter={null}
        />
      </UIProvider>
    );
    const { rerender } = render(toolbar(countTasksByStatus([]), true));

    // Each pill that will carry a count holds a placeholder for it and no number yet.
    const pills = screen.getByTestId("tasks-quick-status");
    expect(pills.querySelectorAll('[data-slot="skeleton-text"]')).toHaveLength(3);
    expect(within(pills).queryByText(/\d/)).not.toBeInTheDocument();

    rerender(toolbar(statusCounts, false));
    expect(pills.querySelectorAll('[data-slot="skeleton-text"]')).toHaveLength(0);
    expect(
      within(screen.getByTestId("tasks-quick-status-in_progress")).getByText("3")
    ).toBeVisible();
  });

  it("Should write the quick status pills through the single status filter", () => {
    const handleStatusChange = vi.fn();
    const statusCounts = countTasksByStatus([]);
    statusCounts.in_progress = 1;
    statusCounts.needs_attention = 2;
    statusCounts.ready = 3;
    const toolbar = (statusFilter: TaskStatus | null) => (
      <UIProvider reducedMotion="never" skipAnimations>
        <TasksListToolbar
          onOwnerChange={() => {}}
          onPriorityChange={() => {}}
          onRecordsFilterChange={() => {}}
          onSearchQueryChange={() => {}}
          onSortChange={() => {}}
          onStatusChange={handleStatusChange}
          ownerFilter={null}
          ownerOptions={[]}
          priorityFilter={null}
          recordsFilter="work"
          searchQuery=""
          sortBy="recent"
          statusCounts={statusCounts}
          statusFilter={statusFilter}
        />
      </UIProvider>
    );
    const { rerender } = render(toolbar(null));

    expect(screen.getByTestId("tasks-quick-status-all")).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByTestId("tasks-quick-status-all")).toHaveTextContent("All6");
    expect(screen.getByTestId("tasks-quick-status-needs_attention")).toHaveTextContent(
      "Needs attention2"
    );
    fireEvent.click(screen.getByTestId("tasks-quick-status-in_progress"));
    expect(handleStatusChange).toHaveBeenCalledWith("in_progress");

    // Facets are counted after the status filter, so with one selected only
    // that pill keeps a (still exact) count and All returns to the unfiltered list.
    rerender(toolbar("in_progress"));
    expect(screen.getByTestId("tasks-quick-status-in_progress")).toHaveAttribute(
      "aria-pressed",
      "true"
    );
    expect(screen.getByTestId("tasks-quick-status-all")).toHaveTextContent(/^All$/);
    fireEvent.click(screen.getByTestId("tasks-quick-status-all"));
    expect(handleStatusChange).toHaveBeenLastCalledWith(null);

    // A status picked from the Filter menu leaves every quick pill unpressed.
    rerender(toolbar("failed"));
    for (const id of ["all", "in_progress", "needs_attention"]) {
      expect(screen.getByTestId(`tasks-quick-status-${id}`)).toHaveAttribute(
        "aria-pressed",
        "false"
      );
    }
  });

  // One visible representation per status: the pill that expresses it, or the
  // Filter chip when no pill can (another status, or a strip too narrow for pills).
  it("Should show the active status once — as a pill or as a Filter chip, never both", () => {
    const toolbar = (statusFilter: TaskStatus | null) => (
      <UIProvider reducedMotion="never" skipAnimations>
        <TasksListToolbar
          onOwnerChange={() => {}}
          onPriorityChange={() => {}}
          onRecordsFilterChange={() => {}}
          onSearchQueryChange={() => {}}
          onSortChange={() => {}}
          onStatusChange={() => {}}
          ownerFilter={null}
          ownerOptions={[]}
          priorityFilter={null}
          recordsFilter="work"
          searchQuery=""
          sortBy="recent"
          statusCounts={countTasksByStatus([])}
          statusFilter={statusFilter}
        />
      </UIProvider>
    );
    const statusChip = () =>
      within(screen.getByTestId("tasks-list-filters")).queryByText("In progress");

    const { rerender, unmount } = render(toolbar("in_progress"));
    expect(screen.getByTestId("tasks-quick-status-in_progress")).toHaveAttribute(
      "aria-pressed",
      "true"
    );
    expect(statusChip()).toBeNull();

    rerender(toolbar("failed"));
    expect(within(screen.getByTestId("tasks-list-filters")).getByText("Failed")).toBeVisible();
    unmount();

    // A strip too narrow for the pills hands the status back to the Filter chip.
    const measure = vi
      .spyOn(HTMLElement.prototype, "getBoundingClientRect")
      .mockReturnValue(DOMRect.fromRect({ width: 400, height: 38 }));
    try {
      render(toolbar("in_progress"));
      expect(screen.queryByTestId("tasks-quick-status")).toBeNull();
      expect(statusChip()).toBeVisible();
    } finally {
      measure.mockRestore();
    }
  });

  it("Should fold search to a toggle only when the strip is too narrow for a usable field", () => {
    const toolbar = (searchQuery: string) => (
      <UIProvider reducedMotion="never" skipAnimations>
        <TasksListToolbar
          onOwnerChange={() => {}}
          onPriorityChange={() => {}}
          onRecordsFilterChange={() => {}}
          onSearchQueryChange={() => {}}
          onSortChange={() => {}}
          onStatusChange={() => {}}
          ownerFilter={null}
          ownerOptions={[]}
          priorityFilter={null}
          recordsFilter="work"
          searchQuery={searchQuery}
          sortBy="recent"
          statusCounts={countTasksByStatus([])}
          statusFilter={null}
        />
      </UIProvider>
    );
    const measure = vi.spyOn(HTMLElement.prototype, "getBoundingClientRect");
    try {
      // Narrow enough for the icon strip, still wide enough for the field.
      measure.mockReturnValue(DOMRect.fromRect({ width: 400, height: 38 }));
      const wide = render(toolbar(""));
      expect(screen.getByTestId("tasks-list-search-input")).toBeVisible();
      wide.unmount();

      // Below the field's usable floor: a named toggle, never a clipped field.
      measure.mockReturnValue(DOMRect.fromRect({ width: 300, height: 38 }));
      const narrow = render(toolbar(""));
      expect(screen.queryByTestId("tasks-list-search-input")).toBeNull();
      expect(screen.getByRole("button", { name: "Search tasks" })).toBeVisible();
      narrow.unmount();

      // A live query keeps its field on screen even in the narrow strip.
      render(toolbar("deploy"));
      expect(screen.getByTestId("tasks-list-search-input")).toHaveValue("deploy");
    } finally {
      measure.mockRestore();
    }
  });

  // W-23 (surface half): the Project filter renders as one chip with the selected scopes, and
  // removing it clears the filter; without the field (outside All projects) there is no chip.
  it("Should render the Project filter chip only when the field is offered", async () => {
    const { projectFilterField } = await import("@/systems/workspace");
    const setValues = vi.fn();
    const toolbar = (field: ReturnType<typeof projectFilterField> | null) => (
      <UIProvider reducedMotion="never" skipAnimations>
        <TasksListToolbar
          onOwnerChange={() => {}}
          onPriorityChange={() => {}}
          onRecordsFilterChange={() => {}}
          onSearchQueryChange={() => {}}
          onSortChange={() => {}}
          onStatusChange={() => {}}
          ownerFilter={null}
          ownerOptions={[]}
          priorityFilter={null}
          projectFilter={{ field, values: ["ws_alpha", "@user"], setValues }}
          recordsFilter="work"
          searchQuery=""
          sortBy="recent"
          statusCounts={countTasksByStatus([])}
          statusFilter={null}
        />
      </UIProvider>
    );

    const hidden = render(toolbar(null));
    expect(screen.queryByText("Project")).toBeNull();
    hidden.unmount();

    render(toolbar(projectFilterField([{ id: "ws_alpha", name: "Alpha" }])));
    const filters = screen.getByTestId("tasks-list-filters");
    expect(within(filters).getByText("Project")).toBeInTheDocument();
    fireEvent.click(within(filters).getByRole("button", { name: /remove/i }));
    expect(setValues).toHaveBeenCalledWith([]);
  });

  it("Should name the profile the list is scoped to, not the create target", () => {
    renderSurface({ tasks: [] });
    expect(screen.getByTestId("tasks-list-surface-empty")).toHaveTextContent(
      "No tasks in default yet"
    );
  });

  it("Should say the machine is empty rather than name a profile under the aggregate", () => {
    render(
      <UIProvider reducedMotion="never" skipAnimations>
        <TasksListSurface
          profile={{ ...STORY_PROFILE_SCOPE, aggregate: true, scopeLabel: null }}
          errorMessage={null}
          searchQuery=""
          statusCounts={countTasksByStatus([])}
          tasks={[]}
        />
      </UIProvider>
    );
    const empty = screen.getByTestId("tasks-list-surface-empty");
    expect(empty).toHaveTextContent("No tasks in any profile yet");
    expect(empty).not.toHaveTextContent("default");
  });

  it("Should render each task's profile owner under the aggregate", () => {
    const task = aggregateProfileTaskFixture;
    render(
      <UIProvider reducedMotion="never" skipAnimations>
        <TasksListSurface
          profile={{
            aggregate: true,
            scopeLabel: null,
            ownerOf: ownerFromRow,
          }}
          errorMessage={null}
          searchQuery=""
          statusCounts={countTasksByStatus([task])}
          tasks={[task]}
        />
      </UIProvider>
    );

    expect(screen.getByTestId(`task-card-profile-${task.id}`)).toHaveTextContent("marketing");
  });

  // All projects (BR-10, BR-4): a mixed list names each row's project, and the row opens the
  // task in place, bound to that project (or `@user`).
  it("Should badge each row with its project and bind its link under All projects", () => {
    const projectTask = buildTask({ id: "task_beta", workspace_id: "ws_beta" });
    const userTask = buildTask({ id: "task_user", title: "User chore", workspace_id: undefined });
    render(
      <UIProvider reducedMotion="never" skipAnimations>
        <TasksListSurface
          profile={STORY_PROFILE_SCOPE}
          projectBadgeOf={task =>
            task.workspace_id ? { kind: "project", name: "Beta" } : { kind: "user", name: "User" }
          }
          searchQuery=""
          statusCounts={countTasksByStatus([projectTask, userTask])}
          tasks={[projectTask, userTask]}
        />
      </UIProvider>
    );

    expect(screen.getByTestId("task-card-project-task_beta")).toHaveTextContent("Beta");
    expect(screen.getByTestId("task-card-project-task_user")).toHaveTextContent("User");
    expect(screen.getByRole("link", { name: "Open Generate API client" })).toHaveAttribute(
      "data-search",
      JSON.stringify({ workspace: "ws_beta" })
    );
    expect(screen.getByRole("link", { name: "Open User chore" })).toHaveAttribute(
      "data-search",
      JSON.stringify({ workspace: "@user" })
    );
  });

  it("Should leave rows unbadged and unbound while one scope is listed", () => {
    renderSurface({ tasks: [buildTask({ workspace_id: "ws_beta" })] });

    expect(screen.queryByTestId("task-card-project-task_001")).toBeNull();
    expect(screen.getByRole("link", { name: "Open Generate API client" })).not.toHaveAttribute(
      "data-search"
    );
  });

  it("Should offer Clear filters when the filters emptied the list", () => {
    const onClearFilters = vi.fn();
    render(
      <UIProvider reducedMotion="never" skipAnimations>
        <TasksListSurface
          filterState="active"
          onClearFilters={onClearFilters}
          profile={STORY_PROFILE_SCOPE}
          searchQuery=""
          statusCounts={countTasksByStatus([])}
          tasks={[]}
        />
      </UIProvider>
    );

    fireEvent.click(screen.getByTestId("tasks-list-clear-filters"));
    expect(onClearFilters).toHaveBeenCalledTimes(1);
  });

  // UT-041
  it("Should scope the empty message to the reveal filter instead of the generic empty", () => {
    const onShowWorkItems = vi.fn();
    renderSurface({ tasks: [], recordsFilter: "loop", onShowWorkItems });

    const empty = screen.getByTestId("tasks-list-surface-loop-empty");
    expect(empty).toHaveTextContent("No loop steps in this project");
    expect(empty).toHaveTextContent("Switch back to Tasks to see your work.");
    expect(screen.queryByTestId("tasks-list-surface-empty")).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Show tasks" }));
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

  it("Should stand in with the table itself while the first page loads", () => {
    renderSurface({ tasks: [], isLoading: true });

    // One named loading area in the container the rows will fill, not the empty-state page.
    const region = screen.getByRole("status");
    expect(region).toBe(screen.getByTestId("tasks-list-surface-loading"));
    expect(region).toHaveAttribute("aria-busy", "true");
    expect(region).toHaveTextContent("Loading tasks");
    // The header is the real one already, without a total it cannot know yet.
    for (const name of ["Task", "ID", "Status", "Priority", "Owner", "Updated"]) {
      expect(within(region).getByRole("columnheader", { name })).toBeInTheDocument();
    }
    expect(within(region).queryByText(/^\(\d+\)$/)).not.toBeInTheDocument();
    // The rows are placeholders: nothing in them can be opened.
    expect(within(region).queryByRole("link")).not.toBeInTheDocument();
    expect(screen.queryByTestId("tasks-list-surface-empty")).not.toBeInTheDocument();
  });

  it("Should keep the rows when the read reports loading but rows already exist", () => {
    renderSurface({ tasks: [buildTask()], isLoading: true });

    expect(screen.queryByRole("status")).not.toBeInTheDocument();
    expect(screen.getByTestId("task-card-task_001")).toBeInTheDocument();
  });

  it("Should render the error state when errorMessage is set and no tasks", () => {
    renderSurface({ tasks: [], errorMessage: "Unable to reach the daemon" });
    expect(screen.getByTestId("tasks-list-surface-error")).toBeInTheDocument();
    expect(screen.getByText(/unable to reach the daemon/i)).toBeInTheDocument();
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
