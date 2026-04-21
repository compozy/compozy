import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

import * as ui from "../src/index";

describe("@compozy/ui exports", () => {
  it("exposes the stable public package surface", () => {
    expect(Object.keys(ui)).toEqual(
      expect.arrayContaining([
        "AppShell",
        "AppShellBrand",
        "AppShellContent",
        "AppShellHeader",
        "AppShellMain",
        "AppShellNavItem",
        "AppShellNavSection",
        "AppShellSidebar",
        "Button",
        "SectionHeading",
        "StatusBadge",
        "SurfaceCard",
        "SurfaceCardBody",
        "SurfaceCardDescription",
        "SurfaceCardEyebrow",
        "SurfaceCardFooter",
        "SurfaceCardHeader",
        "SurfaceCardTitle",
        "UIProvider",
        "buttonVariants",
        "cn",
      ])
    );
  });

  it("does not leak route-specific implementation names", () => {
    const exportNames = Object.keys(ui);

    expect(exportNames.some(name => /(route|workflow|task|review|run|page)/i.test(name))).toBe(
      false
    );
  });

  it("publishes package entrypoints for root, tokens, and utils", () => {
    const packageJson = JSON.parse(
      readFileSync(resolve(import.meta.dirname, "../package.json"), "utf8")
    ) as {
      exports: Record<string, string>;
    };

    expect(packageJson.exports["."]).toBe("./src/index.ts");
    expect(packageJson.exports["./tokens.css"]).toBe("./src/tokens.css");
    expect(packageJson.exports["./utils"]).toBe("./src/lib/utils.ts");
  });
});
