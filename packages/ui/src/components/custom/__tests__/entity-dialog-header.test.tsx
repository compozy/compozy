import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ClipboardCheck } from "lucide-react";
import { describe, expect, it, vi } from "vitest";

import { DIALOG_TOUCH_TARGET_SQUARE_CLASS } from "../../../lib/dialog-shell";
import { ConfirmDialog } from "../confirm-dialog";
import { Dialog, DialogContent } from "../../dialog";
import { EntityDialogHeader } from "../entity-dialog-header";
import { UIProvider } from "../ui-provider";

function renderHeader(props: Partial<React.ComponentProps<typeof EntityDialogHeader>> = {}) {
  return render(
    <UIProvider reducedMotion="always">
      <Dialog open>
        <DialogContent data-testid="host" showCloseButton={false} unframed>
          <EntityDialogHeader
            eyebrow="Autonomy · Task"
            icon={ClipboardCheck}
            title="Create task"
            {...props}
          />
        </DialogContent>
      </Dialog>
    </UIProvider>
  );
}

describe("EntityDialogHeader", () => {
  it("Should render the accent icon well, eyebrow, and title on ruled chrome", async () => {
    renderHeader();

    await waitFor(() => expect(screen.getByRole("dialog")).toBeInTheDocument());
    const host = screen.getByTestId("host");
    expect(host.querySelector('[data-slot="dialog-header"]')).toHaveAttribute(
      "data-variant",
      "ruled"
    );
    const header = host.querySelector('[data-slot="entity-dialog-header"]');
    expect(header).not.toBeNull();
    expect(screen.getByText("Autonomy · Task")).toBeInTheDocument();
    expect(screen.getByText("Create task")).toBeInTheDocument();

    const well = header?.querySelector('[data-slot="entity-dialog-header-icon"]');
    expect(well?.className).toContain("bg-accent-tint");
    expect(well?.querySelector("svg")).not.toBeNull();
  });

  it("Should omit the description and close button unless supplied", async () => {
    renderHeader();

    await waitFor(() => expect(screen.getByRole("dialog")).toBeInTheDocument());
    const header = screen.getByTestId("host").querySelector('[data-slot="entity-dialog-header"]');
    expect(header?.querySelector('[data-slot="dialog-description"]')).toBeNull();
    expect(screen.queryByRole("button", { name: "Close" })).not.toBeInTheDocument();
  });

  it("Should call onClose from the trailing close control", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    renderHeader({ description: "A task is a durable contract.", onClose });

    await waitFor(() => expect(screen.getByRole("dialog")).toBeInTheDocument());
    expect(screen.getByText("A task is a durable contract.")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Close" }));
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("Should raise the close control to the touch target below 760px", async () => {
    renderHeader({ onClose: vi.fn() });

    await waitFor(() => expect(screen.getByRole("dialog")).toBeInTheDocument());
    const close = screen.getByRole("button", { name: "Close" });
    expect(close).toHaveAttribute("data-slot", "entity-dialog-header-close");
    expect(close).toHaveClass(DIALOG_TOUCH_TARGET_SQUARE_CLASS);
    expect(close).not.toHaveAttribute("tabindex", "-1");
  });

  it("Should leave ConfirmDialog on its neutral header", async () => {
    render(
      <UIProvider reducedMotion="always">
        <ConfirmDialog
          cancelLabel="Cancel"
          confirmLabel="Delete"
          contentProps={{ "data-testid": "confirm" }}
          onConfirm={vi.fn()}
          onOpenChange={vi.fn()}
          open
          title="Delete entry?"
        />
      </UIProvider>
    );

    await waitFor(() => expect(screen.getByRole("dialog")).toBeInTheDocument());
    const confirm = screen.getByTestId("confirm");
    expect(confirm.querySelector('[data-slot="entity-dialog-header"]')).toBeNull();
    expect(confirm.querySelector('[data-slot="entity-dialog-header-icon"]')).toBeNull();
  });
});
