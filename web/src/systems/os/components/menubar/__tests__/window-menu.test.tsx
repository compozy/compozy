// Suite: Window menu curation
// Invariant: the Window menu's "Move to desktop" names every other desktop —
// the first nine dispatch their registry slot command, later ones move the
// focused window by desktop id under the family's registry availability; with
// no other desktop it stays in place disabled with its reason. "Move to new tab" and the Arrange
// presets with Balance are registry items (US-010, US-012.AC-4); no retired control is listed.
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
    ...Array.from({ length: 9 }, (_, index) => [
      `window.move_to_desktop.${index + 1}`,
      `Move window to desktop ${index + 1}`,
    ]),
    ["window.unsplit", "Move to new tab"],
    ["layout.arrange.main-stack", "Main and stack"],
    ["layout.arrange.columns", "Columns"],
    ["layout.arrange.grid", "Grid"],
    ["layout.balance", "Balance sizes"],
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
    tabs: [
      {
        id: `tab:${index + 1}`,
        pinned: false,
        root: { id: `leaf:${index + 1}`, kind: "leaf", windowId: `w-${index + 1}` },
      },
    ],
    activeTabId: `tab:${index + 1}`,
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

  it("Should disable Move to desktop with its reason when no other desktop exists", () => {
    renderMenu(1);

    const item = screen.getByTestId("os-menu-move-to-desktop");
    expect(item).toHaveAttribute("aria-disabled", "true");
    expect(item).toHaveTextContent("Move to desktop");
    expect(item).toHaveTextContent("needs another desktop");
  });
});

describe("WindowMenu split and arrange", () => {
  it("Should run Move to new tab and every Arrange preset through the registry", async () => {
    const user = userEvent.setup();
    const { onRun } = renderMenu(1);

    act(() => screen.getByTestId("os-menubar-command-window.unsplit").focus());
    await user.keyboard("{Enter}");
    expect(onRun).toHaveBeenLastCalledWith("window.unsplit");

    act(() => screen.getByTestId("os-menu-arrange").focus());
    await user.keyboard("{ArrowRight}");
    const grid = await screen.findByTestId("os-menubar-command-layout.arrange.grid");
    const submenu = grid.closest("[role=menu]") as HTMLElement;
    expect(
      within(submenu)
        .getAllByRole("menuitem")
        .map(item => item.textContent)
    ).toEqual(["Main and stack", "Columns", "Grid", "Balance sizes"]);
    act(() => screen.getByTestId("os-menubar-command-layout.balance").focus());
    await user.keyboard("{Enter}");
    expect(onRun).toHaveBeenLastCalledWith("layout.balance");
  });

  it("Should list no retired window control [US-018.AC-1]", () => {
    renderMenu(2);
    const menu = screen.getByTestId("os-menu-window");
    expect(menu).not.toHaveTextContent(/minimi[sz]e|zoom|float|maximi[sz]e|merge all/i);
  });
});
