import { existsSync, readFileSync, readdirSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const siteRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..");
const docsRoot = resolve(siteRoot, "content/docs");
const protocolRoot = resolve(siteRoot, "content/docs/network/protocol");

function readDocsJSON<T>(...parts: string[]): T {
  return JSON.parse(readFileSync(resolve(docsRoot, ...parts), "utf8")) as T;
}

function readProtocolJSON<T>(...parts: string[]): T {
  return JSON.parse(readFileSync(resolve(protocolRoot, ...parts), "utf8")) as T;
}

function docsPageExists(...parts: string[]): boolean {
  return existsSync(resolve(docsRoot, ...parts));
}

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

function protocolPageExists(...parts: string[]): boolean {
  return existsSync(resolve(protocolRoot, ...parts));
}

/** Every directory below a collection root that owns a meta.json renders as a sidebar category. */
function collectCategoryDirs(root: string): string[] {
  const found: string[] = [];
  const walk = (dir: string) => {
    for (const entry of readdirSync(dir, { withFileTypes: true })) {
      if (!entry.isDirectory()) continue;
      const child = resolve(dir, entry.name);
      if (existsSync(resolve(child, "meta.json"))) found.push(child);
      walk(child);
    }
  };
  walk(root);
  return found;
}

describe("docs discovery", () => {
  it("declares the journey-ordered single tree in the root docs meta (spec §5.2)", () => {
    const rootMeta = readDocsJSON<{ pages: string[] }>("meta.json");

    expect(rootMeta.pages).toEqual([
      "index",
      "how-to-use-these-docs",
      "---Start here---",
      "getting-started",
      "migration",
      "---Guides & examples---",
      "guides",
      "use-cases",
      "---Core concepts---",
      "sessions",
      "agents",
      "autonomy",
      "memory",
      "workspaces",
      "sandbox",
      "---Automation---",
      "automation",
      "loops",
      "bridges",
      "---Compozy Network---",
      "network",
      "---Extensibility---",
      "marketplace",
      "skills",
      "tools",
      "resources",
      "extensions",
      "hooks",
      "---Operations---",
      "operations",
      "configuration",
      "---Reference---",
      "cli",
      "api",
    ]);
    expect(docsPageExists("how-to-use-these-docs.mdx")).toBe(true);
    expect(docsPageExists("migration", "index.mdx")).toBe(true);
  });

  it("keeps the reference folders out of tab mode now that the tree is unified (D3)", () => {
    const cliMeta = readDocsJSON<{ root?: boolean }>("cli/meta.json");
    const apiMeta = readDocsJSON<{ root?: boolean }>("api/meta.json");

    expect(cliMeta.root).toBeUndefined();
    expect(apiMeta.root).toBeUndefined();
  });

  it("keeps newly added guide and use-case sections discoverable", () => {
    const guidesMeta = readDocsJSON<{ pages: string[] }>("guides/meta.json");
    const useCasesMeta = readDocsJSON<{ pages: string[] }>("use-cases/meta.json");

    expect(guidesMeta.pages).toEqual([
      "index",
      "build-your-first-extension",
      "choose-an-operator-surface",
      "debug-a-failed-session",
      "coordinate-agents-over-network",
    ]);
    expect(useCasesMeta.pages).toEqual([
      "index",
      "prepare-a-project-workspace",
      "review-a-change",
      "release-readiness-sweep",
      "handoff-between-agents",
    ]);

    for (const page of guidesMeta.pages) {
      expect(docsPageExists("guides", `${page}.mdx`)).toBe(true);
    }
    for (const page of useCasesMeta.pages) {
      expect(docsPageExists("use-cases", `${page}.mdx`)).toBe(true);
    }
  });

  it("keeps resources and tools reachable as top-level concept folders", () => {
    const rootMeta = readDocsJSON<{ pages: string[] }>("meta.json");
    const resourcesMeta = readDocsJSON<{ pages: string[] }>("resources/meta.json");
    const toolsMeta = readDocsJSON<{ pages: string[] }>("tools/meta.json");

    expect(rootMeta.pages).toContain("resources");
    expect(rootMeta.pages).toContain("tools");
    expect(resourcesMeta.pages).toEqual(["index", "bundles"]);
    expect(toolsMeta.pages).toEqual([
      "index",
      "toolsets",
      "policy-and-invocation",
      "result-artifacts",
    ]);
    for (const page of resourcesMeta.pages) {
      expect(docsPageExists("resources", `${page}.mdx`)).toBe(true);
    }
    for (const page of toolsMeta.pages) {
      expect(docsPageExists("tools", `${page}.mdx`)).toBe(true);
    }
  });

  it("keeps Memory v2 narrative pages reachable from the memory meta", () => {
    const rootMeta = readDocsJSON<{ pages: string[] }>("meta.json");
    const memoryMeta = readDocsJSON<{ pages: string[] }>("memory/meta.json");

    expect(rootMeta.pages).toContain("memory");
    expect(memoryMeta.pages).toEqual(["system", "scopes", "dream", "best-practices"]);
    for (const page of memoryMeta.pages) {
      expect(docsPageExists("memory", `${page}.mdx`)).toBe(true);
    }
  });

  it("exposes the Slice 1 memory CLI surface from the generated cli meta", () => {
    const memoryMeta = readDocsJSON<{ pages: string[] }>("cli/memory/meta.json");
    const dreamMeta = readDocsJSON<{ pages: string[] }>("cli/memory/dream/meta.json");

    for (const page of ["index", "show", "search", "edit", "delete", "write", "history", "dream"]) {
      expect(memoryMeta.pages).toContain(page);
    }
    expect(memoryMeta.pages).not.toContain("read");
    expect(memoryMeta.pages).not.toContain("consolidate");
    expect(docsPageExists("cli", "memory", "show.mdx")).toBe(true);
    expect(docsPageExists("cli", "memory", "read.mdx")).toBe(false);

    for (const page of ["index", "trigger", "retry", "show", "status"]) {
      expect(dreamMeta.pages).toContain(page);
    }
    expect(dreamMeta.pages).not.toContain("consolidate");
    expect(docsPageExists("cli", "memory", "dream", "trigger.mdx")).toBe(true);
    expect(docsPageExists("cli", "memory", "dream", "consolidate.mdx")).toBe(false);
  });

  it("keeps every generated top-level CLI command discoverable from authored navigation", () => {
    const cliMeta = readDocsJSON<{ pages: string[] }>("cli/meta.json");
    const navigablePages = cliMeta.pages.filter(page => !page.startsWith("---"));

    expect(navigablePages.sort()).toEqual(["index", ...generatedCLITopLevelPages()].sort());
  });

  it("exposes the memory tag in the generated api meta", () => {
    const apiMeta = readDocsJSON<{ pages: string[] }>("api/meta.json");

    expect(apiMeta.pages).toContain("memory");
    expect(docsPageExists("api", "memory.mdx")).toBe(true);
  });

  it("gives every docs category its own landing page", () => {
    const missing = collectCategoryDirs(docsRoot)
      .filter(dir => !existsSync(resolve(dir, "index.mdx")))
      .map(dir => dir.slice(siteRoot.length + 1));

    expect(missing).toEqual([]);
  });

  it("nests the protocol spec under network with its groups and status page intact", () => {
    const networkMeta = readDocsJSON<{ pages: string[] }>("network/meta.json");
    const protocolMeta = readProtocolJSON<{ title?: string; pages: string[] }>("meta.json");

    expect(networkMeta.pages).toContain("protocol-model");
    expect(networkMeta.pages.at(-1)).toBe("protocol");
    expect(protocolMeta.title).toBe("Protocol spec (compozy-network/v0)");
    expect(protocolMeta.pages).toContain("implementation-status");
    expect(protocolPageExists("implementation-status.mdx")).toBe(true);
    expect(docsPageExists("network", "protocol-model.mdx")).toBe(true);
  });
});
