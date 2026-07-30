import { readFileSync, readdirSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const siteRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..");
const docsRoot = resolve(siteRoot, "content/docs");

function generatedCLITopLevelPages(): string[] {
  return readdirSync(resolve(docsRoot, "cli"), { withFileTypes: true })
    .filter(
      entry =>
        entry.isDirectory() ||
        (entry.isFile() && entry.name.endsWith(".mdx") && entry.name !== "index.mdx")
    )
    .map(entry => (entry.isDirectory() ? entry.name : entry.name.slice(0, -".mdx".length)))
    .sort();
}

describe("docs discovery", () => {
  it("keeps every generated top-level CLI command discoverable from authored navigation", () => {
    const cliMeta = JSON.parse(readFileSync(resolve(docsRoot, "cli/meta.json"), "utf8")) as {
      pages: string[];
    };
    const navigablePages = cliMeta.pages.filter(page => !page.startsWith("---"));

    expect(navigablePages.sort()).toEqual(["index", ...generatedCLITopLevelPages()].sort());
  });
});
