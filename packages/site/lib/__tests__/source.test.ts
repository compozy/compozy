import { readdirSync, readFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import type { Folder, Node, Root } from "fumadocs-core/page-tree";
import { loader, type MetaData, type VirtualFile } from "fumadocs-core/source";
import { getLayoutTabs } from "fumadocs-ui/layouts/shared";
import { describe, expect, it } from "vitest";
import { DOCS_ICONS } from "../docs-icons";
import { overviewPageTransformer } from "../docs-overview-tree";
import { createRuntimeLayoutTree } from "../runtime-navigation";

function getNodeName(node: { name?: unknown }): string {
  return typeof node.name === "string" ? node.name : "";
}

function findFolder(children: Node[], id: string): Folder | undefined {
  return children.find((node): node is Folder => node.type === "folder" && node.$id === id);
}

const runtimePageTree: Root = {
  name: "Runtime",
  children: [
    {
      type: "page",
      $id: "index.mdx",
      name: "Runtime Overview",
      url: "/runtime",
    },
    {
      type: "page",
      $id: "how-to-use-these-docs.mdx",
      name: "How to Use These Docs",
      url: "/runtime/how-to-use-these-docs",
    },
    {
      type: "folder",
      $id: "core",
      name: "Core Concepts",
      root: true,
      children: [
        {
          type: "page",
          $id: "core/index.mdx",
          name: "Index",
          url: "/runtime/core",
        },
        { type: "folder", $id: "core/getting-started", name: "Getting Started", children: [] },
        { type: "folder", $id: "core/sessions", name: "Sessions", children: [] },
        { type: "folder", $id: "core/agents", name: "Agents", children: [] },
        { type: "folder", $id: "core/network", name: "Network", children: [] },
        { type: "folder", $id: "core/autonomy", name: "Autonomy", children: [] },
        { type: "folder", $id: "core/memory", name: "Memory", children: [] },
        { type: "folder", $id: "core/resources", name: "Resources", children: [] },
        { type: "folder", $id: "core/tools", name: "Tools", children: [] },
        { type: "folder", $id: "core/skills", name: "Skills", children: [] },
        { type: "folder", $id: "core/sandbox", name: "Sandbox", children: [] },
        { type: "folder", $id: "core/workspaces", name: "Workspaces", children: [] },
        { type: "folder", $id: "core/automation", name: "Automation", children: [] },
        { type: "folder", $id: "core/bridges", name: "Bridges", children: [] },
        { type: "folder", $id: "core/hooks", name: "Hooks", children: [] },
        { type: "folder", $id: "core/extensions", name: "Extensions", children: [] },
        { type: "folder", $id: "core/operations", name: "Operations", children: [] },
        { type: "folder", $id: "core/configuration", name: "Configuration", children: [] },
      ],
    },
    {
      type: "folder",
      $id: "guides",
      name: "Guides",
      children: [
        {
          type: "page",
          $id: "guides/index.mdx",
          name: "Guides Index",
          url: "/runtime/guides",
        },
        {
          type: "page",
          $id: "guides/debug.mdx",
          name: "Debug a Failed Session",
          url: "/runtime/guides/debug-a-failed-session",
        },
      ],
    },
    {
      type: "folder",
      $id: "use-cases",
      name: "Use Cases",
      children: [
        {
          type: "page",
          $id: "use-cases/index.mdx",
          name: "Use Cases Index",
          url: "/runtime/use-cases",
        },
        {
          type: "page",
          $id: "use-cases/review.mdx",
          name: "Review a Change",
          url: "/runtime/use-cases/review-a-change",
        },
      ],
    },
    {
      type: "folder",
      $id: "migration",
      name: "Migration",
      children: [
        {
          type: "page",
          $id: "migration/index.mdx",
          name: "Migrate from v0.2",
          url: "/runtime/migration",
        },
      ],
    },
    {
      type: "folder",
      $id: "cli-reference",
      name: "CLI Reference",
      root: true,
      children: [
        {
          type: "page",
          $id: "cli-reference/index.mdx",
          name: "Index",
          url: "/runtime/cli-reference",
        },
        {
          type: "page",
          $id: "cli-reference/compozy.mdx",
          name: "Compozy",
          url: "/runtime/cli-reference/compozy",
        },
      ],
    },
    {
      type: "folder",
      $id: "api-reference",
      name: "API Reference",
      root: true,
      children: [
        {
          type: "page",
          $id: "api-reference/index.mdx",
          name: "Index",
          url: "/runtime/api-reference",
        },
      ],
    },
  ],
};

describe("runtime navigation tree", () => {
  it("Should produce a single unified core sidebar with Overview, How to Use, and grouped sections", () => {
    const layoutTree = createRuntimeLayoutTree(runtimePageTree);
    const core = findFolder(layoutTree.children, "core");

    expect(core).toBeDefined();
    if (!core) throw new Error("expected unified core folder");
    expect(core.root).toBe(true);
    expect(getNodeName(core)).toBe("CompozyOS");

    const ids = core.children.map(child => child.$id ?? getNodeName(child));
    expect(ids[0]).toBe("runtime-overview");
    expect(ids[1]).toBe("how-to-use-these-docs.mdx");
    expect(ids[2]).toBe("core/getting-started");

    const sectionLabels = core.children
      .filter(child => child.type === "separator")
      .map(child => getNodeName(child));
    expect(sectionLabels).toEqual([
      "System foundations",
      "Tools & automation",
      "Workspace & isolation",
      "Operations & settings",
      "Guides & scenarios",
    ]);
  });

  it("Should group runtime surfaces by reader intent", () => {
    const layoutTree = createRuntimeLayoutTree(runtimePageTree);
    const core = findFolder(layoutTree.children, "core");
    if (!core) throw new Error("expected unified core folder");

    const labelByChildId = new Map<string, string>();
    let currentLabel = "";
    for (const child of core.children) {
      if (child.type === "separator") {
        currentLabel = getNodeName(child);
        continue;
      }
      if (child.$id) labelByChildId.set(child.$id, currentLabel);
    }

    expect(labelByChildId.get("core/automation")).toBe("Tools & automation");
    expect(labelByChildId.get("core/bridges")).toBe("Tools & automation");
    expect(labelByChildId.get("core/operations")).toBe("Operations & settings");
    expect(labelByChildId.get("core/configuration")).toBe("Operations & settings");
    expect(labelByChildId.get("core/extensions")).toBe("Operations & settings");
    expect(labelByChildId.get("core/hooks")).toBe("Operations & settings");
  });

  it("Should nest guides and use-cases as collapsible folders under the Learn section", () => {
    const layoutTree = createRuntimeLayoutTree(runtimePageTree);
    const core = findFolder(layoutTree.children, "core");
    if (!core) throw new Error("expected unified core folder");

    const guides = findFolder(core.children, "guides");
    const useCases = findFolder(core.children, "use-cases");
    expect(guides).toBeDefined();
    expect(useCases).toBeDefined();
    if (!guides || !useCases) throw new Error("expected nested learn folders");

    const guideUrls = guides.children
      .filter(child => child.type === "page")
      .map(page => (page.type === "page" ? page.url : ""));
    const useCaseUrls = useCases.children
      .filter(child => child.type === "page")
      .map(page => (page.type === "page" ? page.url : ""));

    expect(guideUrls).toContain("/runtime/guides/debug-a-failed-session");
    expect(useCaseUrls).toContain("/runtime/use-cases/review-a-change");
  });

  it("Should retain migration while nesting guides and use cases", () => {
    const layoutTree = createRuntimeLayoutTree(runtimePageTree);
    const outerIds = layoutTree.children
      .filter(child => child.type === "folder")
      .map(folder => folder.$id);

    expect(outerIds).toEqual(["core", "migration", "cli-reference", "api-reference"]);
    const migration = findFolder(layoutTree.children, "migration");
    expect(
      migration?.children.some(child => child.type === "page" && child.url === "/runtime/migration")
    ).toBe(true);
  });

  it("Should route the CompozyOS tab to /runtime instead of /runtime/core", () => {
    const layoutTree = createRuntimeLayoutTree(runtimePageTree);
    const tabs = getLayoutTabs(layoutTree);
    const tabUrls = tabs.map(tab => tab.url);
    expect(tabUrls).toContain("/runtime");
    expect(tabUrls).toContain("/runtime/cli-reference");
    expect(tabUrls).toContain("/runtime/api-reference");
    expect(tabUrls).not.toContain("/runtime/core");
    expect(tabUrls).not.toContain("/runtime/guides");
    expect(tabUrls).not.toContain("/runtime/use-cases");
  });
});

function page(path: string, title: string, slugs: string[]): VirtualFile {
  return { type: "page", path, slugs, data: { title } };
}

function meta(path: string, data: MetaData): VirtualFile {
  return { type: "meta", path, data };
}

function buildCategoryTree(files: VirtualFile[]): Root {
  return loader(
    { files },
    { baseUrl: "/runtime", pageTree: { transformers: [overviewPageTransformer()] } }
  ).pageTree;
}

function childNames(folder: Folder): string[] {
  return folder.children.map(getNodeName);
}

describe("docs category overview", () => {
  it("Should expose the landing page as an Overview child when meta omits index", () => {
    const tree = buildCategoryTree([
      meta("network/meta.json", { title: "Network", pages: ["protocol", "threads"] }),
      page("network/index.mdx", "Network Overview", ["network"]),
      page("network/protocol.mdx", "Protocol Model", ["network", "protocol"]),
      page("network/threads.mdx", "Public Threads", ["network", "threads"]),
    ]);

    const network = findFolder(tree.children, "network");
    if (!network) throw new Error("expected network folder");

    expect(network.index).toBeUndefined();
    expect(childNames(network)).toEqual(["Overview", "Protocol Model", "Public Threads"]);
    const overview = network.children[0];
    expect(overview.type === "page" ? overview.url : "").toBe("/runtime/network");
  });

  it("Should relabel and hoist the landing page when meta already lists index", () => {
    const tree = buildCategoryTree([
      meta("loops/meta.json", {
        title: "Loops",
        pages: ["catalog", "---Author---", "index", "authoring"],
      }),
      page("loops/index.mdx", "Loops", ["loops"]),
      page("loops/catalog.mdx", "Catalog and detail", ["loops", "catalog"]),
      page("loops/authoring.mdx", "Authoring loop", ["loops", "authoring"]),
    ]);

    const loops = findFolder(tree.children, "loops");
    if (!loops) throw new Error("expected loops folder");

    expect(loops.index).toBeUndefined();
    expect(childNames(loops)).toEqual([
      "Overview",
      "Catalog and detail",
      "Author",
      "Authoring loop",
    ]);
  });

  it("Should leave a category without a landing page untouched", () => {
    const tree = buildCategoryTree([
      meta("sandbox/meta.json", { title: "Sandbox", pages: ["profiles"] }),
      page("sandbox/profiles.mdx", "Profiles", ["sandbox", "profiles"]),
    ]);

    const sandbox = findFolder(tree.children, "sandbox");
    if (!sandbox) throw new Error("expected sandbox folder");

    expect(sandbox.index).toBeUndefined();
    expect(childNames(sandbox)).toEqual(["Profiles"]);
  });
});

const CONTENT_ROOT = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..", "content");

function collectReferencedIcons(dir: string, found: Map<string, string>): void {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const entryPath = join(dir, entry.name);
    if (entry.isDirectory()) {
      collectReferencedIcons(entryPath, found);
      continue;
    }
    if (entry.name === "meta.json") {
      const icon = (JSON.parse(readFileSync(entryPath, "utf8")) as { icon?: string }).icon;
      if (icon) found.set(`${entryPath}: ${icon}`, icon);
      continue;
    }
    if (entry.name.endsWith(".mdx")) {
      const frontmatter = readFileSync(entryPath, "utf8").match(/^---\n([\s\S]*?)\n---/);
      const icon = frontmatter?.[1].match(/^icon:\s*(\S+)\s*$/m)?.[1];
      if (icon) found.set(`${entryPath}: ${icon}`, icon);
    }
  }
}

describe("docs icon registry", () => {
  it("Should resolve every icon referenced by runtime and protocol content", () => {
    const found = new Map<string, string>();
    for (const tree of ["runtime", "protocol"]) {
      collectReferencedIcons(join(CONTENT_ROOT, tree), found);
    }
    expect(found.size).toBeGreaterThan(0);

    const unresolved = [...found.entries()]
      .filter(([, icon]) => !(icon in DOCS_ICONS))
      .map(([reference]) => reference);
    expect(unresolved).toEqual([]);
  });
});
