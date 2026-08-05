import { fireEvent, render, screen } from "@testing-library/react";
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
