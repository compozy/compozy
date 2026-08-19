// Suite: MenubarCommandItem registry projection
// Invariant: a curated menu item renders and dispatches only the live registry
// command, preserves unavailable commands with their reason, and disappears
// cleanly when its catalog contribution is removed.
// Boundary IN: MenubarCommandItem, registry context, and the real menubar primitive.
// Boundary OUT: catalog transport, availability evaluation, and command execution.
// No existing web suite owns this generic registry-to-menubar adapter.
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { Menubar, MenubarContent, MenubarMenu, MenubarTrigger, UIProvider } from "@compozy/ui";

import { CmdPaletteRegistryProvider } from "../../../contexts/cmd-palette-registry-context";
import type { PaletteRegistry, ResolvedPaletteCommand } from "../../../lib/cmd-palette-types";
import { MenubarCommandItem } from "../menubar-command-item";

function command(overrides: Partial<ResolvedPaletteCommand> = {}): ResolvedPaletteCommand {
  return {
    id: "window.close",
    title: "Close window",
    section: "Window",
    icon: "x-square",
    source: "core",
    bindings: ["meta+KeyW"],
    alias: null,
    destructive: false,
    availability_exempt: false,
    arguments: [],
    action: { kind: "client_op", op: "window.close" },
    execution: { retry_safe: false, single_flight: true },
    visible: true,
    available: true,
    reason: "",
    chords: ["⌘W"],
    ...overrides,
  } as ResolvedPaletteCommand;
}

function registry(commands: readonly ResolvedPaletteCommand[]): PaletteRegistry {
  return {
    commands,
    byId: new Map(commands.map(entry => [entry.id, entry])),
    sources: [{ source: "core", status: "healthy" }],
    catalogRevision: "sha256:test",
    stale: false,
    daemonReachable: true,
  };
}

function Fixture({
  commandId,
  liveRegistry,
  onRun,
}: {
  commandId: string;
  liveRegistry: PaletteRegistry;
  onRun: (commandId: string) => void;
}) {
  return (
    <UIProvider reducedMotion="never" skipAnimations>
      <CmdPaletteRegistryProvider registry={liveRegistry}>
        <Menubar>
          <MenubarMenu open>
            <MenubarTrigger>Window</MenubarTrigger>
            <MenubarContent>
              <MenubarCommandItem commandId={commandId} onRun={onRun} />
            </MenubarContent>
          </MenubarMenu>
        </Menubar>
      </CmdPaletteRegistryProvider>
    </UIProvider>
  );
}

describe("MenubarCommandItem", () => {
  it("Should render registry title, chord, and availability and dispatch its id [UT-145]", async () => {
    const user = userEvent.setup();
    const onRun = vi.fn();
    render(<Fixture commandId="window.close" liveRegistry={registry([command()])} onRun={onRun} />);

    const item = screen.getByTestId("os-menubar-command-window.close");
    expect(item).toHaveTextContent("Close window");
    expect(item).toHaveTextContent("⌘W");
    expect(item).not.toHaveAttribute("aria-disabled", "true");

    await user.click(item);
    expect(onRun).toHaveBeenCalledExactlyOnceWith("window.close");
  });

  it("Should replace a stale chord when the effective keymap changes [UT-102]", () => {
    const onRun = vi.fn();
    const rendered = render(
      <Fixture commandId="window.close" liveRegistry={registry([command()])} onRun={onRun} />
    );
    expect(screen.getByTestId("os-menubar-command-window.close")).toHaveTextContent("⌘W");

    rendered.rerender(
      <Fixture
        commandId="window.close"
        liveRegistry={registry([command({ bindings: ["control+shift+KeyW"], chords: ["⌃⇧W"] })])}
        onRun={onRun}
      />
    );

    const rebound = screen.getByTestId("os-menubar-command-window.close");
    expect(rebound).toHaveTextContent("⌃⇧W");
    expect(rebound).not.toHaveTextContent("⌘W");
  });

  it("Should keep an unavailable command disabled with its verbatim reason [UT-146]", async () => {
    const user = userEvent.setup();
    const onRun = vi.fn();
    render(
      <Fixture
        commandId="window.close"
        liveRegistry={registry([
          command({ available: false, reason: "requires a focused window" }),
        ])}
        onRun={onRun}
      />
    );

    const item = screen.getByTestId("os-menubar-command-window.close");
    expect(item).toHaveTextContent("Close window");
    expect(item).toHaveAttribute("aria-disabled", "true");
    expect(item).toHaveAttribute("title", "requires a focused window");

    await user.click(item);
    expect(onRun).not.toHaveBeenCalled();
  });

  it("Should remove a disabled extension contribution without breaking the menu [UT-147]", () => {
    const onRun = vi.fn();
    const extensionCommand = command({
      id: "ext.notes.capture",
      title: "Capture note",
      source: "extension:notes",
      action: { kind: "client_op", op: "ext.notes.capture" },
    });
    const rendered = render(
      <Fixture
        commandId={extensionCommand.id}
        liveRegistry={registry([extensionCommand])}
        onRun={onRun}
      />
    );
    expect(screen.getByTestId("os-menubar-command-ext.notes.capture")).toHaveTextContent(
      "Capture note"
    );

    rendered.rerender(
      <Fixture commandId={extensionCommand.id} liveRegistry={registry([])} onRun={onRun} />
    );
    expect(screen.queryByTestId("os-menubar-command-ext.notes.capture")).not.toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: "Window" })).toBeInTheDocument();
  });
});
