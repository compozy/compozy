import { existsSync, readFileSync, readdirSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const siteRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..");
const runtimeRoot = resolve(siteRoot, "content/runtime");
const protocolRoot = resolve(siteRoot, "content/protocol");

function readRuntimeJSON<T>(...parts: string[]): T {
  return JSON.parse(readFileSync(resolve(runtimeRoot, ...parts), "utf8")) as T;
}

function readProtocolJSON<T>(...parts: string[]): T {
  return JSON.parse(readFileSync(resolve(protocolRoot, ...parts), "utf8")) as T;
}

function runtimePageExists(...parts: string[]): boolean {
  return existsSync(resolve(runtimeRoot, ...parts));
}

function generatedCLITopLevelPages(): string[] {
  return readdirSync(resolve(runtimeRoot, "cli-reference"), { withFileTypes: true })
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

describe("runtime docs discovery", () => {
  it("exposes orientation, guides, use cases, generated references, and core concepts from runtime meta", () => {
    const runtimeMeta = readRuntimeJSON<{ pages: string[] }>("meta.json");

    expect(runtimeMeta.pages).toEqual([
      "index",
      "how-to-use-these-docs",
      "migration",
      "core",
      "guides",
      "use-cases",
      "cli-reference",
      "api-reference",
    ]);
    expect(runtimePageExists("how-to-use-these-docs.mdx")).toBe(true);
    expect(runtimePageExists("migration", "index.mdx")).toBe(true);
  });

  it("keeps newly added guide and use-case sections discoverable", () => {
    const guidesMeta = readRuntimeJSON<{ pages: string[] }>("guides/meta.json");
    const useCasesMeta = readRuntimeJSON<{ pages: string[] }>("use-cases/meta.json");

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
      expect(runtimePageExists("guides", `${page}.mdx`)).toBe(true);
    }
    for (const page of useCasesMeta.pages) {
      expect(runtimePageExists("use-cases", `${page}.mdx`)).toBe(true);
    }
  });

  it("keeps resources and tools reachable from core concepts", () => {
    const coreMeta = readRuntimeJSON<{ pages: string[] }>("core/meta.json");
    const resourcesMeta = readRuntimeJSON<{ pages: string[] }>("core/resources/meta.json");
    const toolsMeta = readRuntimeJSON<{ pages: string[] }>("core/tools/meta.json");

    expect(coreMeta.pages).toContain("resources");
    expect(coreMeta.pages).toContain("tools");
    expect(resourcesMeta.pages).toEqual(["index", "bundles"]);
    expect(toolsMeta.pages).toEqual([
      "index",
      "toolsets",
      "policy-and-invocation",
      "result-artifacts",
    ]);
    for (const page of resourcesMeta.pages) {
      expect(runtimePageExists("core", "resources", `${page}.mdx`)).toBe(true);
    }
    for (const page of toolsMeta.pages) {
      expect(runtimePageExists("core", "tools", `${page}.mdx`)).toBe(true);
    }
  });

  it("keeps Memory v2 narrative pages reachable from core memory meta", () => {
    const coreMeta = readRuntimeJSON<{ pages: string[] }>("core/meta.json");
    const memoryMeta = readRuntimeJSON<{ pages: string[] }>("core/memory/meta.json");

    expect(coreMeta.pages).toContain("memory");
    expect(memoryMeta.pages).toEqual(["system", "scopes", "dream", "best-practices"]);
    for (const page of memoryMeta.pages) {
      expect(runtimePageExists("core", "memory", `${page}.mdx`)).toBe(true);
    }
  });

  it("exposes the Slice 1 memory CLI surface from the generated cli-reference meta", () => {
    const memoryMeta = readRuntimeJSON<{ pages: string[] }>("cli-reference/memory/meta.json");
    const dreamMeta = readRuntimeJSON<{ pages: string[] }>("cli-reference/memory/dream/meta.json");

    for (const page of ["index", "show", "search", "edit", "delete", "write", "history", "dream"]) {
      expect(memoryMeta.pages).toContain(page);
    }
    expect(memoryMeta.pages).not.toContain("read");
    expect(memoryMeta.pages).not.toContain("consolidate");
    expect(runtimePageExists("cli-reference", "memory", "show.mdx")).toBe(true);
    expect(runtimePageExists("cli-reference", "memory", "read.mdx")).toBe(false);

    for (const page of ["index", "trigger", "retry", "show", "status"]) {
      expect(dreamMeta.pages).toContain(page);
    }
    expect(dreamMeta.pages).not.toContain("consolidate");
    expect(runtimePageExists("cli-reference", "memory", "dream", "trigger.mdx")).toBe(true);
    expect(runtimePageExists("cli-reference", "memory", "dream", "consolidate.mdx")).toBe(false);
  });

  it("keeps every generated top-level CLI command discoverable from authored navigation", () => {
    const cliMeta = readRuntimeJSON<{ pages: string[] }>("cli-reference/meta.json");
    const navigablePages = cliMeta.pages.filter(page => !page.startsWith("---"));

    expect(navigablePages.sort()).toEqual(["index", ...generatedCLITopLevelPages()].sort());
  });

  it("exposes the memory tag in the generated api-reference meta", () => {
    const apiMeta = readRuntimeJSON<{ pages: string[] }>("api-reference/meta.json");

    expect(apiMeta.pages).toContain("memory");
    expect(runtimePageExists("api-reference", "memory.mdx")).toBe(true);
  });

  it("gives every docs category its own landing page", () => {
    // `core` is the CompozyOS tab root; lib/runtime-navigation.ts points its Overview at /runtime,
    // so it is the one category that must not own a landing page of its own.
    const exempt = new Set([resolve(runtimeRoot, "core")]);
    const missing = [...collectCategoryDirs(runtimeRoot), ...collectCategoryDirs(protocolRoot)]
      .filter(dir => !exempt.has(dir))
      .filter(dir => !existsSync(resolve(dir, "index.mdx")))
      .map(dir => dir.slice(siteRoot.length + 1));

    expect(missing).toEqual([]);
  });

  it("keeps protocol implementation status reachable from protocol meta", () => {
    const protocolMeta = readProtocolJSON<{ pages: string[] }>("meta.json");

    expect(protocolMeta.pages).toContain("implementation-status");
    expect(protocolPageExists("implementation-status.mdx")).toBe(true);
  });
});
