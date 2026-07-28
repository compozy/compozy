import { fireEvent, render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, ...rest }: { children: ReactNode } & Record<string, unknown>) => {
    const { params: _params, to: _to, ...domRest } = rest as Record<string, unknown>;
    return <a {...domRest}>{children}</a>;
  },
}));

import { TasksInboxView } from "../tasks-inbox-view";
import { buildInboxFixture, buildInboxItemFixture } from "../test-fixtures";

function makeBaseProps() {
  return {
    laneFilter: "all" as const,
    onLaneChange: vi.fn(),
    statusFilter: null,
    onStatusChange: vi.fn(),
    priorityFilter: null,
    onPriorityChange: vi.fn(),
    unreadOnly: false,
    onToggleUnread: vi.fn(),
    searchQuery: "",
    onSearchChange: vi.fn(),
  };
}

describe("TasksInboxView", () => {
  it("Should keep inbox identity out of the window body", () => {
    render(<TasksInboxView {...makeBaseProps()} inbox={buildInboxFixture()} />);

    expect(screen.queryByTestId("tasks-inbox-page-head")).toBeNull();
    expect(screen.queryByRole("heading", { name: "Inbox" })).toBeNull();
    expect(screen.getByTestId("tasks-inbox-body")).toBeInTheDocument();
  });

  it("Should render the toolbar with a filter trigger, search input, and unread switch", () => {
    render(<TasksInboxView {...makeBaseProps()} inbox={buildInboxFixture()} />);

    expect(screen.getByTestId("tasks-inbox-toolbar")).toBeInTheDocument();
    const trigger = screen.getByTestId("tasks-inbox-filter-trigger");
    expect(trigger).toBeInTheDocument();
    expect(trigger).toHaveTextContent(/Filter/);
    expect(screen.getByTestId("tasks-inbox-search")).toBeInTheDocument();
    expect(screen.getByTestId("tasks-inbox-unread-toggle")).toBeInTheDocument();
  });

  it("Should render approval items under the Needs review group with a warning solid dot", () => {
    const inbox = buildInboxFixture({
      page: { has_more: false, limit: 50, total: 1 },
      unread_total: 1,
      groups: [
        {
          lane: "approvals",
          count: 1,
          unread_count: 1,
          items: [
            buildInboxItemFixture({
              lane: "approvals",
              approval_policy: "manual",
              approval_state: "pending",
              task: {
                id: "task_apr",
                identifier: "TASK-33",
                scope: "workspace",
                status: "pending",
                title: "Rotate keys",
              },
              triage: {
                actor: { kind: "human", ref: "op" },
                archived: false,
                dismissed: false,
                read: false,
                task_id: "task_apr",
                updated_at: "2026-04-17T10:00:00Z",
              },
            }),
          ],
        },
      ],
    });

    render(<TasksInboxView {...makeBaseProps()} inbox={inbox} />);

    const group = screen.getByTestId("tasks-inbox-group-needs_review");
    expect(group).toBeInTheDocument();
    const dot = screen.getByTestId("tasks-inbox-group-dot-needs_review");
    expect(dot).toHaveAttribute("data-tone", "warning");
    expect(dot).toHaveAttribute("data-variant", "solid");
  });

  it("Should render blocked items under the Blocked group with a danger solid dot", () => {
    const inbox = buildInboxFixture({
      page: { has_more: false, limit: 50, total: 1 },
      unread_total: 0,
      groups: [
        {
          lane: "blocked",
          count: 1,
          unread_count: 0,
          items: [
            buildInboxItemFixture({
              lane: "blocked",
              blocking_reason: "awaiting deps",
              task: {
                id: "task_block",
                identifier: "TASK-99",
                scope: "workspace",
                status: "blocked",
                title: "Blocked task",
              },
              triage: {
                actor: { kind: "human", ref: "op" },
                archived: false,
                dismissed: false,
                read: true,
                task_id: "task_block",
                updated_at: "2026-04-17T10:00:00Z",
              },
            }),
          ],
        },
      ],
    });

    render(<TasksInboxView {...makeBaseProps()} inbox={inbox} />);

    const group = screen.getByTestId("tasks-inbox-group-blocked");
    expect(group).toBeInTheDocument();
    const dot = screen.getByTestId("tasks-inbox-group-dot-blocked");
    expect(dot).toHaveAttribute("data-tone", "danger");
    expect(dot).toHaveAttribute("data-variant", "solid");
  });

  it("Should emit search and unread toggle changes", () => {
    const props = makeBaseProps();
    const inbox = buildInboxFixture({
      page: { has_more: false, limit: 50, total: 1 },
      groups: [
        {
          lane: "my_work",
          count: 1,
          unread_count: 0,
          items: [buildInboxItemFixture()],
        },
      ],
    });
    render(<TasksInboxView {...props} inbox={inbox} />);

    fireEvent.change(screen.getByTestId("tasks-inbox-search"), { target: { value: "rotate" } });
    expect(props.onSearchChange).toHaveBeenCalledWith("rotate");

    const toggle = screen
      .getByTestId("tasks-inbox-unread-toggle")
      .querySelector("[role=switch]") as HTMLElement;
    fireEvent.click(toggle);
    expect(props.onToggleUnread).toHaveBeenCalledTimes(1);
    expect(props.onToggleUnread.mock.calls[0]?.[0]).toBe(true);
  });

  it("Should render loading, error, and empty states", () => {
    const { rerender } = render(<TasksInboxView {...makeBaseProps()} inbox={null} isLoading />);
    expect(screen.getByTestId("tasks-inbox-loading")).toBeInTheDocument();

    rerender(<TasksInboxView {...makeBaseProps()} errorMessage="oops" inbox={null} />);
    expect(screen.getByTestId("tasks-inbox-error")).toHaveTextContent("oops");

    rerender(<TasksInboxView {...makeBaseProps()} inbox={buildInboxFixture()} />);
    expect(screen.getByTestId("tasks-inbox-empty")).toBeInTheDocument();
  });

  it("Should invoke approval, retry, archive, dismiss, and mark-read actions", () => {
    const handlers = {
      onApprove: vi.fn(),
      onReject: vi.fn(),
      onRetry: vi.fn(),
      onArchive: vi.fn(),
      onDismiss: vi.fn(),
      onMarkRead: vi.fn(),
    };

    const inbox = buildInboxFixture({
      page: { has_more: false, limit: 50, total: 4 },
      unread_total: 3,
      groups: [
        {
          lane: "approvals",
          count: 1,
          unread_count: 1,
          items: [
            buildInboxItemFixture({
              lane: "approvals",
              approval_policy: "manual",
              approval_state: "pending",
              task: {
                id: "task_apr",
                identifier: "TASK-33",
                scope: "workspace",
                status: "pending",
                title: "Rotate keys",
              },
              triage: {
                actor: { kind: "human", ref: "op" },
                archived: false,
                dismissed: false,
                read: false,
                task_id: "task_apr",
                updated_at: "2026-04-17T10:00:00Z",
              },
            }),
          ],
        },
        {
          lane: "failed_runs",
          count: 1,
          unread_count: 1,
          items: [
            buildInboxItemFixture({
              lane: "failed_runs",
              task: {
                id: "task_fail",
                identifier: "TASK-27",
                scope: "workspace",
                status: "failed",
                title: "Sync embeddings",
              },
              run: {
                attempt: 3,
                id: "run_fail",
                max_attempts: 3,
                queued_at: "2026-04-17T09:55:00Z",
                recovery_count: 0,
                status: "failed",
                error: "session timeout",
                task_id: "task_fail",
              },
              triage: {
                actor: { kind: "human", ref: "op" },
                archived: false,
                dismissed: false,
                read: false,
                task_id: "task_fail",
                updated_at: "2026-04-17T10:00:00Z",
              },
            }),
          ],
        },
        {
          lane: "my_work",
          count: 1,
          unread_count: 1,
          items: [
            buildInboxItemFixture({
              task: {
                id: "task_my",
                identifier: "TASK-42",
                scope: "workspace",
                status: "ready",
                title: "Review work",
              },
            }),
          ],
        },
      ],
    });

    render(<TasksInboxView {...makeBaseProps()} {...handlers} inbox={inbox} />);

    fireEvent.click(screen.getByTestId("tasks-inbox-item-approve-task_apr"));
    fireEvent.click(screen.getByTestId("tasks-inbox-item-reject-task_apr"));
    fireEvent.click(screen.getByTestId("tasks-inbox-item-retry-task_fail"));
    fireEvent.click(screen.getByTestId("tasks-inbox-item-dismiss-task_fail"));
    fireEvent.click(screen.getByTestId("tasks-inbox-item-mark-read-task_my"));
    fireEvent.click(screen.getByTestId("tasks-inbox-item-archive-task_my"));

    expect(handlers.onApprove).toHaveBeenCalledWith("task_apr");
    expect(handlers.onReject).toHaveBeenCalledWith("task_apr");
    expect(handlers.onRetry).toHaveBeenCalledWith("run_fail");
    expect(handlers.onDismiss).toHaveBeenCalledWith("task_fail");
    expect(handlers.onMarkRead).toHaveBeenCalledWith("task_my");
    expect(handlers.onArchive).toHaveBeenCalledWith("task_my");
  });

  it("Should expose an accessible continuation and keep loaded groups visible", () => {
    const onLoadMore = vi.fn();
    const inbox = buildInboxFixture({
      page: { has_more: true, limit: 1, next_cursor: "inbox:1", total: 2 },
      groups: [
        {
          lane: "my_work",
          count: 2,
          unread_count: 0,
          items: [buildInboxItemFixture()],
        },
      ],
    });

    render(<TasksInboxView {...makeBaseProps()} hasMore inbox={inbox} onLoadMore={onLoadMore} />);

    fireEvent.click(screen.getByRole("button", { name: "Load more inbox tasks" }));
    expect(onLoadMore).toHaveBeenCalledTimes(1);
    expect(screen.getByTestId("tasks-inbox-groups")).toBeInTheDocument();
  });

  it("Should retry the failed inbox operation without requesting another page", () => {
    const onLoadMore = vi.fn();
    const onRetryQuery = vi.fn();
    const inbox = buildInboxFixture({
      page: { has_more: true, limit: 1, next_cursor: "inbox:1", total: 2 },
      groups: [
        {
          lane: "my_work",
          count: 2,
          unread_count: 0,
          items: [buildInboxItemFixture()],
        },
      ],
    });

    render(
      <TasksInboxView
        {...makeBaseProps()}
        errorMessage="Inbox refresh failed"
        hasMore
        inbox={inbox}
        onLoadMore={onLoadMore}
        onRetryQuery={onRetryQuery}
      />
    );

    fireEvent.click(screen.getByRole("button", { name: "Retry loading inbox" }));
    expect(onRetryQuery).toHaveBeenCalledTimes(1);
    expect(onLoadMore).not.toHaveBeenCalled();
  });

  it("Should disable only the pending run retry target", () => {
    const inbox = buildInboxFixture({
      page: { has_more: false, limit: 50, total: 2 },
      groups: [
        {
          lane: "failed_runs",
          count: 2,
          unread_count: 2,
          items: [
            buildInboxItemFixture({
              lane: "failed_runs",
              task: { id: "task_a", scope: "workspace", status: "failed", title: "Task A" },
              run: {
                attempt: 1,
                id: "run_a",
                max_attempts: 3,
                queued_at: "2026-04-17T09:55:00Z",
                recovery_count: 0,
                status: "failed",
                task_id: "task_a",
              },
            }),
            buildInboxItemFixture({
              lane: "failed_runs",
              task: { id: "task_b", scope: "workspace", status: "failed", title: "Task B" },
              run: {
                attempt: 1,
                id: "run_b",
                max_attempts: 3,
                queued_at: "2026-04-17T09:56:00Z",
                recovery_count: 0,
                status: "failed",
                task_id: "task_b",
              },
            }),
          ],
        },
      ],
    });

    render(
      <TasksInboxView
        {...makeBaseProps()}
        inbox={inbox}
        onRetry={vi.fn()}
        pendingRetryIds={new Set(["run_a"])}
      />
    );

    expect(screen.getByTestId("tasks-inbox-item-retry-task_a")).toBeDisabled();
    expect(screen.getByTestId("tasks-inbox-item-retry-task_b")).toBeEnabled();
  });

  it("Should label partial inbox groups as loaded of the exact lane total", () => {
    const inbox = buildInboxFixture({
      page: { has_more: true, limit: 1, next_cursor: "inbox:1", total: 10 },
      groups: [
        {
          lane: "approvals",
          count: 10,
          unread_count: 4,
          items: [buildInboxItemFixture({ lane: "approvals" })],
        },
      ],
    });

    render(<TasksInboxView {...makeBaseProps()} inbox={inbox} />);
    expect(screen.getByTestId("tasks-inbox-group-count-needs_review")).toHaveTextContent("1 of 10");
  });
});
