import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const appDir = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const globalCSS = readFileSync(resolve(appDir, "global.css"), "utf8");

describe("site global styles", () => {
  it("does not globally suppress box-shadow-based focus rings", () => {
    expect(globalCSS).not.toContain("box-shadow: none !important;");
  });

  it("maps Fumadocs semantic tokens to the AGH signal palette in dark mode", () => {
    expect(globalCSS).toContain("--color-fd-info: var(--color-info);");
    expect(globalCSS).toContain("--color-fd-warning: var(--color-warning);");
    expect(globalCSS).toContain("--color-fd-error: var(--color-danger);");
    expect(globalCSS).toContain("--color-fd-success: var(--color-success);");
    expect(globalCSS).toContain("--color-fd-idea: var(--color-accent);");
    expect(globalCSS).toContain("--color-fd-diff-remove: var(--color-danger-tint);");
    expect(globalCSS).toContain("--color-fd-diff-add: var(--color-success-tint);");
  });

  it("limits inline code chrome to textual content instead of fenced code blocks", () => {
    expect(globalCSS).toContain(
      ".site-doc-body :is(p, li, td, th, blockquote, h1, h2, h3, h4, h5, h6) code {"
    );
    expect(globalCSS).not.toContain(".site-doc-body :not(pre) > code {");
  });
});
