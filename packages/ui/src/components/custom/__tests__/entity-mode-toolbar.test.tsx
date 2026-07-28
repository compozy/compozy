import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { DIALOG_TOUCH_TARGET_SEGMENTS_CLASS } from "../../../lib/dialog-shell";
import { EntityModeToolbar } from "../entity-mode-toolbar";

function renderToolbar(props: Partial<React.ComponentProps<typeof EntityModeToolbar>> = {}) {
  const onModeChange = vi.fn();
  const view = render(<EntityModeToolbar mode="simple" onModeChange={onModeChange} {...props} />);
  return { ...view, onModeChange };
}

describe("EntityModeToolbar", () => {
  it("Should expose the mode segments as a pressed group", () => {
    renderToolbar();

    expect(screen.getByRole("group", { name: "Editor mode" })).toBeInTheDocument();
    expect(screen.getByTestId("entity-mode-simple")).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByTestId("entity-mode-advanced")).toHaveAttribute("aria-pressed", "false");
  });

  it("Should report the selected mode without owning it", async () => {
    const user = userEvent.setup();
    const { onModeChange } = renderToolbar();

    await user.click(screen.getByTestId("entity-mode-advanced"));
    expect(onModeChange).toHaveBeenCalledWith("advanced");
    // Controlled: the pressed state only moves when the consumer re-renders.
    expect(screen.getByTestId("entity-mode-simple")).toHaveAttribute("aria-pressed", "true");
  });

  it("Should prefix its test ids per consumer", () => {
    renderToolbar({ testIdPrefix: "task" });

    expect(screen.getByTestId("task-mode-simple")).toBeInTheDocument();
    expect(screen.queryByTestId("entity-mode-simple")).not.toBeInTheDocument();
  });

  it("Should omit the trailing slot when the entity carries no scope control", () => {
    const { container } = renderToolbar({ trailingLabel: "Scope" });

    expect(container.querySelector('[data-slot="entity-mode-toolbar"]')).toHaveAttribute(
      "data-mode",
      "simple"
    );
    expect(screen.queryByText("Scope")).not.toBeInTheDocument();
  });

  it("Should raise the mode segments to the touch target below 760px", () => {
    renderToolbar();

    expect(screen.getByRole("group", { name: "Editor mode" })).toHaveClass(
      DIALOG_TOUCH_TARGET_SEGMENTS_CLASS
    );
  });

  it("Should keep both mode segments reachable by keyboard", async () => {
    const user = userEvent.setup();
    renderToolbar();

    screen.getByTestId("entity-mode-simple").focus();
    expect(screen.getByTestId("entity-mode-simple")).toHaveFocus();
    await user.tab();
    expect(screen.getByTestId("entity-mode-advanced")).toHaveFocus();
  });

  it("Should render the trailing slot with its label", () => {
    renderToolbar({
      trailing: <button type="button">Workspace</button>,
      trailingLabel: "Scope",
    });

    expect(screen.getByText("Scope")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Workspace" })).toBeInTheDocument();
  });
});
