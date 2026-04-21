import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

describe("tokens.css", () => {
  const tokensPath = resolve(import.meta.dirname, "../src/tokens.css");
  const css = readFileSync(tokensPath, "utf8");

  it("ships the mockup font faces and dark theme defaults", () => {
    expect(css).toContain('font-family: "Nippo"');
    expect(css).toContain('font-family: "Disket Mono"');
    expect(css).toContain("--background: #1a1918");
    expect(css).toContain("--sidebar: #0d0c0b");
    expect(css).toContain('--font-display: "Nippo", "Disket Mono", var(--font-mono)');
  });

  it("defines shadcn-compatible theme tokens and tone styles", () => {
    expect(css).toContain("--color-background: var(--background)");
    expect(css).toContain("--color-sidebar-border: var(--sidebar-border)");
    expect(css).toContain("--tone-accent-bg");
    expect(css).toContain("--tone-info-text");
    expect(css).toContain("@layer base");
  });
});
