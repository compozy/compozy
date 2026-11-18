import { readFileSync, readdirSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const appDir = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const siteDir = resolve(appDir, "..");
const globalCSS = readFileSync(resolve(appDir, "global.css"), "utf8");

function sourceFiles(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap(entry => {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) return entry.name === "__tests__" ? [] : sourceFiles(path);
    return /\.(css|tsx?)$/.test(entry.name) ? [path] : [];
  });
}

describe("site global styles", () => {
  it("does not globally suppress box-shadow-based focus rings", () => {
    expect(globalCSS).not.toContain("box-shadow: none !important;");
  });

  it("keeps the retired docs-shell tokens and masthead rules deleted", () => {
    const offenders = ["app", "components", "lib"]
      .flatMap(dir => sourceFiles(resolve(siteDir, dir)))
      .filter(file => /(?<![\w-])--site-doc-|\bsite-doc-masthead/.test(readFileSync(file, "utf8")));

    expect(offenders).toEqual([]);
  });
});
