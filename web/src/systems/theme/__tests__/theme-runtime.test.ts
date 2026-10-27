// Suite: Theme runtime
// Invariant: one theme preference per browser (localStorage `compozy.theme`, default dark) resolves
// to a painted theme; <html> data-theme, `.dark`, color-scheme and theme-color always move together,
// follow the OS live under `system`, sync across tabs, reach the desktop shell (native chrome), and
// the pre-paint boot script paints exactly what the runtime would (no flash, no mismatch).
// Appearance fonts (#984) ride the same boot script: the daemon's family names become the
// `--user-font-*` overrides heading the token stacks, cached for a flash-free next boot, and a
// bundled face downloads only once chosen.
import { readFileSync } from "node:fs";
import { join } from "node:path";

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { THEME_CHROME_COLOR, applyTheme } from "../lib/apply-theme";
import {
  FONT_STORAGE_KEY,
  applyFontAppearance,
  isStorableFontFamily,
  loadBundledFonts,
  writeFontAppearanceCache,
} from "../lib/font-appearance";
import { installThemeRuntime } from "../lib/install-theme-runtime";
import { THEME_STORAGE_KEY, readThemePreference, resolveTheme } from "../lib/theme-preference";
import { selectResolvedTheme, themePreferenceLogic } from "../stores/theme-preference-store";
import type { ThemePreference } from "../types";

const jetBrainsMono = vi.hoisted(() => vi.fn());
vi.mock("@fontsource-variable/jetbrains-mono", () => {
  jetBrainsMono();
  return {};
});

const BOOT_SCRIPT = readFileSync(join(__dirname, "../../../../public/theme-boot.js"), "utf8");
const TOKENS_DIR = join(__dirname, "../../../../../packages/ui/src");

/** The `--color-rail` declared by a token file (tokens.css = dark default, tokens-light.css = light). */
function railToken(file: string): string | undefined {
  const source = readFileSync(join(TOKENS_DIR, file), "utf8");
  return /^\s*--color-rail:\s*([^;]+);/m.exec(source)?.[1]?.trim();
}

type SchemeListener = (event: { matches: boolean }) => void;

function stubSystemScheme(prefersDark: boolean) {
  const listeners = new Set<SchemeListener>();
  vi.stubGlobal(
    "matchMedia",
    vi.fn().mockImplementation((query: string) => ({
      matches: prefersDark,
      media: query,
      addEventListener: (_type: string, listener: SchemeListener) => listeners.add(listener),
      removeEventListener: (_type: string, listener: SchemeListener) => listeners.delete(listener),
    }))
  );
  return {
    change(next: boolean) {
      prefersDark = next;
      for (const listener of listeners) listener({ matches: next });
    },
    listenerCount: () => listeners.size,
  };
}

function paintedState() {
  const root = document.documentElement;
  return {
    dataTheme: root.dataset.theme,
    dark: root.classList.contains("dark"),
    colorScheme: root.style.colorScheme,
    themeColor: document.querySelector('meta[name="theme-color"]')?.getAttribute("content"),
  };
}

function resetDocument() {
  const root = document.documentElement;
  delete root.dataset.theme;
  root.classList.remove("dark");
  root.style.colorScheme = "";
  document.head.innerHTML = '<meta name="theme-color" content="#E8572A" />';
}

function createStore(preference: ThemePreference, systemPrefersDark = false) {
  return themePreferenceLogic.createStore({ preference, systemPrefersDark });
}

describe("theme runtime", () => {
  beforeEach(() => {
    window.localStorage.clear();
    resetDocument();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("Should default to dark and ignore unknown stored values", () => {
    expect(readThemePreference()).toBe("dark");
    window.localStorage.setItem(THEME_STORAGE_KEY, "sepia");
    expect(readThemePreference()).toBe("dark");
    window.localStorage.setItem(THEME_STORAGE_KEY, "system");
    expect(readThemePreference()).toBe("system");
  });

  it("Should persist explicit choices and toggle the resolved theme to an explicit value", () => {
    const store = createStore("dark");

    store.trigger.preferenceSet({ preference: "system" });
    expect(window.localStorage.getItem(THEME_STORAGE_KEY)).toBe("system");
    expect(selectResolvedTheme(store.getSnapshot().context)).toBe("light");

    // Under `system` resolving to light, the toggle stores an explicit dark (D4).
    store.trigger.resolvedToggled();
    expect(store.getSnapshot().context.preference).toBe("dark");
    expect(window.localStorage.getItem(THEME_STORAGE_KEY)).toBe("dark");

    store.trigger.resolvedToggled();
    expect(store.getSnapshot().context.preference).toBe("light");
    expect(window.localStorage.getItem(THEME_STORAGE_KEY)).toBe("light");
  });

  it("Should set data-theme, the dark class, color-scheme and theme-color together", () => {
    applyTheme("light");
    expect(paintedState()).toEqual({
      dataTheme: "light",
      dark: false,
      colorScheme: "light",
      themeColor: "#f2f2f3",
    });

    applyTheme("dark");
    expect(paintedState()).toEqual({
      dataTheme: "dark",
      dark: true,
      colorScheme: "dark",
      themeColor: "#0a0a0a",
    });
  });

  it("Should paint the browser chrome with each theme's --color-rail token", () => {
    // The boot script is pinned to the runtime by the boot-paint parity cases below.
    expect(THEME_CHROME_COLOR).toEqual({
      dark: railToken("tokens.css"),
      light: railToken("tokens-light.css"),
    });
  });

  it("Should repaint on preference changes and follow the OS live under system", () => {
    const scheme = stubSystemScheme(true);
    const store = createStore("light", true);
    const teardown = installThemeRuntime(store);
    expect(paintedState().dataTheme).toBe("light");

    store.trigger.preferenceSet({ preference: "system" });
    expect(paintedState()).toMatchObject({ dataTheme: "dark", dark: true });

    scheme.change(false);
    expect(paintedState()).toMatchObject({ dataTheme: "light", dark: false });

    teardown();
    expect(scheme.listenerCount()).toBe(0);
  });

  it("Should adopt a preference written by another tab without writing it back", () => {
    stubSystemScheme(false);
    const store = createStore("dark");
    const teardown = installThemeRuntime(store);

    window.dispatchEvent(
      new StorageEvent("storage", { key: THEME_STORAGE_KEY, newValue: "light" })
    );
    expect(store.getSnapshot().context.preference).toBe("light");
    expect(paintedState().dataTheme).toBe("light");
    expect(window.localStorage.getItem(THEME_STORAGE_KEY)).toBeNull();

    teardown();
  });

  it("Should report the preference to the desktop shell now and on every change", () => {
    stubSystemScheme(false);
    const set = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal("compozyShell", { theme: { set } });
    const store = createStore("light");
    const teardown = installThemeRuntime(store);

    store.trigger.preferenceSet({ preference: "system" });
    expect(set.mock.calls).toEqual([["light"], ["system"]]);

    teardown();
    store.trigger.preferenceSet({ preference: "dark" });
    expect(set).toHaveBeenCalledTimes(2);
  });

  it.each([
    [null, false],
    [null, true],
    ["light", true],
    ["dark", false],
    ["system", true],
    ["system", false],
    ["garbage", false],
  ] as const)(
    "Should boot-paint stored %s (system dark %s) exactly as the runtime does",
    (stored, systemDark) => {
      stubSystemScheme(systemDark);
      if (stored !== null) window.localStorage.setItem(THEME_STORAGE_KEY, stored);

      new Function(BOOT_SCRIPT)();
      const booted = paintedState();

      resetDocument();
      applyTheme(resolveTheme(readThemePreference(), systemDark));
      expect(booted).toEqual(paintedState());
    }
  );
});

describe("font appearance runtime", () => {
  const root = document.documentElement;
  const fontVariables = () => ({
    sans: root.style.getPropertyValue("--user-font-sans"),
    mono: root.style.getPropertyValue("--user-font-mono"),
  });

  beforeEach(() => {
    window.localStorage.clear();
    resetDocument();
    root.style.removeProperty("--user-font-sans");
    root.style.removeProperty("--user-font-mono");
    jetBrainsMono.mockClear();
  });

  it("Should head the stacks with the chosen families and drop the override on reset", () => {
    applyFontAppearance(root, { sans: "JetBrains Mono Variable", mono: "ui-monospace" });
    // Chromium lacks the `ui-monospace` generic, so System names the platform faces too.
    expect(fontVariables()).toEqual({
      sans: '"JetBrains Mono Variable"',
      mono: 'ui-monospace, "SF Mono", "SFMono-Regular", Menlo, Consolas, "Liberation Mono"',
    });

    applyFontAppearance(root, { sans: "", mono: "" });
    expect(fontVariables()).toEqual({ sans: "", mono: "" });
  });

  it.each([
    [{ sans: "Berkeley Mono", mono: "system-ui" }],
    [{ sans: "", mono: "Fira Code" }],
    [{ sans: "", mono: "" }],
  ])("Should boot-paint the cached fonts %o exactly as the runtime does", appearance => {
    writeFontAppearanceCache(appearance);
    new Function(BOOT_SCRIPT)();
    const booted = fontVariables();

    root.style.removeProperty("--user-font-sans");
    root.style.removeProperty("--user-font-mono");
    applyFontAppearance(root, appearance);
    expect(booted).toEqual(fontVariables());
  });

  it("Should keep the built-in stacks when the font cache is corrupt", () => {
    window.localStorage.setItem(FONT_STORAGE_KEY, "{not json");
    new Function(BOOT_SCRIPT)();
    expect(fontVariables()).toEqual({ sans: "", mono: "" });
  });

  it("Should download a bundled face only once it is chosen", async () => {
    await loadBundledFonts({ sans: "", mono: "Fira Code" });
    expect(jetBrainsMono).not.toHaveBeenCalled();

    await loadBundledFonts({ sans: "", mono: "JetBrains Mono Variable" });
    expect(jetBrainsMono).toHaveBeenCalledTimes(1);
  });

  it.each([
    ["JetBrains Mono Variable", true],
    ["Source Han Sans", true],
    ["Operator's Font 2.0", true],
    ["", false],
    [" Inter", false],
    ['Inter", serif', false],
    ["Inter; color: red", false],
    ["Fira Code, monospace", false],
    ["x".repeat(129), false],
  ])("Should mirror the daemon's family rule for %j (storable: %s)", (family, storable) => {
    expect(isStorableFontFamily(family)).toBe(storable);
  });
});
