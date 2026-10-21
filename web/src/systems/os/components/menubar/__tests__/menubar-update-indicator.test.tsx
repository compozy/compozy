// Suite: menubar update indicator (UT-049..UT-052)
// Invariant: the control exists only while there is something to say, and its
// one click updates CompozyOS in place — or, when this install cannot update
// itself, opens Settings for the detail. A retryable failure keeps the action here.
// Boundary IN: DOM presence, accessible name, and which callback activation runs.
// Boundary OUT: which snapshots map to which state — settingsUpdateIndicatorState,
// owned by systems/settings/lib/__tests__/update-presentation.test.ts. Pill visuals
// are a visual contract verified by story screenshots, not asserted here.
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { ProductUpdateIndicatorState } from "@/systems/settings";

import { MenubarUpdateIndicator } from "../menubar-update-indicator";

function renderIndicator(state: ProductUpdateIndicatorState, canUpdate = true) {
  const onUpdate = vi.fn();
  const onOpenUpdates = vi.fn();
  render(
    <MenubarUpdateIndicator
      onOpenUpdates={onOpenUpdates}
      onUpdate={canUpdate ? onUpdate : undefined}
      state={state}
    />
  );
  return { onUpdate, onOpenUpdates };
}

describe("MenubarUpdateIndicator", () => {
  it("Should stay out of the DOM entirely when nothing is offered", () => {
    renderIndicator("hidden");

    // Absent, not hidden: a CSS-hidden control would still be tab-reachable.
    expect(screen.queryByTestId("os-menubar-update")).toBeNull();
  });

  it("Should update in place with one click instead of opening Settings", async () => {
    const user = userEvent.setup();
    const { onUpdate, onOpenUpdates } = renderIndicator("available");

    await user.click(screen.getByRole("button", { name: "Update" }));

    expect(onUpdate).toHaveBeenCalledTimes(1);
    expect(onOpenUpdates).not.toHaveBeenCalled();
  });

  it("Should be reachable by keyboard and activate on Enter and Space", async () => {
    const user = userEvent.setup();
    const { onUpdate } = renderIndicator("available");
    const indicator = screen.getByRole("button", { name: "Update" });

    await user.tab();
    expect(indicator).toHaveFocus();

    await user.keyboard("{Enter}");
    expect(onUpdate).toHaveBeenCalledTimes(1);

    await user.keyboard(" ");
    expect(onUpdate).toHaveBeenCalledTimes(2);
  });

  it("Should open Settings when this install cannot update itself", async () => {
    const user = userEvent.setup();
    const { onOpenUpdates } = renderIndicator("available", false);

    await user.click(screen.getByRole("button", { name: "Update" }));

    expect(onOpenUpdates).toHaveBeenCalledTimes(1);
  });

  it("Should hold a disabled spinner while the update runs", () => {
    renderIndicator("updating");

    const control = screen.getByTestId("os-menubar-update");
    expect(control).toBeDisabled();
    expect(screen.getByRole("status")).toHaveTextContent("Updating…");
  });

  it("Should retry a failed update in place when an update action is available", async () => {
    const user = userEvent.setup();
    const { onUpdate, onOpenUpdates } = renderIndicator("failed");

    await user.click(screen.getByRole("button", { name: "Retry update" }));

    expect(onUpdate).toHaveBeenCalledTimes(1);
    expect(onOpenUpdates).not.toHaveBeenCalled();
  });

  it("Should open Settings from a failed update when no retry action is available", async () => {
    const user = userEvent.setup();
    const { onUpdate, onOpenUpdates } = renderIndicator("failed", false);

    await user.click(screen.getByRole("button", { name: "Update failed" }));

    expect(onOpenUpdates).toHaveBeenCalledTimes(1);
    expect(onUpdate).not.toHaveBeenCalled();
  });
});
