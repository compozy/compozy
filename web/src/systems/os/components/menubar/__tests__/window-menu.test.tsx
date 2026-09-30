// Suite: Window menu curation
// Invariant: the Window menu lists the Arrange presets in the shell-rail order
// straight from the registry, and "Move window to" names each other desktop
// while dispatching that desktop's registry slot command; with no other desktop
// it stays in place disabled with its reason.
// Boundary IN: WindowMenu, the registry context, and the real menubar primitive.
// Boundary OUT: desktop runtime state (move targets are injected) and command execution.
// No existing suite owns the Window menu; the item adapter has its own suite.
import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { Menubar, UIProvider } from "@compozy/ui";

import { CmdPaletteRegistryProvider } from "../../../contexts/cmd-palette-registry-context";
import type { WindowMoveTarget } from "../../../hooks/use-window-move-targets";
import {
  paletteRegistryFixture,
  resolvedPaletteCommand,
} from "../../../mocks/cmd-palette-fixtures";
import { WindowMenu } from "../window-menu";

let moveTargets: readonly WindowMoveTarget[] = [];

vi.mock("../../../hooks/use-window-move-targets", () => ({
  useWindowMoveTargets: () => moveTargets,
}));

const REGISTRY = paletteRegistryFixture(
  [
    ["layout.arrange.grid", "Grid"],
    ["layout.balance", "Balance sizes"],
    ["layout.arrange.main-stack", "Main and stack"],
    ["layout.arrange.columns", "Columns"],
    ["window.move_to_desktop.2", "Move window to desktop 2"],
    ["window.move_to_desktop.3", "Move window to desktop 3"],
  ].map(([id, title]) =>
    resolvedPaletteCommand({
      id,
      title,
      action: { kind: "client_op", op: id },
    })
  )
);

function renderMenu(onRun = vi.fn()) {
  render(
    <UIProvider reducedMotion="never" skipAnimations>
      <CmdPaletteRegistryProvider registry={REGISTRY}>
        <Menubar>
          <WindowMenu open onOpenChange={vi.fn()} onRun={onRun} />
        </Menubar>
      </CmdPaletteRegistryProvider>
    </UIProvider>
  );
  return onRun;
}

describe("WindowMenu", () => {
  beforeEach(() => {
    moveTargets = [];
  });

  it("Should list the Arrange presets in the shell-rail order", () => {
    renderMenu();

    const arrange = screen.getByTestId("os-menu-arrange");
    expect(within(arrange).getByText("Arrange")).toBeInTheDocument();
    expect(
      within(arrange)
        .getAllByRole("menuitem")
        .map(item => item.textContent)
    ).toEqual(["Main and stack", "Columns", "Grid", "Balance sizes"]);
  });

  it("Should name each other desktop and run its move slot", async () => {
    const user = userEvent.setup();
    moveTargets = [
      { commandId: "window.move_to_desktop.2", desktopId: "desktop:b", name: "Review" },
      { commandId: "window.move_to_desktop.3", desktopId: "desktop:c", name: "Research" },
    ];
    const onRun = renderMenu();

    // Keyboard path into the submenu: highlight the trigger row, then ArrowRight.
    const trigger = screen.getByTestId("os-menu-move-to-desktop");
    act(() => trigger.focus());
    await user.keyboard("{ArrowRight}");
    const research = await screen.findByTestId("os-menubar-command-window.move_to_desktop.3");
    expect(research).toHaveTextContent("Research");
    expect(screen.getByTestId("os-menubar-command-window.move_to_desktop.2")).toHaveTextContent(
      "Review"
    );

    act(() => research.focus());
    await user.keyboard("{Enter}");
    expect(onRun).toHaveBeenCalledExactlyOnceWith("window.move_to_desktop.3");
  });

  it("Should disable Move window to with its reason when no other desktop exists", () => {
    renderMenu();

    const item = screen.getByTestId("os-menu-move-to-desktop");
    expect(item).toHaveAttribute("aria-disabled", "true");
    expect(item).toHaveTextContent("Move window to");
    expect(item).toHaveTextContent("needs another desktop");
  });
});
