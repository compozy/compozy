// Suite: Window menu curation
// Invariant: the Window menu lists the Arrange presets in the shell-rail order
// straight from the registry, and "Move window to" names every other desktop —
// the first nine dispatch their registry slot command, later ones move the
// focused window by desktop id under the family's registry availability; with
// no other desktop it stays in place disabled with its reason.
// Boundary IN: WindowMenu, the real move-target hook over a projection atom,
// the registry context, and the real menubar primitive.
// Boundary OUT: the window-manager controller (a stub records the move) and command execution.
// No existing suite owns the Window menu; the item adapter has its own suite.
import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { Menubar, UIProvider } from "@compozy/ui";

import { CmdPaletteRegistryProvider } from "../../../contexts/cmd-palette-registry-context";
import { OsShellContext, type OsShellHandle } from "../../../contexts/os-shell-context";
import type { OsDesktopRuntimeStore, WindowManagerController } from "../../../lib/os-types";
import { RoutingCoordinator } from "../../../lib/routing-coordinator";
import { createWindowManagerProjectionAtom } from "../../../lib/window-manager-projection";
import type { LayoutDesktop } from "../../../lib/window-manager-types";
import {
  paletteRegistryFixture,
  resolvedPaletteCommand,
} from "../../../mocks/cmd-palette-fixtures";
import { WindowMenu } from "../window-menu";

const REGISTRY = paletteRegistryFixture(
  [
    ["layout.arrange.grid", "Grid"],
    ["layout.balance", "Balance sizes"],
    ["layout.arrange.main-stack", "Main and stack"],
    ["layout.arrange.columns", "Columns"],
    ...Array.from({ length: 9 }, (_, index) => [
      `window.move_to_desktop.${index + 1}`,
      `Move window to desktop ${index + 1}`,
    ]),
  ].map(([id, title]) =>
    resolvedPaletteCommand({
      id,
      title,
      action: { kind: "client_op", op: id },
    })
  )
);

const DESKTOP_NAMES = [
  "Main",
  "Review",
  "Research",
  "Ops",
  "Launch",
  "Docs",
  "Support",
  "Finance",
  "Legal",
  "Archive",
];

function desktop(name: string, index: number): LayoutDesktop {
  return {
    id: `desktop:${index + 1}`,
    name,
    order: index,
    groups: [],
    floating: [],
    floatingStacks: [],
  };
}

function renderMenu(desktopCount: number, onRun = vi.fn()) {
  const state = {
    desktops: DESKTOP_NAMES.slice(0, desktopCount).map(desktop),
    activeDesktopId: "desktop:1",
    focusedId: "w-focused",
    windows: { "w-focused": { id: "w-focused", desktopId: "desktop:1" } },
  } as unknown as OsDesktopRuntimeStore;
  const moveWindowToDesktop = vi.fn();
  const manager = {
    getState: () => state,
    moveWindowToDesktop,
  } as unknown as WindowManagerController;
  const shell: OsShellHandle = {
    projection: createWindowManagerProjectionAtom(state),
    manager,
    coordinator: new RoutingCoordinator(manager, { navigate: vi.fn(), replace: vi.fn() }),
  };
  render(
    <UIProvider reducedMotion="never" skipAnimations>
      <OsShellContext.Provider value={shell}>
        <CmdPaletteRegistryProvider registry={REGISTRY}>
          <Menubar>
            <WindowMenu open onOpenChange={vi.fn()} onRun={onRun} />
          </Menubar>
        </CmdPaletteRegistryProvider>
      </OsShellContext.Provider>
    </UIProvider>
  );
  return { onRun, moveWindowToDesktop };
}

async function openMoveSubmenu(user: ReturnType<typeof userEvent.setup>) {
  // Keyboard path into the submenu: highlight the trigger row, then ArrowRight.
  const trigger = screen.getByTestId("os-menu-move-to-desktop");
  act(() => trigger.focus());
  await user.keyboard("{ArrowRight}");
}

describe("WindowMenu", () => {
  it("Should list the Arrange presets in the shell-rail order", () => {
    renderMenu(1);

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
    const { onRun } = renderMenu(3);

    await openMoveSubmenu(user);
    const research = await screen.findByTestId("os-menubar-command-window.move_to_desktop.3");
    expect(research).toHaveTextContent("Research");
    expect(screen.getByTestId("os-menubar-command-window.move_to_desktop.2")).toHaveTextContent(
      "Review"
    );

    act(() => research.focus());
    await user.keyboard("{Enter}");
    expect(onRun).toHaveBeenCalledExactlyOnceWith("window.move_to_desktop.3");
  });

  it("Should list every other desktop, moving past the ninth by desktop id", async () => {
    const user = userEvent.setup();
    const { onRun, moveWindowToDesktop } = renderMenu(10);

    await openMoveSubmenu(user);
    const archive = await screen.findByTestId("os-menubar-move-to-desktop-desktop:10");
    const submenu = archive.closest("[role=menu]");
    expect(
      within(submenu as HTMLElement)
        .getAllByRole("menuitem")
        .map(item => item.textContent)
    ).toEqual(DESKTOP_NAMES.slice(1));
    expect(screen.getByTestId("os-menubar-command-window.move_to_desktop.9")).toHaveTextContent(
      "Legal"
    );

    act(() => archive.focus());
    await user.keyboard("{Enter}");
    expect(moveWindowToDesktop).toHaveBeenCalledExactlyOnceWith("w-focused", "desktop:10");
    expect(onRun).not.toHaveBeenCalled();
  });

  it("Should disable Move window to with its reason when no other desktop exists", () => {
    renderMenu(1);

    const item = screen.getByTestId("os-menu-move-to-desktop");
    expect(item).toHaveAttribute("aria-disabled", "true");
    expect(item).toHaveTextContent("Move window to");
    expect(item).toHaveTextContent("needs another desktop");
  });
});
