import { describe, expect, it } from "vitest";

import { productTitleBarOverlay, productWindowChrome } from "../product-window-chrome";

// Suite: product window chrome policy
// Invariant: native product chrome occupies the 52px topbar only on supported desktop platforms,
// macOS lights sit centred in the topbar's leading reserve, and Linux controls follow the theme.
// Boundary IN: platform-specific BrowserWindow title-bar options.
// Boundary OUT: Electron rendering and menubar layout, owned by desktop/e2e/_electron/__tests__/shell.spec.ts.
describe("product window chrome policy", () => {
  it("Should expose native macOS traffic lights centred in the topbar's leading reserve", () => {
    expect(productWindowChrome("darwin", true)).toEqual({
      titleBarStyle: "hidden",
      titleBarOverlay: { height: 52 },
      trafficLightPosition: { x: 15, y: 19 },
    });
    expect(productWindowChrome("darwin", false)).toEqual(productWindowChrome("darwin", true));
  });

  it("Should paint native Linux controls on the rail surface of the resolved theme", () => {
    expect(productWindowChrome("linux", true)).toEqual({
      titleBarStyle: "hidden",
      titleBarOverlay: { color: "#0a0a0a", symbolColor: "#f5f5f5", height: 52 },
    });
    expect(productWindowChrome("linux", false)).toEqual({
      titleBarStyle: "hidden",
      titleBarOverlay: { color: "#f2f2f3", symbolColor: "#1a1a1a", height: 52 },
    });
  });

  it("Should recolor only the Linux overlay when the theme changes", () => {
    expect(productTitleBarOverlay("linux", false)).toEqual({
      color: "#f2f2f3",
      symbolColor: "#1a1a1a",
      height: 52,
    });
    expect(productTitleBarOverlay("darwin", false)).toBeUndefined();
    expect(productTitleBarOverlay("win32", true)).toBeUndefined();
  });

  it("Should preserve the system title bar on unsupported platforms", () => {
    expect(productWindowChrome("win32", true)).toEqual({});
  });
});
