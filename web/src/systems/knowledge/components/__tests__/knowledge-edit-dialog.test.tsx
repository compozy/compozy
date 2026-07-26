import { dialogShellClass, UIProvider } from "@agh/ui";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { KnowledgeEditDialog } from "../knowledge-edit-dialog";

function renderDialog(props: Partial<React.ComponentProps<typeof KnowledgeEditDialog>> = {}) {
  const merged: React.ComponentProps<typeof KnowledgeEditDialog> = {
    open: true,
    onOpenChange: vi.fn(),
    filename: "user.md",
    name: "operator-style",
    type: "user",
    scope: "global",
    initialContent: "# Initial content",
    initialDescription: "initial description",
    isPending: false,
    onConfirm: vi.fn(),
    ...props,
  };
  return render(
    <UIProvider reducedMotion="always">
      <KnowledgeEditDialog {...merged} />
    </UIProvider>
  );
}

describe("KnowledgeEditDialog", () => {
  it("Should render the initial content and description", () => {
    renderDialog();
    expect(screen.getByTestId("knowledge-edit-content")).toHaveValue("# Initial content");
    expect(screen.getByTestId("knowledge-edit-description")).toHaveValue("initial description");
    expect(screen.getByRole("dialog")).toHaveAttribute("data-frame", "unframed");
    expect(screen.getByText("Description").closest('[data-slot="field"]')).not.toBeNull();
    expect(screen.getByText("Content").closest('[data-slot="field"]')).not.toBeNull();
    expect(screen.getByRole("dialog").querySelector('[data-slot="dialog-header"]')).toHaveAttribute(
      "data-variant",
      "ruled"
    );
    expect(screen.getByRole("dialog").querySelector('[data-slot="dialog-footer"]')).toHaveAttribute(
      "data-variant",
      "ruled"
    );
  });

  it("Should host the editor on the shared sm modal token", () => {
    renderDialog();
    const host = screen.getByTestId("knowledge-edit-dialog");
    for (const token of dialogShellClass("sm").split(" ")) {
      expect(host.className).toContain(token);
    }
    expect(host.className).not.toMatch(/max-w-2xl/);
  });

  it("Should lock name and type behind ImmutableIdentity instead of an editable control", () => {
    renderDialog();
    const identity = screen.getByTestId("knowledge-edit-identity");
    expect(within(identity).getByText("operator-style")).toBeInTheDocument();
    expect(within(identity).getByText("user")).toBeInTheDocument();
    // A disabled input is forbidden as a data display, so neither field may exist
    // as a form control on the edit surface at all.
    expect(screen.queryByLabelText("Name")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Type")).not.toBeInTheDocument();
  });

  it("Should disable the confirm button until content changes", () => {
    renderDialog();
    expect(screen.getByTestId("confirm-edit-memory-btn")).toBeDisabled();
  });

  it("Should enable the confirm button for a description-only edit", async () => {
    const user = userEvent.setup();
    renderDialog();
    await user.type(screen.getByTestId("knowledge-edit-description"), " revised");
    expect(screen.getByTestId("confirm-edit-memory-btn")).toBeEnabled();
  });

  it("Should call onConfirm with the edited content and trimmed description", async () => {
    const user = userEvent.setup();
    const onConfirm = vi.fn().mockResolvedValue(undefined);
    renderDialog({ onConfirm });

    await user.type(screen.getByTestId("knowledge-edit-content"), " more body");
    await user.click(screen.getByTestId("confirm-edit-memory-btn"));

    expect(onConfirm).toHaveBeenCalledWith({
      content: "# Initial content more body",
      description: "initial description",
    });
  });

  it("Should send undefined description when the field is cleared", async () => {
    const user = userEvent.setup();
    const onConfirm = vi.fn().mockResolvedValue(undefined);
    renderDialog({ onConfirm });

    await user.clear(screen.getByTestId("knowledge-edit-description"));
    await user.type(screen.getByTestId("knowledge-edit-content"), " more");
    await user.click(screen.getByTestId("confirm-edit-memory-btn"));

    expect(onConfirm).toHaveBeenCalledWith({
      content: "# Initial content more",
      description: undefined,
    });
  });

  it("Should disable the confirm button while a save is pending", async () => {
    const user = userEvent.setup();
    renderDialog({ isPending: true });
    await user.type(screen.getByTestId("knowledge-edit-content"), " edit");
    expect(screen.getByTestId("confirm-edit-memory-btn")).toBeDisabled();
  });

  it("Should surface the dialog error message inside the dialog", () => {
    renderDialog({ error: "Edit rejected" });
    expect(screen.getByTestId("knowledge-edit-dialog-error")).toHaveTextContent("Edit rejected");
  });

  it("Should call onOpenChange(false) when cancel is clicked", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();
    renderDialog({ onOpenChange });
    await user.click(screen.getByTestId("cancel-edit-memory-btn"));
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });
});
