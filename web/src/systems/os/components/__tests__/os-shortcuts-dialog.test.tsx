// Suite: OS shortcuts dialog
// Invariant: a shell action that needs the live window-manager command fence stays
// unavailable until that fence is established, and every configurable registry action stays
// discoverable in the keyboard reference.
// Boundary IN: OsShortcutsDialog, its shell context, and the window-manager runtime.
// Boundary OUT: daemon synchronization and browser-level settings navigation.
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";

import { OsShellContext, type OsShellHandle } from "../../contexts/os-shell-context";
import { RoutingCoordinator, type OsRouterPort } from "../../lib/routing-coordinator";
import { WindowManagerRuntime } from "../../runtime/window-manager-runtime";
import { OsShortcutsDialog } from "../os-shortcuts-dialog";

const { desktopState } = vi.hoisted(() => ({
  desktopState: {
    hydration: "pending",
    connectionStatus: "disconnected",
    snapshot: null,
    client: null,
    windowManagerConfig: {
      shortcuts: { "workspace.picker": ["meta+control+KeyO"] },
      effectiveShortcuts: {
        "workspace.picker": ["meta+control+KeyO", "meta+shift+KeyO"],
        "shortcuts.cheatsheet": ["shift+Slash", "meta+Slash"],
      },
    },
  },
}));

vi.mock("../../hooks/use-desktop", () => ({
  useDesktop: (selector: (state: typeof desktopState) => unknown) => selector(desktopState),
}));

const managers: WindowManagerRuntime[] = [];

function createShell(): OsShellHandle {
  const manager = new WindowManagerRuntime(new QueryClient());
  managers.push(manager);
  const router: OsRouterPort = { navigate: () => {}, replace: () => {} };
  const coordinator = new RoutingCoordinator(manager, router);
  coordinator.completeHydration();
  return { projection: manager.projectionAtom, manager, coordinator };
}

describe("OsShortcutsDialog", () => {
  afterEach(() => {
    desktopState.hydration = "pending";
    desktopState.connectionStatus = "disconnected";
    for (const manager of managers.splice(0)) manager.destroy();
  });

  it("Should retain the dialog while window-manager commands are unavailable", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();
    const shell = createShell();
    render(
      <OsShellContext.Provider value={shell}>
        <OsShortcutsDialog open onOpenChange={onOpenChange} />
      </OsShellContext.Provider>
    );

    const edit = screen.getByTestId("os-shortcuts-edit");
    expect(edit).toBeDisabled();
    await user.click(edit);
    expect(onOpenChange).not.toHaveBeenCalled();
  });

  it("Should list daemon-fed alternates and the surface-local reference [UT-075]", () => {
    const shell = createShell();
    render(
      <OsShellContext.Provider value={shell}>
        <OsShortcutsDialog open onOpenChange={vi.fn()} />
      </OsShellContext.Provider>
    );

    expect(screen.getByText("Workspaces")).toBeVisible();
    expect(screen.getByTestId("os-shortcut-row-workspace.picker")).toHaveTextContent(
      "Workspace picker"
    );
    expect(screen.getByTestId("os-shortcut-row-workspace.picker")).toHaveTextContent("⌘⌃O");
    expect(screen.getByTestId("os-shortcut-local-composer.focus")).toHaveTextContent(
      "Focus composer"
    );
  });
});
