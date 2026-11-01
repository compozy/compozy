import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { primarySessionFixture } from "../../testing";
import { withSessionProcessSummary } from "../../mocks/process-fixtures";
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

  // Invariant: a delete of a session with processes running answers keep or terminate;
  // without processes it confirms as before, with no answer.
  it("Should ask whether a delete also stops the session's running processes", () => {
    const onConfirm = vi.fn();
    const { rerender } = render(
      <SessionDeleteDialog
        open
        onOpenChange={vi.fn()}
        session={withSessionProcessSummary(primarySessionFixture)}
        isDeleting={false}
        onConfirm={onConfirm}
      />
    );
    const dialog = screen.getByTestId("delete-dialog");
    expect(dialog).toHaveTextContent("This session started 3 processes that are still running.");
    fireEvent.click(within(dialog).getByRole("button", { name: "Keep processes running" }));
    expect(onConfirm).toHaveBeenLastCalledWith("keep");
    fireEvent.click(within(dialog).getByRole("button", { name: "Delete and stop 3 processes" }));
    expect(onConfirm).toHaveBeenLastCalledWith("terminate");

    rerender(
      <SessionDeleteDialog
        open
        onOpenChange={vi.fn()}
        session={primarySessionFixture}
        isDeleting={false}
        onConfirm={onConfirm}
      />
    );
    expect(screen.queryByTestId("delete-dialog-keep-processes")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Delete session" }));
    expect(onConfirm).toHaveBeenLastCalledWith(undefined);
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
    badge: index === 0 ? ("running" as const) : ("stopped" as const),
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

  it("exposes a single bulk failure and retries without changing the singular confirmation", () => {
    const onConfirm = vi.fn();
    const onRetry = vi.fn();
    const onOpenChange = vi.fn();
    render(
      <SessionDeleteDialog
        open
        sessions={[sessions[0]!]}
        results={[{ id: "set-0", status: "failed", error: "Session is locked" }]}
        isDeleting={false}
        onConfirm={onConfirm}
        onRetry={onRetry}
        onOpenChange={onOpenChange}
      />
    );
    expect(screen.getByRole("heading", { name: "Delete session" })).toBeInTheDocument();
    expect(screen.getByRole("alert")).toHaveTextContent("Couldn't delete: Session is locked");
    fireEvent.click(screen.getByTestId("delete-dialog-retry"));
    expect(onRetry).toHaveBeenCalledOnce();
    expect(onConfirm).not.toHaveBeenCalled();
    fireEvent.click(screen.getByTestId("delete-dialog-cancel"));
    expect(onOpenChange).toHaveBeenCalledWith(false);
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

  it("names failed targets beyond the preview and keeps their daemon errors visible", () => {
    render(
      <SessionDeleteDialog
        open
        sessions={sessions}
        results={sessions.map((session, index) =>
          index < 5
            ? { id: session.id, status: "done" }
            : { id: session.id, status: "failed", error: `Locked ${index}` }
        )}
        isDeleting={false}
        onConfirm={vi.fn()}
        onRetry={vi.fn()}
        onOpenChange={vi.fn()}
      />
    );
    expect(screen.getAllByRole("listitem")).toHaveLength(6);
    expect(screen.getAllByRole("alert").map(alert => alert.textContent)).toEqual([
      "Work 5: Couldn't delete: Locked 5",
      "Work 6: Couldn't delete: Locked 6",
    ]);
    expect(screen.getByTestId("delete-dialog-retry")).toHaveTextContent("Retry 2");
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

describe("SessionDeleteDialog child sessions", () => {
  const session = { ...primarySessionFixture, id: "parent", name: "Parent", archived_at: null };

  it("keeps the dialog unchanged without child sessions", () => {
    render(
      <SessionDeleteDialog
        open
        sessions={[session]}
        isDeleting={false}
        onConfirm={vi.fn()}
        onOpenChange={vi.fn()}
      />
    );
    expect(screen.queryByTestId("session-children-include")).not.toBeInTheDocument();
  });

  it("states the count, starts as answered, and reports the answer", () => {
    const onIncludedChange = vi.fn();
    render(
      <SessionDeleteDialog
        open
        sessions={[session]}
        isDeleting={false}
        childSessions={{
          count: 2,
          included: false,
          includedProcessCount: 0,
          pending: false,
          onIncludedChange,
        }}
        onConfirm={vi.fn()}
        onOpenChange={vi.fn()}
      />
    );
    expect(screen.getByText("Also delete 2 child sessions")).toBeInTheDocument();
    const checkbox = screen.getByTestId("session-children-include");
    expect(checkbox).not.toBeChecked();
    fireEvent.click(checkbox);
    expect(onIncludedChange).toHaveBeenCalledWith(true);
  });

  it("holds the confirm while an included tree is still being read", () => {
    render(
      <SessionDeleteDialog
        open
        sessions={[session]}
        isDeleting={false}
        childSessions={{
          count: 1,
          included: true,
          includedProcessCount: 0,
          pending: true,
          onIncludedChange: vi.fn(),
        }}
        onConfirm={vi.fn()}
        onOpenChange={vi.fn()}
      />
    );
    expect(screen.getByTestId("delete-dialog-confirm")).toBeDisabled();
  });

  it("asks about the processes included children left running", () => {
    render(
      <SessionDeleteDialog
        open
        sessions={[session]}
        isDeleting={false}
        childSessions={{
          count: 1,
          included: true,
          includedProcessCount: 2,
          pending: false,
          onIncludedChange: vi.fn(),
        }}
        onConfirm={vi.fn()}
        onOpenChange={vi.fn()}
      />
    );
    expect(screen.getByTestId("delete-dialog-keep-processes")).toBeInTheDocument();
    expect(screen.getByTestId("delete-dialog-confirm")).toHaveTextContent(
      "Delete and stop 2 processes"
    );
  });
});
