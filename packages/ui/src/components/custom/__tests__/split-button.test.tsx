// Suite: SplitButton behavior
// Invariant: the primary action and its alternatives remain independently keyboard-operable.
// Boundary IN: SplitButton plus the real dropdown primitive.
// Boundary OUT: domain action routing, owned by each consuming system.
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { DropdownMenuItem } from "../../dropdown-menu";
import { SplitButton } from "../split-button";

const MENU_LABEL = "More exit actions";

function renderSplitButton(overrides: Partial<React.ComponentProps<typeof SplitButton>> = {}) {
  const onAction = overrides.onAction ?? vi.fn();
  render(
    <SplitButton label="Commit" menuLabel={MENU_LABEL} onAction={onAction} {...overrides}>
      {overrides.children ?? <DropdownMenuItem>Push</DropdownMenuItem>}
    </SplitButton>
  );
  return {
    onAction,
    action: screen.getByRole("button", { name: "Commit" }),
    trigger: screen.getByRole("button", { name: MENU_LABEL }),
  };
}

describe("SplitButton", () => {
  it("Should fire the primary action on click, Enter, and Space", async () => {
    const user = userEvent.setup();
    const { action, onAction } = renderSplitButton();

    await user.click(action);
    action.focus();
    await user.keyboard("{Enter}");
    await user.keyboard("[Space]");

    expect(onAction).toHaveBeenCalledTimes(3);
  });

  it("Should propagate variant and size to both segments", () => {
    const { action, trigger } = renderSplitButton({ size: "sm", variant: "destructive" });

    for (const segment of [action, trigger]) {
      expect(segment).toHaveAttribute("data-variant", "destructive");
      expect(segment).toHaveAttribute("data-size", "sm");
    }
  });

  it("Should open the menu from the chevron and from ArrowDown on the action", async () => {
    const user = userEvent.setup();
    const { action, trigger } = renderSplitButton();

    await user.click(trigger);
    expect(await screen.findByRole("menuitem", { name: "Push" })).toBeInTheDocument();
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("menuitem", { name: "Push" })).not.toBeInTheDocument();

    action.focus();
    await user.keyboard("{ArrowDown}");
    expect(await screen.findByRole("menuitem", { name: "Push" })).toBeInTheDocument();
  });

  it("Should keep a blocked action inert and announced while the menu stays reachable", async () => {
    const user = userEvent.setup();
    const reason = "Git actions are paused while a bound session is running.";
    const { action, onAction, trigger } = renderSplitButton({
      blocked: true,
      blockedReason: reason,
    });

    await user.click(action);

    expect(onAction).not.toHaveBeenCalled();
    expect(action).toHaveAttribute("aria-disabled", "true");
    expect(action).toHaveAccessibleDescription(reason);
    expect(trigger).not.toBeDisabled();
  });

  it("Should disable both segments while the control is hard-disabled", () => {
    const { action, trigger } = renderSplitButton({ disabled: true });

    expect(action).toBeDisabled();
    expect(trigger).toBeDisabled();
  });

  it("Should render no menu trigger when no alternatives are supplied", () => {
    render(
      <SplitButton label="Commit" menuLabel={MENU_LABEL} onAction={vi.fn()}>
        {null}
      </SplitButton>
    );

    expect(screen.queryByRole("button", { name: MENU_LABEL })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Commit" })).toBeInTheDocument();
  });
});
