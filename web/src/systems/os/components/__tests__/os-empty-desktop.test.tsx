// Suite: empty desktop card
// Invariant: an empty desktop names itself and offers New session only when a project can take it;
// in Global the action stays visible, disabled, and says why through a focusable wrapper.
// Owning layer: OsEmptyDesktop (and the shared OsNewSessionButton it composes).
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { TooltipProvider } from "@compozy/ui";

import { OsEmptyDesktop } from "../os-empty-desktop";

function renderCard(hasProject: boolean) {
  const onNewSession = vi.fn();
  render(
    <TooltipProvider delay={0}>
      <OsEmptyDesktop
        desktopName="Desktop 2"
        paletteShortcutLabel="⌘K"
        hasProject={hasProject}
        onNewSession={onNewSession}
      />
    </TooltipProvider>
  );
  return onNewSession;
}

describe("OsEmptyDesktop", () => {
  it("Should name the empty desktop and start a session when a project is active", async () => {
    const user = userEvent.setup();
    const onNewSession = renderCard(true);

    expect(screen.getByRole("heading", { name: "Desktop 2 is empty" })).toBeInTheDocument();
    expect(screen.getByTestId("os-desk-hint")).toHaveTextContent("⌘K");
    await user.click(screen.getByRole("button", { name: "New session" }));
    expect(onNewSession).toHaveBeenCalledOnce();
  });

  it("Should keep New session disabled in Global scope and explain why", async () => {
    const user = userEvent.setup();
    const onNewSession = renderCard(false);

    expect(screen.getByRole("button", { name: /New session/ })).toBeDisabled();
    const reason = screen.getByTestId("os-desk-new-session-disabled");
    expect(reason).toHaveAccessibleName("New session — pick a project to start a session");

    await user.tab();
    expect(reason).toHaveFocus();
    await waitFor(() =>
      expect(screen.getByText("Pick a project to start a session")).toBeInTheDocument()
    );
    expect(onNewSession).not.toHaveBeenCalled();
  });
});
