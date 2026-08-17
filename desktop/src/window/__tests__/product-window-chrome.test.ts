import { describe, expect, it } from "vitest";

import { productWindowChrome } from "../product-window-chrome";

// Suite: product window chrome policy
// Invariant: native product chrome occupies the menubar only on supported desktop platforms.
// Boundary IN: platform-specific BrowserWindow title-bar options.
// Boundary OUT: Electron rendering and menubar layout, owned by desktop/e2e/_electron/__tests__/shell.spec.ts.
describe("product window chrome policy", () => {
  it("Should expose native macOS traffic lights through the menubar", () => {
    expect(productWindowChrome("darwin")).toEqual({
      titleBarStyle: "hidden",
      titleBarOverlay: { height: 44 },
    });
  });

  it("Should expose native Linux controls over a transparent menubar", () => {
    expect(productWindowChrome("linux")).toEqual({
      titleBarStyle: "hidden",
      titleBarOverlay: {
        color: "#00000000",
        symbolColor: "#ececef",
        height: 44,
      },
    });
  });

  it("Should preserve the system title bar on unsupported platforms", () => {
    expect(productWindowChrome("win32")).toEqual({});
  });
});
