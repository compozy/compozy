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
  it("Should lead an approval row with the attention glyph", () => {
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

    // An approval waits on a person, so the row leads with the attention glyph.
    expect(row.querySelector("[data-slot=tasks-inbox-row-glyph]")).toHaveAttribute(
      "data-state",
      "attention"
    );
  });

  it("Should lead a blocked row with the attention glyph", () => {
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
    const glyph = screen
      .getByTestId("tasks-inbox-item-task_block")
      .querySelector("[data-slot=tasks-inbox-row-glyph]");
    expect(glyph).toHaveAttribute("data-state", "attention");
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

  it("Should render activity freshness through Time, never as a raw id or 'now ago'", () => {
    const item = buildInboxItemFixture({
      latest_activity_at: new Date().toISOString(),
      task: {
        id: "task_fresh",
        identifier: "TASK-7",
        scope: "workspace",
        status: "ready",
        title: "Fresh work",
      },
    });

    render(<TasksInboxItem group="updates" item={item} />);

    const row = screen.getByTestId("tasks-inbox-item-task_fresh");
    expect(row).not.toHaveTextContent(/now ago|— ago/);
    expect(row).not.toHaveTextContent("task-7");
    expect(row.querySelector("time")).not.toBeNull();
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
