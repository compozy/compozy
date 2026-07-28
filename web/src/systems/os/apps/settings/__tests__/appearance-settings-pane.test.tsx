// Suite: Appearance settings pane
// Invariant: the pane binds wallpaper/magnification/reduce-motion to the OS controller with APG
// radio-group semantics, and states the system reduced-motion precedence truthfully (US-015.EC-1).
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";

import { OsShellContext, type OsShellHandle } from "../../../contexts/os-shell-context";
import { WindowManagerRuntime } from "../../../runtime/window-manager-runtime";
import { RoutingCoordinator, type OsRouterPort } from "../../../lib/routing-coordinator";
import { AppearanceSettingsPane } from "../appearance-settings-pane";

const managers: WindowManagerRuntime[] = [];

vi.mock("@/systems/settings", () => ({
  useSettingsTopbar: vi.fn(),
}));

function matchMediaStub(matches: boolean) {
  return vi.fn().mockImplementation((query: string) => ({
    matches,
    media: query,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    addListener: vi.fn(),
    removeListener: vi.fn(),
  }));
}

function renderPane({ systemReducedMotion = false } = {}) {
  vi.stubGlobal("matchMedia", matchMediaStub(systemReducedMotion));
  const manager = new WindowManagerRuntime(new QueryClient());
  managers.push(manager);
  const port: OsRouterPort = { navigate: () => {}, replace: () => {} };
  const shell: OsShellHandle = {
    store: manager,
    manager,
    coordinator: new RoutingCoordinator(manager, port),
  };
  render(
    <OsShellContext.Provider value={shell}>
      <AppearanceSettingsPane />
    </OsShellContext.Provider>
  );
  return { manager };
}

describe("AppearanceSettingsPane", () => {
  afterEach(() => {
    for (const manager of managers.splice(0)) manager.destroy();
    vi.unstubAllGlobals();
  });

  it("Should select wallpapers as a radio group with pointer and arrow keys", async () => {
    const user = userEvent.setup();
    const { manager } = renderPane();

    const group = screen.getByRole("radiogroup", { name: "Wallpaper" });
    expect(group).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /Ember/ })).toHaveAttribute("aria-checked", "true");

    await user.click(screen.getByRole("radio", { name: /Carbon/ }));
    expect(manager.getState().wallpaper).toBe("carbon");
    expect(screen.getByRole("radio", { name: /Carbon/ })).toHaveAttribute("aria-checked", "true");

    // Arrow keys move AND select (automatic activation), wrapping the group.
    screen.getByRole("radio", { name: /Carbon/ }).focus();
    await user.keyboard("{ArrowRight}");
    expect(manager.getState().wallpaper).toBe("ember");
    await user.keyboard("{ArrowLeft}");
    expect(manager.getState().wallpaper).toBe("carbon");
  });

  it("Should write magnification and reduce-motion to the OS presentation controller", async () => {
    const user = userEvent.setup();
    const { manager } = renderPane();

    await user.click(screen.getByTestId("os-appearance-magnify"));
    expect(manager.getState().dockMagnify).toBe(false);

    await user.click(screen.getByTestId("os-appearance-reduce-motion"));
    expect(manager.getState().reduceMotion).toBe(true);
  });

  it("Should state that the system reduced-motion preference wins while it is active (US-015.EC-1)", () => {
    renderPane({ systemReducedMotion: true });
    expect(
      screen.getByText(/system already prefers reduced motion — that preference wins/i)
    ).toBeInTheDocument();
  });
});
