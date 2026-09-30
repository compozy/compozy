import { readFile } from "node:fs/promises";

import type { NativeTheme, TitleBarOverlay } from "electron";

import { writeFileAtomic } from "../files/atomic-write";
import { isThemePreference, type ThemePreference } from "../product/product-contract";
import { productTitleBarOverlay } from "./product-window-chrome";

/** Preference until the renderer reports one (mirrors the web default, D4). */
export const DEFAULT_THEME_PREFERENCE: ThemePreference = "dark";

/**
 * Native background per window surface and resolved theme — the color of the
 * first thing each page paints, so reveal and live-resize edges never flash.
 * `chrome` is the product shell's `--color-rail` (mirrors `THEME_CHROME_COLOR`
 * in web/src/systems/theme and web/public/theme-boot.js); `canvas` is the boot
 * page's `--canvas` (`--color-canvas`).
 */
export const WINDOW_BACKGROUNDS = {
  chrome: { dark: "#0a0a0a", light: "#f2f2f3" },
  canvas: { dark: "#131313", light: "#ffffff" },
} as const;
export type WindowSurface = keyof typeof WINDOW_BACKGROUNDS;

/** The slice of Electron's `nativeTheme` this reads and drives. */
interface ThemeSource {
  themeSource: NativeTheme["themeSource"];
  readonly shouldUseDarkColors: boolean;
  on(event: "updated", listener: () => void): unknown;
}

/** The slice of a `BrowserWindow` this repaints. */
interface ThemedWindow {
  setBackgroundColor(color: string): void;
  setTitleBarOverlay(options: TitleBarOverlay): void;
  isDestroyed(): boolean;
  once(event: "closed", listener: () => void): unknown;
}

/**
 * Keeps native chrome and window backgrounds in the renderer's theme.
 *
 * The renderer owns the preference (localStorage) and reports it over the
 * product bridge; this persists the last report and applies it through
 * `nativeTheme.themeSource`. Setting the source before any window exists makes
 * `prefers-color-scheme` in every renderer — the boot page included — resolve
 * to the user's theme from the first frame, keeps `system` following the OS,
 * and lets `shouldUseDarkColors` pick each window's background.
 */
export class WindowTheme {
  readonly #nativeTheme: ThemeSource;
  readonly #path: string;
  readonly #platform: NodeJS.Platform;
  readonly #windows = new Map<ThemedWindow, WindowSurface>();
  readonly #repaint = () => this.#paintWindows();
  #preference: ThemePreference = DEFAULT_THEME_PREFERENCE;
  /** The preference the file holds; null when absent, invalid, or unknown. */
  #persisted: ThemePreference | null = null;
  /** Serializes file writes so the last one to land carries the latest report. */
  #publication: Promise<void> = Promise.resolve();

  constructor(options: { nativeTheme: ThemeSource; path: string; platform?: NodeJS.Platform }) {
    this.#nativeTheme = options.nativeTheme;
    this.#path = options.path;
    this.#platform = options.platform ?? process.platform;
  }

  /** Applies the persisted preference; call before creating any window. */
  async load(): Promise<void> {
    this.#persisted = await readThemePreference(this.#path);
    this.#preference = this.#persisted ?? DEFAULT_THEME_PREFERENCE;
    this.#nativeTheme.themeSource = this.#preference;
    this.#nativeTheme.on("updated", this.#repaint);
  }

  /** The resolved theme native chrome paints in right now. */
  get dark(): boolean {
    return this.#nativeTheme.shouldUseDarkColors;
  }

  backgroundColor(surface: WindowSurface): string {
    const palette = WINDOW_BACKGROUNDS[surface];
    return this.dark ? palette.dark : palette.light;
  }

  /**
   * Repaints `window`'s background on every theme change until it closes; a
   * `chrome` window also recolors its Linux title-bar overlay (window controls).
   */
  track(window: ThemedWindow, surface: WindowSurface): void {
    this.#windows.set(window, surface);
    window.once("closed", () => this.#windows.delete(window));
  }

  /**
   * Adopts the renderer's preference: native theme now, persisted for the next
   * launch. Rapid reports queue one write at a time, each writing the latest
   * preference, so the file converges on the last report; a report matching an
   * unpersisted preference (an earlier write failed) writes it again.
   */
  async set(preference: ThemePreference): Promise<void> {
    if (preference !== this.#preference) {
      this.#preference = preference;
      this.#nativeTheme.themeSource = preference;
      this.#paintWindows();
    }
    const write = this.#publication.then(async () => {
      const latest = this.#preference;
      if (latest === this.#persisted) return;
      await writeFileAtomic(this.#path, `${JSON.stringify({ preference: latest })}\n`, 0o600);
      this.#persisted = latest;
    });
    // Keep the chain usable after a failure; this caller still receives it below.
    this.#publication = write.then(
      () => undefined,
      () => undefined
    );
    await write;
  }

  #paintWindows(): void {
    const overlay = productTitleBarOverlay(this.#platform, this.dark);
    for (const [window, surface] of this.#windows) {
      if (window.isDestroyed()) continue;
      window.setBackgroundColor(this.backgroundColor(surface));
      if (surface === "chrome" && overlay) window.setTitleBarOverlay(overlay);
    }
  }
}

async function readThemePreference(path: string): Promise<ThemePreference | null> {
  try {
    const value: unknown = JSON.parse(await readFile(path, "utf8"));
    const preference =
      value && typeof value === "object" ? (value as Record<string, unknown>).preference : null;
    return isThemePreference(preference) ? preference : null;
  } catch {
    return null;
  }
}
