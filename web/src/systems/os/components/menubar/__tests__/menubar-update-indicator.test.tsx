// Suite: menubar update indicator (UT-049..UT-052)
// Invariant: the indicator exists only while an update is genuinely offered, and
// when it exists it is a keyboard-operable control that navigates to Settings.
// Boundary IN: the indicator's DOM presence, accessible name, and activation.
// Boundary OUT: which snapshots count as "available" — that predicate is
// settingsUpdateIndicatorAvailable, owned by
// systems/settings/lib/__tests__/update-presentation.test.ts. Focus-ring visuals
// are a visual contract (VC-14), verified by screenshot evidence, not asserted here.
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import {
  settingsUpdateApplyingFixture,
  settingsUpdateBlockedFixture,
  settingsUpdateBothAvailableFixture,
  settingsUpdateRolledBackFixture,
  settingsUpdateStagedFixture,
  settingsUpdateStatusFixture,
} from "@/systems/settings/mocks/settings-update-fixture";
import { settingsUpdateIndicatorAvailable } from "@/systems/settings";

import { MenubarUpdateIndicator } from "../menubar-update-indicator";

function renderIndicator(available: boolean) {
  const onActivate = vi.fn();
  render(<MenubarUpdateIndicator available={available} onActivate={onActivate} />);
  return { onActivate };
}

describe("MenubarUpdateIndicator", () => {
  it("Should render an update offer only when the daemon reports one available", () => {
    renderIndicator(settingsUpdateIndicatorAvailable(settingsUpdateBothAvailableFixture));

    expect(screen.getByRole("button", { name: "Update available" })).toBeInTheDocument();
  });

  it("Should stay out of the DOM entirely when nothing is offered", () => {
    renderIndicator(settingsUpdateIndicatorAvailable(settingsUpdateStatusFixture));

    // Absent, not hidden: a CSS-hidden control would still be tab-reachable.
    expect(screen.queryByRole("button", { name: "Update available" })).toBeNull();
  });

  it.each([
    ["applying", settingsUpdateApplyingFixture],
    ["staged", settingsUpdateStagedFixture],
    ["blocked", settingsUpdateBlockedFixture],
    ["failed", settingsUpdateRolledBackFixture],
  ])("Should stay absent while the daemon reports %s", (_state, fixture) => {
    renderIndicator(settingsUpdateIndicatorAvailable(fixture));

    expect(screen.queryByRole("button", { name: "Update available" })).toBeNull();
  });

  it("Should navigate to the Updates section when activated by pointer", async () => {
    const user = userEvent.setup();
    const { onActivate } = renderIndicator(true);

    await user.click(screen.getByRole("button", { name: "Update available" }));

    expect(onActivate).toHaveBeenCalledTimes(1);
  });

  it("Should be reachable by keyboard and activate on Enter and Space", async () => {
    const user = userEvent.setup();
    const { onActivate } = renderIndicator(true);
    const indicator = screen.getByRole("button", { name: "Update available" });

    await user.tab();
    expect(indicator).toHaveFocus();

    await user.keyboard("{Enter}");
    expect(onActivate).toHaveBeenCalledTimes(1);

    await user.keyboard(" ");
    expect(onActivate).toHaveBeenCalledTimes(2);
  });

  it("Should offer no menu, popup, or count alongside the glyph", () => {
    renderIndicator(true);
    const indicator = screen.getByRole("button", { name: "Update available" });

    expect(indicator).not.toHaveAttribute("aria-haspopup");
    expect(indicator).toHaveTextContent("");
  });
});
