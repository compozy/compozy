import type { Folder, Node, Root } from "fumadocs-core/page-tree";
import { getLayoutTabs } from "fumadocs-ui/layouts/shared";
import { describe, expect, it } from "vitest";
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
          $id: "cli-reference/agh.mdx",
          name: "Agh",
          url: "/runtime/cli-reference/agh",
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
    expect(getNodeName(core)).toBe("Core Concepts");

    const ids = core.children.map(child => child.$id ?? getNodeName(child));
    expect(ids[0]).toBe("runtime-overview");
    expect(ids[1]).toBe("how-to-use-these-docs.mdx");
    expect(ids[2]).toBe("core/getting-started");

    const sectionLabels = core.children
      .filter(child => child.type === "separator")
      .map(child => getNodeName(child));
    expect(sectionLabels).toEqual(["Foundation", "Capabilities", "Workspace", "Settings", "Learn"]);
  });

  it("Should place automation, bridges under Capabilities and operations, configuration, extensions, hooks under Settings", () => {
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

    expect(labelByChildId.get("core/automation")).toBe("Capabilities");
    expect(labelByChildId.get("core/bridges")).toBe("Capabilities");
    expect(labelByChildId.get("core/operations")).toBe("Settings");
    expect(labelByChildId.get("core/configuration")).toBe("Settings");
    expect(labelByChildId.get("core/extensions")).toBe("Settings");
    expect(labelByChildId.get("core/hooks")).toBe("Settings");
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

  it("Should drop guides and use-cases as outer-level folders while keeping cli + api refs", () => {
    const layoutTree = createRuntimeLayoutTree(runtimePageTree);
    const outerIds = layoutTree.children
      .filter(child => child.type === "folder")
      .map(folder => folder.$id);

    expect(outerIds).toEqual(["core", "cli-reference", "api-reference"]);
  });

  it("Should expose 3 tabs and route Core Concepts to /runtime instead of /runtime/core", () => {
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

  it("Should keep the sandbox index as the folder target instead of a duplicate child page", async () => {
    const metadata = (await import("../../content/runtime/core/sandbox/meta.json")) as {
      default: {
        pages?: unknown;
      };
    };
    const pages = metadata.default.pages;

    expect(Array.isArray(pages) && pages.includes("index")).toBe(false);
  });
});
