import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Info, Trash2, Zap } from "lucide-react";
import * as React from "react";
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
      cancelButtonProps: { "data-testid": "cancel-action" },
      error: "Delete rejected",
      errorProps: { "data-testid": "confirm-error" },
      note: "Builtin fallback will become effective again.",
      noteProps: { "data-testid": "confirm-note" },
      footNote: (
        <>
          <Info />
          mode: drain
        </>
      ),
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
    await waitFor(() => expect(screen.getByTestId("cancel-action")).toHaveFocus());
    expect(screen.getByTestId("confirm-error")).toHaveAttribute("role", "alert");
    expect(screen.getByTestId("confirm-error")).toHaveTextContent("Delete rejected");
    const note = screen.getByTestId("confirm-note");
    expect(note).toHaveAttribute("role", "note");
    expect(note).toHaveAttribute("data-variant", "info");
    expect(note).toHaveTextContent("Builtin fallback will become effective again.");
    const footer = dialog.querySelector<HTMLElement>('[data-slot="dialog-footer"]');
    const footnote = dialog.querySelector<HTMLElement>('[data-slot="confirm-dialog-footnote"]');
    const actions = dialog.querySelector<HTMLElement>('[data-slot="confirm-dialog-actions"]');
    if (!footer || !footnote || !actions) {
      throw new Error("confirm dialog footer slots were not rendered");
    }
    expect(footer).toContainElement(footnote);
    expect(footer).toContainElement(actions);
    expect(footnote).toHaveTextContent("mode: drain");
    expect(footnote.querySelector("svg")).not.toBeNull();
  });

  it("Should block confirmation until confirmTyping matches exactly", async () => {
    const user = userEvent.setup();
    const onConfirm = vi.fn();
    const consumerRef = React.createRef<HTMLInputElement>();
    renderDialog({
      confirmTyping: "operator-style.md",
      onConfirm,
      confirmInputProps: { "data-testid": "confirm-typing", ref: consumerRef },
      cancelButtonProps: { "data-testid": "cancel-action" },
      confirmButtonProps: { "data-testid": "confirm-action" },
    });

    await waitFor(() => expect(screen.getByTestId("confirm-typing")).toHaveFocus());
    expect(consumerRef.current).toBe(screen.getByTestId("confirm-typing"));
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

  // `children` is the Dialog's trigger slot, so content meant for the dialog
  // body must go through `body` — otherwise it renders outside the surface and
  // silently disappears (the failure this slot exists to prevent).
  it("Should render body content inside the dialog and children as the trigger", async () => {
    const user = userEvent.setup();
    render(
      <UIProvider reducedMotion="always">
        <ConfirmDialog
          body={<p data-testid="confirm-body">Choose what happens to work already in flight.</p>}
          cancelButtonProps={{ "data-testid": "cancel-action" }}
          noteProps={{ "data-testid": "confirm-note" }}
          cancelLabel="Cancel"
          confirmLabel="Pause"
          contentProps={{ "data-testid": "confirm-content" }}
          description="Pause this lane?"
          note="task_03 is running"
          onConfirm={() => undefined}
          title="Pause lane?"
        >
          <DialogTrigger render={<Button variant="outline">Open confirm</Button>} />
        </ConfirmDialog>
      </UIProvider>
    );

    // Before opening, the trigger is visible but the body is not rendered at all.
    expect(screen.queryByTestId("confirm-body")).toBeNull();
    await user.click(screen.getByRole("button", { name: "Open confirm" }));

    const content = await screen.findByTestId("confirm-content");
    const body = screen.getByTestId("confirm-body");
    expect(content).toContainElement(body);
    expect(body).toHaveTextContent("Choose what happens to work already in flight.");
    const order = Array.from(content.querySelectorAll("[data-testid]")).map(node =>
      node.getAttribute("data-testid")
    );
    expect(order).toContain("confirm-note");
    expect(order).toContain("confirm-body");
    expect(order).toContain("cancel-action");
    expect(order.indexOf("confirm-note")).toBeLessThan(order.indexOf("confirm-body"));
    expect(order.indexOf("confirm-body")).toBeLessThan(order.indexOf("cancel-action"));
  });

  it("Should render the head icon well per iconTone and keep the title name clean", async () => {
    renderDialog({
      contentProps: { "data-testid": "confirm-dialog" },
      eyebrow: "Run",
      icon: Zap,
      iconTone: "danger",
      title: "Kill this run?",
      tone: "danger",
    });

    await waitFor(() => expect(screen.getByRole("dialog")).toBeInTheDocument());
    const well = screen
      .getByTestId("confirm-dialog")
      .querySelector('[data-slot="confirm-dialog-icon"]');
    expect(well).toHaveAttribute("data-icon-tone", "danger");
    expect(well?.className).toContain("bg-danger-tint");
    expect(well?.className).toContain("rounded-icon-well");
    expect(well?.querySelector("svg")).not.toBeNull();
    expect(screen.getByRole("heading", { name: "Kill this run?" })).toBeInTheDocument();
    expect(screen.getByText("Run")).toBeInTheDocument();
  });
});
