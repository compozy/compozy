// Invariant (#1717): Clean up history deletes only what its count says. The destructive action
// carries the dry run's total, stays off with nothing to delete and while the count for new
// choices is in flight, past 100 sessions waits for the person to type that total, keeps
// Everything's parts checked and locked, and while the daemon deletes it only offers Stop; a run
// that kept sessions offers to retry them.
// Owning layer: the dialog component (one suite per dialog).
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { ARCHIVED_CLEANUP_FIXTURES } from "../../mocks/archived-cleanup-fixtures";
import { DEFAULT_CLEANUP_OPTIONS, EMPTY_CLEANUP_IMPACT } from "../../lib/archived-cleanup-model";
import {
  ArchivedCleanupDialog,
  type ArchivedCleanupDialogProps,
} from "../session-list/archived-cleanup-dialog";

function dialogProps(overrides: Partial<ArchivedCleanupDialogProps> = {}) {
  const fixture = ARCHIVED_CLEANUP_FIXTURES.dialog;
  return {
    open: true,
    onOpenChange: vi.fn(),
    scopeName: fixture.scopeName,
    options: fixture.options,
    onOptionsChange: vi.fn(),
    impact: { ...fixture.impact, sessions: 20, children: 30, subagents: 22, total: 72 },
    counts: fixture.counts,
    phase: { kind: "form" },
    onConfirm: vi.fn(),
    onRetry: vi.fn(),
    onStop: vi.fn(),
    ...overrides,
  } satisfies ArchivedCleanupDialogProps;
}

function renderDialog(overrides: Partial<ArchivedCleanupDialogProps> = {}) {
  const props = dialogProps(overrides);
  return { props, ...render(<ArchivedCleanupDialog {...props} />) };
}

describe("ArchivedCleanupDialog", () => {
  it("Should confirm with the action that names the dry run's total", () => {
    const { props } = renderDialog();
    expect(screen.getByText("72 sessions will be deleted")).toBeInTheDocument();
    const confirm = screen.getByTestId("archived-cleanup-confirm");
    expect(confirm).toHaveTextContent("Delete 72 sessions");
    fireEvent.click(confirm);
    expect(props.onConfirm).toHaveBeenCalledOnce();
  });

  it("Should keep the action off when nothing matches", () => {
    const { props } = renderDialog({ impact: EMPTY_CLEANUP_IMPACT });
    expect(screen.getByText("Nothing to delete")).toBeInTheDocument();
    const confirm = screen.getByTestId("archived-cleanup-confirm");
    expect(confirm).toBeDisabled();
    fireEvent.click(confirm);
    expect(props.onConfirm).not.toHaveBeenCalled();
  });

  // A switch from a small range to a large one must not confirm the small count shown meanwhile.
  it("Should keep the action off while the count for new choices is in flight", () => {
    const { props } = renderDialog({ counting: true });
    expect(screen.getByText("72 sessions will be deleted")).toBeInTheDocument();
    const confirm = screen.getByTestId("archived-cleanup-confirm");
    expect(confirm).toBeDisabled();
    fireEvent.click(confirm);
    expect(props.onConfirm).not.toHaveBeenCalled();
  });

  it("Should keep the action off while the count loads or fails", () => {
    const { rerender } = renderDialog({ impact: undefined });
    expect(screen.getByText("Counting archived sessions…")).toBeInTheDocument();
    expect(screen.getByTestId("archived-cleanup-confirm")).toBeDisabled();
    rerender(<ArchivedCleanupDialog {...dialogProps({ previewError: "database is locked" })} />);
    expect(screen.getByText("Couldn’t count archived sessions")).toBeInTheDocument();
    expect(screen.getByTestId("archived-cleanup-confirm")).toBeDisabled();
  });

  it("Should wait for the typed total past 100 sessions, separators optional", () => {
    const fixture = ARCHIVED_CLEANUP_FIXTURES.scale;
    const { props } = renderDialog({ impact: fixture.impact, acrossProjects: true });
    const confirm = screen.getByTestId("archived-cleanup-confirm");
    expect(confirm).toHaveTextContent(`Delete ${(1204).toLocaleString()} sessions`);
    expect(confirm).toBeDisabled();
    const typed = screen.getByTestId("archived-cleanup-typed");
    fireEvent.change(typed, { target: { value: "1203" } });
    expect(confirm).toBeDisabled();
    fireEvent.change(typed, { target: { value: "1,204" } });
    expect(confirm).toBeEnabled();
    fireEvent.click(confirm);
    expect(props.onConfirm).toHaveBeenCalledOnce();
  });

  it("Should keep Child sessions and Subagents checked and locked under Everything", () => {
    const { props } = renderDialog();
    for (const id of ["archived-cleanup-what-children", "archived-cleanup-what-subagents"]) {
      const check = screen.getByTestId(id);
      expect(check).toBeChecked();
      expect(check).toHaveAttribute("data-disabled");
    }
    fireEvent.click(screen.getByTestId("archived-cleanup-what-everything"));
    expect(props.onOptionsChange).toHaveBeenCalledWith({
      ...DEFAULT_CLEANUP_OPTIONS,
      what: { everything: false, children: true, subagents: true },
    });
  });

  it("Should offer only Stop and Close while the daemon deletes", () => {
    const { props } = renderDialog({ phase: { kind: "deleting", done: 64, total: 158 } });
    expect(screen.getByTestId("archived-cleanup-progress")).toHaveTextContent("64 of 158");
    expect(screen.queryByTestId("archived-cleanup-confirm")).not.toBeInTheDocument();
    fireEvent.click(screen.getByTestId("archived-cleanup-stop"));
    expect(props.onStop).toHaveBeenCalledOnce();
  });

  it("Should list the threads a partial run kept and retry them", () => {
    const { props } = renderDialog({ phase: ARCHIVED_CLEANUP_FIXTURES.partial.phase });
    expect(screen.getByText("Some sessions weren’t deleted")).toBeInTheDocument();
    const failures = screen.getByTestId("archived-cleanup-failures");
    expect(failures).toHaveTextContent("Fix #1704 cap reasoning effort levels");
    expect(failures).toHaveTextContent("It is an always-on agent's main session.");
    const retry = screen.getByTestId("archived-cleanup-retry");
    expect(retry).toHaveTextContent("Try 2 again");
    fireEvent.click(retry);
    expect(props.onRetry).toHaveBeenCalledOnce();
  });
});
