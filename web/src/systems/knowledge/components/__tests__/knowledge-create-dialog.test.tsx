import { UIProvider } from "@agh/ui";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { KnowledgeCreateDialog } from "../knowledge-create-dialog";

function renderDialog(props: Partial<React.ComponentProps<typeof KnowledgeCreateDialog>> = {}) {
  const merged: React.ComponentProps<typeof KnowledgeCreateDialog> = {
    open: true,
    onOpenChange: vi.fn(),
    scope: "workspace",
    defaultType: "project",
    isPending: false,
    onConfirm: vi.fn(),
    ...props,
  };
  return render(
    <UIProvider reducedMotion="always">
      <KnowledgeCreateDialog {...merged} />
    </UIProvider>
  );
}

describe("KnowledgeCreateDialog", () => {
  it("Should present the knowledge form and its type choices", () => {
    renderDialog();
    expect(screen.getByRole("heading", { name: "Create knowledge entry" })).toBeInTheDocument();
    expect(screen.getByRole("radiogroup", { name: "Knowledge type" })).toBeInTheDocument();
  });

  it("Should expose a close action", () => {
    renderDialog();
    expect(screen.getByRole("button", { name: "Close" })).toBeInTheDocument();
  });

  it("Should render the available type choices for the Type picker", () => {
    renderDialog();
    const grid = screen.getByTestId("knowledge-create-type-grid");
    expect(grid).toHaveAttribute("role", "radiogroup");
    const cards = within(grid).getAllByRole("radio");
    expect(cards).toHaveLength(4);
    expect(screen.getByTestId("knowledge-create-type-user")).toBeInTheDocument();
    expect(screen.getByTestId("knowledge-create-type-feedback")).toBeInTheDocument();
    expect(screen.getByTestId("knowledge-create-type-project")).toBeInTheDocument();
    expect(screen.getByTestId("knowledge-create-type-reference")).toBeInTheDocument();
  });

  it("Should pre-select the defaultType card", () => {
    renderDialog();
    const projectCard = screen.getByTestId("knowledge-create-type-project");
    expect(projectCard).toHaveAttribute("aria-checked", "true");
    expect(screen.getByTestId("knowledge-create-type-user")).toHaveAttribute(
      "aria-checked",
      "false"
    );
  });

  it("Should disable the confirm button until name and content are present", async () => {
    const user = userEvent.setup();
    renderDialog();

    expect(screen.getByTestId("confirm-create-memory-btn")).toBeDisabled();
    await user.type(screen.getByTestId("knowledge-create-name"), "Launch Memory");
    expect(screen.getByTestId("confirm-create-memory-btn")).toBeDisabled();
    await user.type(screen.getByTestId("knowledge-create-content"), "Use the launch playbook.");
    expect(screen.getByTestId("confirm-create-memory-btn")).toBeEnabled();
  });

  it("Should call onConfirm with the RadioCard-selected type and trimmed input", async () => {
    const user = userEvent.setup();
    const onConfirm = vi.fn().mockResolvedValue(undefined);
    renderDialog({ onConfirm });

    await user.click(screen.getByTestId("knowledge-create-type-reference"));
    expect(screen.getByTestId("knowledge-create-type-reference")).toHaveAttribute(
      "aria-checked",
      "true"
    );
    await user.type(screen.getByTestId("knowledge-create-name"), "  Launch Memory  ");
    await user.type(screen.getByTestId("knowledge-create-description"), "  contract  ");
    await user.type(screen.getByTestId("knowledge-create-content"), "Use the launch playbook.");
    await user.click(screen.getByTestId("confirm-create-memory-btn"));

    expect(onConfirm).toHaveBeenCalledWith({
      type: "reference",
      name: "Launch Memory",
      description: "contract",
      content: "Use the launch playbook.",
    });
  });

  it("Should surface the dialog error", () => {
    renderDialog({ error: "Write rejected" });
    expect(screen.getByTestId("knowledge-create-dialog-error")).toHaveTextContent("Write rejected");
  });

  it("Should call onOpenChange(false) when cancel is clicked", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();
    renderDialog({ onOpenChange });

    await user.click(screen.getByTestId("cancel-create-memory-btn"));

    expect(onOpenChange).toHaveBeenCalledWith(false);
  });
});
