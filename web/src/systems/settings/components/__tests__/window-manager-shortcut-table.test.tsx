// Suite: Settings shortcut table
// Invariant: every registry command is listed and bindable, and every contested claim is
// refused by the daemon, named after the command that holds it, and transferable only by an
// explicit act that leaves the loser visibly unbound.
// Boundary IN: the table, its recorder, its alias cell, and the single write path they share.
// Boundary OUT: HTTP transport, chord grammar (window-manager-shortcuts suite), and the
// daemon's own conflict arbitration (settings package suites).
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  parseSettingsWindowManagerSection,
  type WindowManagerSettingsWire,
} from "@/systems/os/lib/window-manager-settings-section";
import { WindowManagerSettingsError } from "@/systems/os/adapters/window-manager-settings-api";

import { useWindowManagerAliasEditor } from "../../hooks/use-window-manager-alias-editor";
import { useWindowManagerBindingMutations } from "../../hooks/use-window-manager-binding-mutations";
import { useGlobalShortcutRecorder } from "../../hooks/use-global-shortcut-recorder";
import { useWindowManagerShortcutRecorder } from "../../hooks/use-window-manager-shortcut-recorder";
import { settingsWindowManagerSectionFixture } from "../../mocks/window-manager-fixtures";
import { WindowManagerShortcutTable } from "../layouts/window-manager-shortcut-table";
import { WindowManagerGlobalHotkeys } from "../layouts/window-manager-global-hotkeys";

const { updateWindowManagerBindings } = vi.hoisted(() => ({
  updateWindowManagerBindings: vi.fn(),
}));

vi.mock("@/systems/os/adapters/window-manager-settings-api", async importOriginal => {
  const actual =
    await importOriginal<typeof import("@/systems/os/adapters/window-manager-settings-api")>();
  return { ...actual, updateWindowManagerBindings };
});

function sectionFrom(mutate: (wire: WindowManagerSettingsWire) => void = () => {}) {
  const wire = structuredClone(settingsWindowManagerSectionFixture) as WindowManagerSettingsWire;
  mutate(wire);
  return parseSettingsWindowManagerSection(wire);
}

/**
 * The live wiring, not a stand-in: the table, the recorder and the alias editor
 * share one write path, and most of what this suite proves lives in how they
 * react to the daemon's answer on that path.
 */
function ShortcutSurface({
  section = sectionFrom(),
}: {
  section?: ReturnType<typeof sectionFrom>;
}) {
  const mutations = useWindowManagerBindingMutations("workspace:alpha");
  const recorder = useWindowManagerShortcutRecorder(section, mutations);
  const globalRecorder = useGlobalShortcutRecorder(section, mutations);
  const aliases = useWindowManagerAliasEditor(
    section,
    mutations,
    commandId => section.commands.find(command => command.id === commandId)?.title ?? commandId
  );
  return (
    <>
      <WindowManagerShortcutTable aliases={aliases} recorder={recorder} section={section} />
      <WindowManagerGlobalHotkeys recorder={globalRecorder} section={section} />
    </>
  );
}

function renderTable(section?: ReturnType<typeof sectionFrom>) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const tree = (next?: ReturnType<typeof sectionFrom>) => (
    <QueryClientProvider client={client}>
      <ShortcutSurface section={next ?? section} />
    </QueryClientProvider>
  );
  const view = render(tree());
  return {
    ...view,
    /** Re-renders with the section a later daemon answer would have served. */
    showSection: (next: ReturnType<typeof sectionFrom>) => view.rerender(tree(next)),
  };
}

describe("WindowManagerShortcutTable", () => {
  beforeEach(() => {
    updateWindowManagerBindings.mockReset();
    updateWindowManagerBindings.mockResolvedValue(sectionFrom());
  });

  afterEach(() => {
    delete window.compozyShell;
  });

  it("Should mark global hotkeys as desktop-only in the browser [UT-150]", () => {
    renderTable();

    const globalHotkeys = screen.getByTestId("window-manager-global-hotkeys");
    expect(globalHotkeys).toHaveTextContent("Global hotkeys");
    expect(globalHotkeys).toHaveTextContent("desktop only");
    expect(globalHotkeys).toHaveTextContent("requires desktop shell");
    expect(
      within(globalHotkeys).getByRole("button", { name: /Record global hotkey/ })
    ).toBeDisabled();
  });

  it("Should show shell registration truth and the Accessibility deep link [UT-150]", () => {
    window.compozyShell = {
      platform: "darwin",
      on: vi.fn(() => () => {}),
      globalShortcuts: {
        sync: vi.fn(async () => []),
        status: vi.fn(async () => []),
      },
    };
    renderTable(
      sectionFrom(wire => {
        wire.global_shortcuts = [
          {
            command_id: "palette.summon.global",
            intended_chord: "meta+shift+KeyK",
            active_chord: "meta+shift+Space",
            status: "failed_in_use",
          },
          {
            command_id: "session.new",
            intended_chord: "meta+shift+KeyN",
            status: "failed_permission",
          },
        ];
      })
    );

    const globalHotkeys = screen.getByTestId("window-manager-global-hotkeys");
    expect(globalHotkeys).toHaveTextContent("captured");
    expect(globalHotkeys).toHaveTextContent("unavailable — in use by another application");
    expect(globalHotkeys).toHaveTextContent("Accessibility permission required");
    expect(
      within(globalHotkeys).getByRole("button", { name: /Open System Settings/ })
    ).toHaveAttribute(
      "href",
      "x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility"
    );
  });

  it("Should write the whole global map when a shell hotkey is recorded [UT-150]", async () => {
    window.compozyShell = {
      platform: "darwin",
      on: vi.fn(() => () => {}),
      globalShortcuts: {
        sync: vi.fn(async () => []),
        status: vi.fn(async () => []),
      },
    };
    const user = userEvent.setup();
    renderTable();

    await user.click(
      screen.getByRole("button", { name: /Record global hotkey for palette\.summon\.global/ })
    );
    await user.keyboard("{Meta>}{Shift>}k{/Shift}{/Meta}");

    await waitFor(() => expect(updateWindowManagerBindings).toHaveBeenCalledTimes(1));
    expect(updateWindowManagerBindings.mock.calls[0]?.[0]).toMatchObject({
      globalShortcuts: { "palette.summon.global": "meta+shift+KeyK" },
      workspaceId: "workspace:alpha",
    });
  });

  it("Should list the whole registry and narrow it by source [UT-148]", async () => {
    const user = userEvent.setup();
    renderTable();

    // Bindable-but-unavailable commands are still someone's to bind: the row
    // exists because the registry carries the command, not because this client
    // can run it right now.
    expect(screen.getByTestId("window-manager-shortcut-session.new")).toBeVisible();
    expect(screen.getByTestId("window-manager-shortcut-ext.notes.capture")).toBeVisible();
    expect(screen.getByTestId("window-manager-shortcut-window.move")).toBeVisible();

    await user.click(screen.getByTestId("shortcut-source-ext.notes"));

    expect(screen.getByTestId("window-manager-shortcut-ext.notes.capture")).toBeVisible();
    expect(screen.queryByTestId("window-manager-shortcut-session.new")).not.toBeInTheDocument();
  });

  it("Should send the whole override map when a chord is recorded [UT-148]", async () => {
    const user = userEvent.setup();
    renderTable();

    await user.click(screen.getByTestId("shortcut-recorder-window.close"));
    await user.keyboard("{Meta>}{Shift>}j{/Shift}{/Meta}");

    await waitFor(() => expect(updateWindowManagerBindings).toHaveBeenCalledTimes(1));
    // The daemon replaces the map wholesale, so the write carries every override
    // in force — not just the one that changed.
    expect(updateWindowManagerBindings.mock.calls[0]?.[0]).toMatchObject({
      workspaceId: "workspace:alpha",
      shortcuts: { "window.close": ["meta+shift+KeyJ"], "window.tab.new": ["meta+Digit3"] },
    });
  });

  it("Should block a taken chord naming its owner and transfer it only on overwrite [UT-148]", async () => {
    const user = userEvent.setup();
    // `session.new` ships with ⌘N in the fixture registry, so claiming it for
    // the notes command is exactly the daemon refusal the story describes.
    updateWindowManagerBindings.mockRejectedValueOnce(
      new WindowManagerSettingsError(
        "conflict",
        409,
        "shortcut_conflict",
        "session.new",
        "meta+KeyN"
      )
    );
    const afterOverwrite = sectionFrom(wire => {
      wire.config.shortcuts = { ...wire.config.shortcuts, "session.new": [] };
      wire.effective_shortcuts = {
        ...wire.effective_shortcuts,
        "session.new": [],
        "ext.notes.capture": ["meta+KeyN"],
      };
    });
    updateWindowManagerBindings.mockResolvedValueOnce(afterOverwrite);
    const view = renderTable();

    await user.click(screen.getByTestId("shortcut-recorder-ext.notes.capture"));
    await user.keyboard("{Meta>}n{/Meta}");

    const conflict = await screen.findByTestId("shortcut-conflict-ext.notes.capture");
    // The owner is named by its title, not by the id the daemon answered with.
    expect(conflict).toHaveTextContent("New session");
    expect(conflict).toHaveTextContent("⌘N is already used by");
    expect(updateWindowManagerBindings).toHaveBeenCalledTimes(1);
    expect(updateWindowManagerBindings.mock.calls[0]?.[0]).not.toHaveProperty("overwrite");

    await user.click(within(conflict).getByRole("button", { name: "Overwrite" }));

    await waitFor(() => expect(updateWindowManagerBindings).toHaveBeenCalledTimes(2));
    expect(updateWindowManagerBindings.mock.calls[1]?.[0]).toMatchObject({ overwrite: true });

    // The loser keeps its row and says what it lost.
    view.showSection(afterOverwrite);
    expect(screen.getByTestId("window-manager-shortcut-session.new")).toHaveTextContent("unbound");
  });

  it("Should keep Escape to itself while recording [UT-148]", async () => {
    const user = userEvent.setup();
    // The shell listens for Escape on `document` in the bubble phase and reads
    // it as "leave this surface" (`use-os-shortcuts.ts`). Cancelling a capture
    // must not also cost the operator their place.
    const shellListener = vi.fn();
    document.addEventListener("keydown", shellListener);
    renderTable();

    await user.click(screen.getByTestId("shortcut-recorder-window.close"));
    expect(screen.getByTestId("shortcut-recorder-window.close")).toHaveTextContent("Press keys…");
    shellListener.mockClear();
    await user.keyboard("{Escape}");

    expect(screen.getByTestId("shortcut-recorder-window.close")).not.toHaveTextContent(
      "Press keys…"
    );
    expect(shellListener).not.toHaveBeenCalled();
    expect(updateWindowManagerBindings).not.toHaveBeenCalled();
    document.removeEventListener("keydown", shellListener);
  });

  it("Should reset one command and every command [UT-148]", async () => {
    const user = userEvent.setup();
    renderTable();

    await user.click(screen.getByTestId("shortcut-reset-window.tab.new"));
    await waitFor(() => expect(updateWindowManagerBindings).toHaveBeenCalledTimes(1));
    expect(updateWindowManagerBindings.mock.calls[0]?.[0].shortcuts).not.toHaveProperty(
      "window.tab.new"
    );

    await user.click(screen.getByTestId("shortcut-reset-all"));
    await waitFor(() => expect(updateWindowManagerBindings).toHaveBeenCalledTimes(2));
    expect(updateWindowManagerBindings.mock.calls[1]?.[0].shortcuts).toEqual({});
  });

  it("Should reject an alias holding whitespace without asking the daemon [UT-149]", async () => {
    const user = userEvent.setup();
    renderTable();

    const field = screen.getByTestId("shortcut-alias-session.new");
    await user.type(field, "new session");
    await user.tab();

    expect(screen.getByText("1–32 characters, no whitespace")).toBeVisible();
    expect(field).toHaveAttribute("aria-invalid", "true");
    expect(updateWindowManagerBindings).not.toHaveBeenCalled();
  });

  it("Should save a valid alias and block one already taken [UT-149]", async () => {
    const user = userEvent.setup();
    renderTable();

    await user.type(screen.getByTestId("shortcut-alias-session.new"), "ns");
    await user.tab();

    await waitFor(() => expect(updateWindowManagerBindings).toHaveBeenCalledTimes(1));
    expect(updateWindowManagerBindings.mock.calls[0]?.[0].aliases).toMatchObject({
      "session.new": "ns",
    });

    updateWindowManagerBindings.mockRejectedValueOnce(
      new WindowManagerSettingsError("conflict", 409, "alias_conflict", "session.new", null, "ns")
    );
    await user.type(screen.getByTestId("shortcut-alias-window.close"), "ns");
    await user.tab();

    const conflict = await screen.findByTestId("alias-conflict-window.close");
    expect(conflict).toHaveTextContent("ns is already used by New session");
    expect(within(conflict).getByRole("button", { name: "Move alias" })).toBeVisible();
  });

  it("Should surface a diagnostic for an override the daemon could not resolve [UT-148]", () => {
    renderTable(
      sectionFrom(wire => {
        wire.diagnostics = [{ command_id: "ext.gone.command", message: "unknown command id" }];
      })
    );

    expect(screen.getByText(/ext\.gone\.command: unknown command id/)).toBeVisible();
  });
});
