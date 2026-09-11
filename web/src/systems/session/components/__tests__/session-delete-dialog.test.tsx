import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { primarySessionFixture } from "../../testing";
import { SessionDeleteDialog } from "../session-delete-dialog";

describe("SessionDeleteDialog", () => {
  it("Should keep the dialog open while deletion is pending", () => {
    const onOpenChange = vi.fn();
    render(
      <SessionDeleteDialog
        open
        onOpenChange={onOpenChange}
        session={primarySessionFixture}
        isDeleting
        onConfirm={vi.fn()}
      />
    );

    expect(screen.getByTestId("delete-dialog-confirm")).toBeDisabled();
    fireEvent.keyDown(document, { key: "Escape" });
    expect(onOpenChange).not.toHaveBeenCalledWith(false);
  });

  it("Should allow normal dismissal before deletion starts", () => {
    const onOpenChange = vi.fn();
    render(
      <SessionDeleteDialog
        open
        onOpenChange={onOpenChange}
        session={primarySessionFixture}
        isDeleting={false}
        onConfirm={vi.fn()}
      />
    );

    fireEvent.keyDown(document, { key: "Escape" });
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });
});

// Invariant: the confirmation names the actual set and blocks dismissal during progress;
// result/retry presentation belongs to this dialog suite, fan-out to the lifecycle suite.
describe("SessionDeleteDialog sets", () => {
  const sessions = Array.from({ length: 7 }, (_, index) => ({
    ...primarySessionFixture,
    id: `set-${index}`,
    name: `Work ${index}`,
    archived_at: null,
    state: index === 0 ? ("active" as const) : ("stopped" as const),
    badge: index === 0 ? "running" : "stopped",
  }));

  it("uses the unchanged singular confirmation for a set of one", () => {
    render(
      <SessionDeleteDialog
        open
        sessions={[sessions[0]!]}
        isDeleting={false}
        onConfirm={vi.fn()}
        onOpenChange={vi.fn()}
      />
    );
    expect(screen.getByRole("heading", { name: "Delete session" })).toBeInTheDocument();
    expect(screen.getByTestId("delete-dialog-confirm")).toHaveTextContent("Delete session");
    expect(screen.queryByTestId("delete-dialog-row-set-0")).not.toBeInTheDocument();
    expect(screen.queryByTestId("delete-dialog-note")).not.toBeInTheDocument();
  });

  it("discloses the count, capped set, overflow, and active membership", () => {
    render(
      <SessionDeleteDialog
        open
        sessions={sessions}
        isDeleting={false}
        onConfirm={vi.fn()}
        onOpenChange={vi.fn()}
      />
    );
    expect(screen.getByRole("heading", { name: "Delete 7 sessions" })).toBeInTheDocument();
    expect(screen.getByText(/including their transcripts and history/)).toHaveTextContent(
      "This permanently removes 7 sessions"
    );
    expect(screen.getAllByRole("listitem")).toHaveLength(6);
    expect(screen.getByText("and 2 more")).toBeInTheDocument();
    expect(screen.queryByTestId("delete-dialog-row-set-5")).not.toBeInTheDocument();
    expect(screen.getByTestId("delete-dialog-note")).toHaveTextContent("1 of them is active.");
  });

  it("shows settled and in-flight results and prevents closing during deletion", () => {
    const onOpenChange = vi.fn();
    render(
      <SessionDeleteDialog
        open
        sessions={sessions.slice(0, 3)}
        results={[
          { id: "set-0", status: "done" },
          { id: "set-1", status: "running" },
          { id: "set-2", status: "pending" },
        ]}
        isDeleting
        onConfirm={vi.fn()}
        onOpenChange={onOpenChange}
      />
    );
    expect(
      within(screen.getByTestId("delete-dialog-row-set-0")).getByLabelText("Deleted")
    ).toBeInTheDocument();
    expect(
      within(screen.getByTestId("delete-dialog-row-set-1")).getByLabelText("Deleting")
    ).toBeInTheDocument();
    expect(screen.getByTestId("delete-dialog-confirm")).toHaveTextContent("Deleting 2 of 3");
    expect(screen.getByTestId("delete-dialog-cancel")).toBeDisabled();
    expect(screen.getByTestId("delete-dialog-note")).toHaveTextContent(
      "Don't close the window while this runs."
    );
    fireEvent.keyDown(document, { key: "Escape" });
    expect(onOpenChange).not.toHaveBeenCalledWith(false);
  });

  it("explains partial failure with the daemon error and exposes retry", () => {
    const onRetry = vi.fn();
    const onOpenChange = vi.fn();
    render(
      <SessionDeleteDialog
        open
        sessions={sessions.slice(0, 3)}
        results={[
          { id: "set-0", status: "done" },
          { id: "set-1", status: "done" },
          { id: "set-2", status: "failed", error: "Session is locked" },
        ]}
        isDeleting={false}
        onConfirm={vi.fn()}
        onRetry={onRetry}
        onOpenChange={onOpenChange}
      />
    );
    expect(screen.getByText(/2 deleted · 1 couldn't be deleted/)).toBeInTheDocument();
    expect(
      screen.getByText(/The one that failed is still in the list and still selected/)
    ).toBeInTheDocument();
    expect(
      within(screen.getByTestId("delete-dialog-row-set-2")).getByText(
        "Couldn't delete: Session is locked"
      )
    ).toBeInTheDocument();
    expect(screen.queryByTestId("delete-dialog-note")).not.toBeInTheDocument();
    fireEvent.click(screen.getByTestId("delete-dialog-retry"));
    expect(onRetry).toHaveBeenCalledOnce();
    fireEvent.click(screen.getByTestId("delete-dialog-cancel"));
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });
});
