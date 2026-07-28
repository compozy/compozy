import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { TaskPauseDialog } from "../task-pause-dialog";

function buildDialog() {
  return {
    close: vi.fn(),
    confirm: vi.fn().mockResolvedValue(undefined),
    error: "Provide a pause reason.",
    isOpen: true,
    onOpenChange: vi.fn(),
    onReasonChange: vi.fn(),
    open: vi.fn(),
    reason: "",
  };
}

describe("TaskPauseDialog", () => {
  it("Should associate validation feedback with the reason field", () => {
    const dialog = buildDialog();
    render(<TaskPauseDialog dialog={dialog} />);

    expect(screen.getByTestId("tasks-detail-pause-reason")).toHaveAttribute(
      "aria-describedby",
      "tasks-detail-pause-error"
    );
    expect(screen.getByTestId("tasks-detail-pause-reason")).toHaveAttribute("aria-invalid", "true");
  });

  it("Should block Escape dismissal and expose progress while pause is pending", () => {
    const dialog = buildDialog();
    render(<TaskPauseDialog dialog={dialog} isPending />);

    const confirm = screen.getByTestId("tasks-detail-pause-confirm");
    expect(confirm).toBeDisabled();
    expect(confirm).toHaveAttribute("aria-busy", "true");
    expect(confirm).toHaveTextContent("Pausing…");

    fireEvent.keyDown(document, { key: "Escape" });
    expect(dialog.onOpenChange).not.toHaveBeenCalledWith(false);
  });
});
