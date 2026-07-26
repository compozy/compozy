import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Trash2 } from "lucide-react";
import { describe, expect, it, vi } from "vitest";

import { Button } from "../button";
import { UIProvider } from "../custom/ui-provider";
import { ConfirmDialog } from "../custom/confirm-dialog";
import { DialogTrigger } from "../dialog";

function renderDialog(props: Partial<React.ComponentProps<typeof ConfirmDialog>> = {}) {
  const merged: React.ComponentProps<typeof ConfirmDialog> = {
    open: true,
    onOpenChange: vi.fn(),
    title: "Delete entry?",
    description: "This removes the selected entry.",
    confirmLabel: "Delete",
    cancelLabel: "Cancel",
    onConfirm: vi.fn(),
    ...props,
  };
  return render(
    <UIProvider reducedMotion="always">
      <ConfirmDialog {...merged} />
    </UIProvider>
  );
}

describe("ConfirmDialog", () => {
  it("Should render danger tone through the ruled dialog shell", async () => {
    renderDialog({
      contentProps: { "data-testid": "confirm-dialog" },
      confirmButtonProps: { "data-testid": "confirm-action" },
      confirmIcon: Trash2,
    });

    await waitFor(() => expect(screen.getByRole("dialog")).toBeInTheDocument());
    const dialog = screen.getByTestId("confirm-dialog");
    expect(dialog).toHaveAttribute("data-frame", "unframed");
    expect(dialog.querySelector('[data-slot="dialog-header"]')).toHaveAttribute(
      "data-variant",
      "ruled"
    );
    expect(dialog.querySelector('[data-slot="dialog-footer"]')).toHaveAttribute(
      "data-variant",
      "ruled"
    );
    expect(screen.getByTestId("confirm-action").querySelector("svg")).not.toBeNull();
  });

  it("Should block confirmation until confirmTyping matches exactly", async () => {
    const user = userEvent.setup();
    const onConfirm = vi.fn();
    renderDialog({
      confirmTyping: "operator-style.md",
      onConfirm,
      confirmInputProps: { "data-testid": "confirm-typing" },
      confirmButtonProps: { "data-testid": "confirm-action" },
    });

    const button = screen.getByTestId("confirm-action");
    expect(button).toBeDisabled();
    await user.type(screen.getByTestId("confirm-typing"), "operator-style");
    expect(button).toBeDisabled();
    await user.clear(screen.getByTestId("confirm-typing"));
    await user.type(screen.getByTestId("confirm-typing"), "operator-style.md");
    expect(button).toBeEnabled();
    await user.click(button);
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });

  it("Should focus the cancel button by default", async () => {
    renderDialog({
      cancelButtonProps: { "data-testid": "cancel-action" },
    });

    await waitFor(() => expect(screen.getByTestId("cancel-action")).toHaveFocus());
  });

  it("Should render error copy in an alert region", () => {
    renderDialog({
      error: "Delete rejected",
      errorProps: { "data-testid": "confirm-error" },
    });

    expect(screen.getByTestId("confirm-error")).toHaveAttribute("role", "alert");
    expect(screen.getByTestId("confirm-error")).toHaveTextContent("Delete rejected");
  });

  it("Should render note copy with the requested tone", () => {
    renderDialog({
      note: "Builtin fallback will become effective again.",
      noteProps: { "data-testid": "confirm-note" },
    });

    const note = screen.getByTestId("confirm-note");
    expect(note).toHaveAttribute("role", "note");
    expect(note).toHaveAttribute("data-variant", "info");
    expect(note).toHaveTextContent("Builtin fallback will become effective again.");
  });

  it("Should clear typed confirmation after an uncontrolled close and reopen", async () => {
    const user = userEvent.setup();
    render(
      <UIProvider reducedMotion="always">
        <ConfirmDialog
          cancelButtonProps={{ "data-testid": "cancel-action" }}
          cancelLabel="Cancel"
          confirmButtonProps={{ "data-testid": "confirm-action" }}
          confirmInputProps={{ "data-testid": "confirm-typing" }}
          confirmLabel="Delete"
          confirmTyping="operator-style.md"
          description="Confirm the filename before removing this entry."
          onConfirm={() => undefined}
          title="Delete knowledge entry?"
        >
          <DialogTrigger render={<Button variant="outline">Open confirm</Button>} />
        </ConfirmDialog>
      </UIProvider>
    );

    await user.click(screen.getByRole("button", { name: "Open confirm" }));
    await user.type(screen.getByTestId("confirm-typing"), "operator-style.md");
    expect(screen.getByTestId("confirm-action")).toBeEnabled();

    await user.click(screen.getByTestId("cancel-action"));
    await waitFor(() => expect(screen.queryByTestId("confirm-typing")).toBeNull());

    await user.click(screen.getByRole("button", { name: "Open confirm" }));
    expect(await screen.findByTestId("confirm-typing")).toHaveValue("");
    expect(screen.getByTestId("confirm-action")).toBeDisabled();
  });
});
