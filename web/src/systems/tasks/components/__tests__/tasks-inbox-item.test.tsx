import { fireEvent, render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, ...rest }: { children: ReactNode } & Record<string, unknown>) => {
    const { params: _params, to: _to, ...domRest } = rest as Record<string, unknown>;
    return <a {...domRest}>{children}</a>;
  },
}));

import { TasksInboxItem } from "../tasks-inbox-item";
import { buildInboxItemFixture } from "../test-fixtures";

describe("TasksInboxItem", () => {
  it("Should render a 3-col grid (rail / body / meta) with the rail painted by the group tone, not a left border", () => {
    const item = buildInboxItemFixture({
      lane: "approvals",
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
    });

    render(<TasksInboxItem group="needs_review" item={item} />);

    const row = screen.getByTestId("tasks-inbox-item-task_apr");
    expect(row).toHaveAttribute("data-group", "needs_review");

    const rail = row.querySelector("[data-slot=tasks-inbox-row-rail]");
    expect(rail).not.toBeNull();
  });

  it("Should paint the rail with the blocked danger tone when the row belongs to the blocked group", () => {
    const item = buildInboxItemFixture({
      lane: "blocked",
      task: {
        id: "task_block",
        identifier: "TASK-99",
        scope: "workspace",
        status: "blocked",
        title: "Blocked task",
      },
    });

    render(<TasksInboxItem group="blocked" item={item} />);
    const rail = screen
      .getByTestId("tasks-inbox-item-task_block")
      .querySelector("[data-slot=tasks-inbox-row-rail]");
    expect(rail).not.toBeNull();
  });

  it("Should render Reject as a ghost-danger button and Approve as the single accent CTA", () => {
    const onApprove = vi.fn();
    const onReject = vi.fn();
    const item = buildInboxItemFixture({
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
    });

    render(
      <TasksInboxItem group="needs_review" item={item} onApprove={onApprove} onReject={onReject} />
    );

    const actions = screen.getByTestId("tasks-inbox-item-actions-task_apr");
    const buttons = actions.querySelectorAll("[data-slot=button]");
    expect(buttons).toHaveLength(3);

    expect(screen.getByTestId("tasks-inbox-item-reject-task_apr")).toHaveAttribute(
      "data-variant",
      "destructive-ghost"
    );
    expect(screen.getByTestId("tasks-inbox-item-approve-task_apr")).toHaveAttribute(
      "data-variant",
      "primary"
    );
    expect(screen.getByTestId("tasks-inbox-item-open-task_apr")).toBeInTheDocument();

    fireEvent.click(screen.getByTestId("tasks-inbox-item-approve-task_apr"));
    expect(onApprove).toHaveBeenCalledTimes(1);
    expect(onApprove).toHaveBeenCalledWith("task_apr");
  });

  it("Should keep inline actions enabled when the row itself has no open handler", () => {
    const item = buildInboxItemFixture({
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
    });

    render(<TasksInboxItem group="needs_review" item={item} onApprove={vi.fn()} />);

    const row = screen.getByTestId("tasks-inbox-item-task_apr");
    expect(row).not.toHaveAttribute("role", "button");
    expect(row).not.toHaveAttribute("aria-disabled");
    expect(screen.getByTestId("tasks-inbox-item-approve-task_apr")).toBeEnabled();
  });

  it("Should not invoke row selection when the Reject button is clicked", () => {
    const onOpen = vi.fn();
    const onReject = vi.fn();
    const item = buildInboxItemFixture({
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
    });

    render(
      <TasksInboxItem
        group="needs_review"
        item={item}
        onApprove={vi.fn()}
        onOpen={onOpen}
        onReject={onReject}
      />
    );

    fireEvent.click(screen.getByTestId("tasks-inbox-item-reject-task_apr"));

    expect(onReject).toHaveBeenCalledWith("task_apr");
    expect(onOpen).not.toHaveBeenCalled();
  });
});
