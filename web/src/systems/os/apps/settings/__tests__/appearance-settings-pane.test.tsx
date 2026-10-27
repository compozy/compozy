// Suite: Appearance settings pane
// Invariant: the pane binds the browser theme preference (light/dark/system) to the theme runtime,
// wallpaper/reduce-motion to the OS controller with APG radio-group semantics, and states the
// system reduced-motion precedence truthfully (US-015.EC-1). The Interface and Code fonts (#984)
// are daemon-wide: a choice PATCHes only its own `shell.appearance` key, reset sends "", and the
// installed-font list (Local Font Access) offers only monospace families to the Code picker. A
// rejected save says why instead of silently reverting.
// Owning layer: the pane; the daemon's shell settings route and queryLocalFonts are mocked I/O.
import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { toast } from "sonner";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { OsShellContext, type OsShellHandle } from "../../../contexts/os-shell-context";
import { WindowManagerRuntime } from "../../../runtime/window-manager-runtime";
import { RoutingCoordinator, type OsRouterPort } from "../../../lib/routing-coordinator";
import { FontAppearanceSync, THEME_STORAGE_KEY, themePreferenceStore } from "@/systems/theme";
import { AppearanceSettingsPane } from "../appearance-settings-pane";

const managers: WindowManagerRuntime[] = [];

vi.mock("@/systems/settings", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/settings")>()),
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

const daemon: { appearance: { ui_font: string; mono_font: string } } = {
  appearance: { ui_font: "", mono_font: "" },
};

function json(body: unknown) {
  return new Response(JSON.stringify(body), { headers: { "content-type": "application/json" } });
}

let fetcher: ReturnType<typeof vi.spyOn<typeof globalThis, "fetch">>;

async function shellWrites() {
  const bodies: unknown[] = [];
  for (const [input] of fetcher.mock.calls) {
    const request = input as Request;
    if (request.method === "PATCH") bodies.push(await request.clone().json());
  }
  return bodies;
}

/** Families report equal `i`/`W` widths only when their name says Mono. */
function stubInstalledFonts(families: string[]) {
  vi.stubGlobal(
    "queryLocalFonts",
    vi.fn(async () => families.map(family => ({ family })))
  );
  let font = "";
  vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockReturnValue({
    set font(value: string) {
      font = value;
    },
    get font() {
      return font;
    },
    measureText: (text: string) => ({
      width: /Mono/.test(font) ? text.length * 10 : text.length * (text.startsWith("i") ? 4 : 12),
    }),
  } as unknown as CanvasRenderingContext2D);
}

function renderPane({ systemReducedMotion = false } = {}) {
  vi.stubGlobal("matchMedia", matchMediaStub(systemReducedMotion));
  const manager = new WindowManagerRuntime(new QueryClient());
  managers.push(manager);
  const port: OsRouterPort = { navigate: () => {}, replace: () => {} };
  const shell: OsShellHandle = {
    projection: manager.projectionAtom,
    manager,
    coordinator: new RoutingCoordinator(manager, port),
  };
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <OsShellContext.Provider value={shell}>
        <FontAppearanceSync />
        <AppearanceSettingsPane />
      </OsShellContext.Provider>
    </QueryClientProvider>
  );
  return { manager };
}

describe("AppearanceSettingsPane", () => {
  beforeEach(() => {
    daemon.appearance = { ui_font: "", mono_font: "" };
    fetcher = vi.spyOn(globalThis, "fetch").mockImplementation(async input => {
      const request = input as Request;
      if (new URL(request.url).pathname !== "/api/settings/shell") {
        return new Response(null, { status: 404 });
      }
      if (request.method === "PATCH") {
        const body = (await request.clone().json()) as {
          config: { appearance?: Partial<typeof daemon.appearance> };
        };
        Object.assign(daemon.appearance, body.config.appearance);
        return json({ applied: true });
      }
      return json({ config: { sessions: {}, appearance: { ...daemon.appearance } } });
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
    cleanup();
    for (const manager of managers.splice(0)) manager.destroy();
    themePreferenceStore.trigger.preferenceSet({ preference: "dark" });
    window.localStorage.clear();
    vi.unstubAllGlobals();
  });

  it("Should select wallpapers as a radio group with pointer and arrow keys", async () => {
    const user = userEvent.setup();
    const { manager } = renderPane();

    const group = screen.getByRole("radiogroup", { name: "Wallpaper" });
    expect(group).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /Flat/ })).toHaveAttribute("aria-checked", "true");

    await user.click(screen.getByRole("radio", { name: /Carbon/ }));
    expect(manager.getState().wallpaper).toBe("carbon");
    expect(screen.getByRole("radio", { name: /Carbon/ })).toHaveAttribute("aria-checked", "true");

    // Arrow keys move AND select (automatic activation), wrapping the group.
    screen.getByRole("radio", { name: /Carbon/ }).focus();
    await user.keyboard("{ArrowRight}");
    expect(manager.getState().wallpaper).toBe("flat");
    await user.keyboard("{ArrowLeft}");
    expect(manager.getState().wallpaper).toBe("carbon");
  });

  it("Should offer Light, Dark and System and persist the chosen theme", async () => {
    const user = userEvent.setup();
    renderPane();

    const group = screen.getByRole("group", { name: "Theme" });
    expect(group).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Dark" })).toHaveAttribute("aria-pressed", "true");

    await user.click(screen.getByRole("button", { name: "System" }));
    expect(themePreferenceStore.getSnapshot().context.preference).toBe("system");
    expect(window.localStorage.getItem(THEME_STORAGE_KEY)).toBe("system");
    expect(screen.getByRole("button", { name: "System" })).toHaveAttribute("aria-pressed", "true");

    await user.click(screen.getByRole("button", { name: "Light" }));
    expect(window.localStorage.getItem(THEME_STORAGE_KEY)).toBe("light");
  });

  it("Should write reduce-motion to the OS presentation controller", async () => {
    const user = userEvent.setup();
    const { manager } = renderPane();

    await user.click(screen.getByTestId("os-appearance-reduce-motion"));
    expect(manager.getState().reduceMotion).toBe(true);
  });

  it("Should state that the system reduced-motion preference wins while it is active (US-015.EC-1)", () => {
    renderPane({ systemReducedMotion: true });
    expect(
      screen.getByText(/system already prefers reduced motion — that preference wins/i)
    ).toBeInTheDocument();
  });

  it("Should save a catalog font for every client and reset it to the default", async () => {
    const user = userEvent.setup();
    renderPane();

    const trigger = screen.getByTestId("os-appearance-font-sans");
    await waitFor(() => expect(trigger).toHaveTextContent("Inter"));
    await user.click(trigger);
    await user.click(await screen.findByRole("option", { name: "JetBrains Mono" }));

    await waitFor(() => expect(trigger).toHaveTextContent("JetBrains Mono"));
    // Published only after the lazy face's stylesheet resolved.
    await waitFor(() =>
      expect(document.documentElement.style.getPropertyValue("--user-font-sans")).toBe(
        '"JetBrains Mono Variable"'
      )
    );
    expect(await shellWrites()).toEqual([
      { config: { sessions: {}, appearance: { ui_font: "JetBrains Mono Variable" } } },
    ]);

    await user.click(screen.getByTestId("os-appearance-font-sans-reset"));
    await waitFor(async () =>
      expect((await shellWrites()).at(-1)).toEqual({
        config: { sessions: {}, appearance: { ui_font: "" } },
      })
    );
    await waitFor(() => expect(trigger).toHaveTextContent("Inter"));
    await waitFor(() =>
      expect(document.documentElement.style.getPropertyValue("--user-font-sans")).toBe("")
    );
  });

  it("Should explain a rejected font save and keep the stored font", async () => {
    const user = userEvent.setup();
    const toastError = vi.spyOn(toast, "error").mockReturnValue("toast");
    fetcher.mockImplementation(async input => {
      const request = input as Request;
      if (request.method === "PATCH") {
        return new Response(JSON.stringify({ error: { message: "daemon unavailable" } }), {
          status: 503,
          headers: { "content-type": "application/json" },
        });
      }
      return json({ config: { sessions: {}, appearance: { ...daemon.appearance } } });
    });
    renderPane();

    const trigger = screen.getByTestId("os-appearance-font-mono");
    await waitFor(() => expect(trigger).toHaveTextContent("Geist Mono"));
    await user.click(trigger);
    await user.click(await screen.findByRole("option", { name: "JetBrains Mono" }));

    await waitFor(() => expect(toastError).toHaveBeenCalledTimes(1));
    expect(toastError.mock.calls[0]?.[0]).toMatch(/^Couldn't save the Code font/);
    await waitFor(() => expect(trigger).toHaveTextContent("Geist Mono"));
  });

  it("Should offer installed fonts, and only monospace ones for Code", async () => {
    const user = userEvent.setup();
    stubInstalledFonts(["Berkeley Mono", "Helvetica Neue", ".SF NS Private"]);
    renderPane();

    await user.click(screen.getByTestId("os-appearance-font-mono"));
    const codeInstalled = await screen.findByRole("group", { name: "Installed" });
    expect(
      within(codeInstalled)
        .getAllByRole("option")
        .map(option => option.textContent)
    ).toEqual(["Berkeley Mono"]);
    await user.keyboard("{Escape}");

    await user.click(screen.getByTestId("os-appearance-font-sans"));
    const sansInstalled = await screen.findByRole("group", { name: "Installed" });
    expect(
      within(sansInstalled)
        .getAllByRole("option")
        .map(option => option.textContent)
    ).toEqual(["Berkeley Mono", "Helvetica Neue"]);
  });
});
