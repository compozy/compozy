import { readFileSync } from "node:fs";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

import type { TitleBarOverlay } from "electron";
import { afterEach, describe, expect, it } from "vitest";

import { productTitleBarOverlay } from "../product-window-chrome";

import { WINDOW_BACKGROUNDS, WindowTheme } from "../window-theme";

type Source = "light" | "dark" | "system";

/** Electron's nativeTheme semantics: the source wins unless it is `system`, which follows the OS. */
function fakeNativeTheme(osDark: boolean) {
  const listeners = new Set<() => void>();
  let source: Source = "system";
  const theme = {
    get themeSource(): Source {
      return source;
    },
    set themeSource(next: Source) {
      source = next;
      for (const listener of listeners) listener();
    },
    get shouldUseDarkColors(): boolean {
      return source === "system" ? osDark : source === "dark";
    },
    on(_event: "updated", listener: () => void) {
      listeners.add(listener);
      return theme;
    },
    switchOS(dark: boolean) {
      osDark = dark;
      for (const listener of listeners) listener();
    },
  };
  return theme;
}

function fakeWindow() {
  const closers: Array<() => void> = [];
  const window = {
    background: "",
    overlay: null as TitleBarOverlay | null,
    destroyed: false,
    setBackgroundColor(color: string) {
      window.background = color;
    },
    setTitleBarOverlay(options: TitleBarOverlay) {
      window.overlay = options;
    },
    isDestroyed: () => window.destroyed,
    once(_event: "closed", listener: () => void) {
      closers.push(listener);
      return window;
    },
    close() {
      window.destroyed = true;
      for (const closer of closers) closer();
    },
  };
  return window;
}

const TOKENS_DIR = join(__dirname, "../../../../packages/ui/src");

/** A color token as declared by a token file (tokens.css = dark default, tokens-light.css = light). */
function token(file: string, name: string): string | undefined {
  const source = readFileSync(join(TOKENS_DIR, file), "utf8");
  return new RegExp(`^\\s*--${name}:\\s*([^;]+);`, "m").exec(source)?.[1]?.trim();
}

const directories: string[] = [];

async function themePath(content?: string): Promise<string> {
  const directory = await mkdtemp(join(tmpdir(), "compozy-desktop-theme-"));
  directories.push(directory);
  const path = join(directory, "desktop-theme.json");
  if (content !== undefined) await writeFile(path, content);
  return path;
}

function createTheme(
  nativeTheme: ReturnType<typeof fakeNativeTheme>,
  path: string,
  platform: NodeJS.Platform = "darwin"
) {
  return new WindowTheme({ nativeTheme, path, platform });
}

// Invariant: windows open and stay painted in the renderer's last reported theme (default dark),
// the native theme source carries the preference so `system` follows the OS, Linux window controls
// on chrome windows recolor with it, and the report persists.
describe("WindowTheme", () => {
  afterEach(async () => {
    for (const directory of directories.splice(0))
      await rm(directory, { recursive: true, force: true });
  });

  it("Should paint window backgrounds with the surface tokens each page paints first", () => {
    expect(WINDOW_BACKGROUNDS).toEqual({
      chrome: {
        dark: token("tokens.css", "color-rail"),
        light: token("tokens-light.css", "color-rail"),
      },
      canvas: {
        dark: token("tokens.css", "color-canvas"),
        light: token("tokens-light.css", "color-canvas"),
      },
    });
  });

  it.each([undefined, "not json", '{"preference":"sepia"}'])(
    "Should open dark on a light OS when no valid preference is stored (%s)",
    async stored => {
      const nativeTheme = fakeNativeTheme(false);
      await createTheme(nativeTheme, await themePath(stored)).load();
      expect(nativeTheme.themeSource).toBe("dark");
      expect(nativeTheme.shouldUseDarkColors).toBe(true);
    }
  );

  it("Should open in the stored preference and let system follow the OS", async () => {
    const nativeTheme = fakeNativeTheme(false);
    const theme = createTheme(nativeTheme, await themePath('{"preference":"system"}'));
    await theme.load();
    const window = fakeWindow();
    theme.track(window, "chrome");

    expect(theme.backgroundColor("chrome")).toBe(WINDOW_BACKGROUNDS.chrome.light);
    nativeTheme.switchOS(true);
    expect(window.background).toBe(WINDOW_BACKGROUNDS.chrome.dark);
  });

  it("Should apply, repaint and persist a reported preference", async () => {
    const nativeTheme = fakeNativeTheme(true);
    const path = await themePath();
    const theme = createTheme(nativeTheme, path);
    await theme.load();
    const open = fakeWindow();
    const closed = fakeWindow();
    const boot = fakeWindow();
    theme.track(open, "chrome");
    theme.track(boot, "canvas");
    theme.track(closed, "chrome");
    closed.close();

    await theme.set("light");

    expect(nativeTheme.themeSource).toBe("light");
    expect(open.background).toBe(WINDOW_BACKGROUNDS.chrome.light);
    expect(boot.background).toBe(WINDOW_BACKGROUNDS.canvas.light);
    expect(closed.background).toBe("");
    expect(JSON.parse(await readFile(path, "utf8"))).toEqual({ preference: "light" });

    const relaunched = fakeNativeTheme(true);
    await createTheme(relaunched, path).load();
    expect(relaunched.shouldUseDarkColors).toBe(false);
  });

  it("Should recolor the Linux window controls of chrome windows only", async () => {
    const nativeTheme = fakeNativeTheme(true);
    const theme = createTheme(nativeTheme, await themePath(), "linux");
    await theme.load();
    const product = fakeWindow();
    const boot = fakeWindow();
    theme.track(product, "chrome");
    theme.track(boot, "canvas");

    await theme.set("light");

    expect(product.overlay).toEqual(productTitleBarOverlay("linux", false));
    expect(boot.overlay).toBeNull();
  });

  it("Should leave the title bar alone off Linux", async () => {
    const nativeTheme = fakeNativeTheme(true);
    const theme = createTheme(nativeTheme, await themePath(), "darwin");
    await theme.load();
    const product = fakeWindow();
    theme.track(product, "chrome");

    await theme.set("light");

    expect(theme.dark).toBe(false);
    expect(product.overlay).toBeNull();
  });
});
