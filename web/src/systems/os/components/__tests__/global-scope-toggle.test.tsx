// Suite: menubar global-scope toggle
// Invariant: the one scope control — a locked globe toggle never fires a change, the flip
// announces through the polite live region only on state change (never on mount), and the
// locked reason composes into the accessible name. Pressed state is aria-pressed.
// Boundary IN: the presentational toggle. Boundary OUT: store wiring (desktop shell suites).

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { UIProvider } from "@compozy/ui";

import { GlobalScopeToggle } from "../global-scope-toggle";

function renderToggle(props: Partial<React.ComponentProps<typeof GlobalScopeToggle>> = {}) {
  const onCheckedChange = vi.fn();
  const view = render(
    <UIProvider reducedMotion="always">
      <GlobalScopeToggle
        checked={false}
        tooltip="Global scope ~"
        onCheckedChange={onCheckedChange}
        {...props}
      />
    </UIProvider>
  );
  return { ...view, onCheckedChange };
}

function rerenderToggle(
  view: ReturnType<typeof renderToggle>,
  props: Partial<React.ComponentProps<typeof GlobalScopeToggle>>
) {
  view.rerender(
    <UIProvider reducedMotion="always">
      <GlobalScopeToggle
        checked={false}
        tooltip="Global scope ~"
        onCheckedChange={view.onCheckedChange}
        {...props}
      />
    </UIProvider>
  );
}

describe("GlobalScopeToggle", () => {
  it("Should flip through click and report the next state", async () => {
    const user = userEvent.setup();
    const { onCheckedChange } = renderToggle();

    const control = screen.getByRole("button", { name: "Global scope" });
    expect(control).toHaveAttribute("aria-pressed", "false");
    await user.click(control);

    expect(onCheckedChange).toHaveBeenCalledWith(true);
  });

  it("Should stay inert while locked and expose the reason in its name", async () => {
    const user = userEvent.setup();
    const { onCheckedChange } = renderToggle({
      checked: true,
      locked: true,
      lockedReason: "Add a workspace to scope down",
    });

    const control = screen.getByRole("button", {
      name: "Global scope — Add a workspace to scope down",
    });
    expect(control).toHaveAttribute("aria-pressed", "true");
    expect(control).toHaveAttribute("aria-disabled", "true");

    await user.click(control);
    expect(onCheckedChange).not.toHaveBeenCalled();
  });

  it("Should announce flips through the live region but never on mount", () => {
    const view = renderToggle({ checked: false });
    const liveRegion = () => document.querySelector('[aria-live="polite"]');

    expect(liveRegion()).toHaveTextContent("");

    rerenderToggle(view, { checked: true });
    expect(liveRegion()).toHaveTextContent("Global scope on");

    rerenderToggle(view, { checked: false });
    expect(liveRegion()).toHaveTextContent("Global scope off");
  });
});
