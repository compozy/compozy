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
import { CmdPaletteRegistryProvider } from "../../contexts/cmd-palette-registry-context";
import type { PaletteRegistry, ResolvedPaletteCommand } from "../../lib/cmd-palette-types";
import { OsShortcutsDialog } from "../os-shortcuts-dialog";

/**
 * The cheatsheet is a projection of the registry: a row exists because the
 * catalog carries the command, and its chords come from the daemon keymap.
 */
const CHEATSHEET_REGISTRY: PaletteRegistry = (() => {
  const commands = [
    { id: "workspace.picker", title: "Workspace picker", section: "Workspaces", source: "core" },
    { id: "shortcuts.cheatsheet", title: "This sheet", section: "Shell", source: "core" },
    {
      id: "ext.notes.capture",
      title: "Capture note",
      section: "Notes",
      source: "ext.notes",
      alias: "cap",
    },
  ].map(entry => ({
    icon: "command",
    bindings: [],
    alias: null,
    ...entry,
    destructive: false,
    availability_exempt: true,
    arguments: [],
    action: { kind: "client_op", op: entry.id },
    execution: { retry_safe: true, single_flight: false },
    visible: true,
    available: true,
    reason: "",
    chords: [],
  })) as unknown as ResolvedPaletteCommand[];
  return {
    commands,
    byId: new Map(commands.map(command => [command.id, command])),
    sources: [{ source: "core", status: "healthy" }],
    catalogRevision: "sha256:test",
    stale: false,
    daemonReachable: true,
  };
})();

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
        "ext.notes.capture": ["alt+shift+KeyN", "meta+alt+KeyN"],
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
        <CmdPaletteRegistryProvider registry={CHEATSHEET_REGISTRY}>
          <OsShortcutsDialog open onOpenChange={onOpenChange} />
        </CmdPaletteRegistryProvider>
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
        <CmdPaletteRegistryProvider registry={CHEATSHEET_REGISTRY}>
          <OsShortcutsDialog open onOpenChange={vi.fn()} />
        </CmdPaletteRegistryProvider>
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

  it("Should group rows by contributing source and carry each alias [UT-151]", () => {
    const shell = createShell();
    render(
      <OsShellContext.Provider value={shell}>
        <CmdPaletteRegistryProvider registry={CHEATSHEET_REGISTRY}>
          <OsShortcutsDialog open onOpenChange={vi.fn()} />
        </CmdPaletteRegistryProvider>
      </OsShellContext.Provider>
    );

    // Core first, then whatever contributed — a band exists because something
    // is under it, so an extension's rows can never be silently folded in.
    expect(screen.getByTestId("os-shortcut-source-core")).toHaveTextContent("Core areas");
    expect(screen.getByTestId("os-shortcut-source-ext.notes")).toHaveTextContent("notes");
    expect(screen.getByTestId("os-shortcut-row-ext.notes.capture")).toHaveTextContent(
      "Capture note (cap)"
    );
    expect(screen.getByTestId("os-shortcut-row-ext.notes.capture")).toHaveTextContent("⌥⇧N");
    expect(screen.getByTestId("os-shortcut-row-ext.notes.capture")).toHaveTextContent("⌘⌥N");
  });
});
