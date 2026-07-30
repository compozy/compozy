import { readdirSync, readFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import type { Folder, Node, Root } from "fumadocs-core/page-tree";
import { loader, type MetaData, type VirtualFile } from "fumadocs-core/source";
import { describe, expect, it } from "vitest";
import rootMeta from "../../content/docs/meta.json";
import { DOCS_ICONS } from "../docs-icons";
import { assertDocsTreeComplete, DOCS_GROUP_BY_FOLDER, docsGroupForUrl } from "../docs-navigation";
import { overviewPageTransformer } from "../docs-overview-tree";

function getNodeName(node: { name?: unknown }): string {
  return typeof node.name === "string" ? node.name : "";
}

function findFolder(children: Node[], id: string): Folder | undefined {
  return children.find((node): node is Folder => node.type === "folder" && node.$id === id);
}

function treeFromIds(folderIds: string[], pageUrls: string[]): Root {
  return {
    name: "Docs",
    children: [
      ...pageUrls.map(url => ({
        type: "page" as const,
        $id: `${url}.mdx`,
        name: url,
        url,
      })),
      ...folderIds.map(id => ({
        type: "folder" as const,
        $id: id,
        name: id,
        children: [],
      })),
    ],
  };
}

const ROOT_FOLDER_IDS = rootMeta.pages.filter(id => !id.startsWith("---") && id !== "index");

describe("docs tree completeness", () => {
  it("Should accept a tree where every root meta id resolves to a folder or page", () => {
    const pages = ["/docs", "/docs/how-to-use-these-docs"];
    const folders = ROOT_FOLDER_IDS.filter(id => id !== "how-to-use-these-docs");
    expect(() => assertDocsTreeComplete(treeFromIds(folders, pages))).not.toThrow();
  });

  it("Should fail the build when a listed folder is missing from the tree", () => {
    const pages = ["/docs", "/docs/how-to-use-these-docs"];
    const folders = ROOT_FOLDER_IDS.filter(id => id !== "how-to-use-these-docs" && id !== "loops");
    expect(() => assertDocsTreeComplete(treeFromIds(folders, pages))).toThrow(/loops/);
  });
});

describe("docs sidebar groups (D14)", () => {
  it("Should declare the eight journey-ordered groups in the root meta", () => {
    const groupLabels = rootMeta.pages
      .filter(id => id.startsWith("---") && id.endsWith("---"))
      .map(id => id.slice(3, -3));
    expect(groupLabels).toEqual([
      "Start here",
      "Guides & examples",
      "Core concepts",
      "Automation",
      "Compozy Network",
      "Extensibility",
      "Operations",
      "Reference",
    ]);
  });

  it("Should map every top-level folder to its sidebar group", () => {
    expect(DOCS_GROUP_BY_FOLDER.get("getting-started")).toBe("Start here");
    expect(DOCS_GROUP_BY_FOLDER.get("guides")).toBe("Guides & examples");
    expect(DOCS_GROUP_BY_FOLDER.get("sessions")).toBe("Core concepts");
    expect(DOCS_GROUP_BY_FOLDER.get("loops")).toBe("Automation");
    expect(DOCS_GROUP_BY_FOLDER.get("network")).toBe("Compozy Network");
    expect(DOCS_GROUP_BY_FOLDER.get("marketplace")).toBe("Extensibility");
    expect(DOCS_GROUP_BY_FOLDER.get("configuration")).toBe("Operations");
    expect(DOCS_GROUP_BY_FOLDER.get("cli")).toBe("Reference");
    expect(DOCS_GROUP_BY_FOLDER.get("api")).toBe("Reference");
  });

  it("Should resolve page URLs to their sidebar group with Docs as the ungrouped fallback", () => {
    expect(docsGroupForUrl("/docs")).toBe("Docs");
    expect(docsGroupForUrl("/docs/how-to-use-these-docs")).toBe("Docs");
    expect(docsGroupForUrl("/docs/sessions/lifecycle")).toBe("Core concepts");
    expect(docsGroupForUrl("/docs/network/protocol/envelope")).toBe("Compozy Network");
    expect(docsGroupForUrl("/docs/cli/session")).toBe("Reference");
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
    { baseUrl: "/docs", pageTree: { transformers: [overviewPageTransformer()] } }
  ).pageTree;
}

function childNames(folder: Folder): string[] {
  return folder.children.map(getNodeName);
}

describe("docs category overview", () => {
  it("Should expose the landing page as an Overview child when meta omits index", () => {
    const tree = buildCategoryTree([
      meta("network/meta.json", { title: "Network", pages: ["protocol-model", "threads"] }),
      page("network/index.mdx", "Network Overview", ["network"]),
      page("network/protocol-model.mdx", "Protocol Model", ["network", "protocol-model"]),
      page("network/threads.mdx", "Public Threads", ["network", "threads"]),
    ]);

    const network = findFolder(tree.children, "network");
    if (!network) throw new Error("expected network folder");

    expect(network.index?.url).toBe("/docs/network");
    expect(childNames(network)).toEqual(["Overview", "Protocol Model", "Public Threads"]);
    const overview = network.children[0];
    expect(overview).toBe(network.index);
    expect(overview.type === "page" ? overview.url : "").toBe("/docs/network");
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

    expect(loops.index?.url).toBe("/docs/loops");
    expect(childNames(loops)).toEqual([
      "Overview",
      "Catalog and detail",
      "Author",
      "Authoring loop",
    ]);
    expect(loops.children[0]).toBe(loops.index);
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
  it("Should resolve every icon referenced by docs content", () => {
    const found = new Map<string, string>();
    collectReferencedIcons(join(CONTENT_ROOT, "docs"), found);
    expect(found.size).toBeGreaterThan(0);

    const unresolved = [...found.entries()]
      .filter(([, icon]) => !(icon in DOCS_ICONS))
      .map(([reference]) => reference);
    expect(unresolved).toEqual([]);
  });
});
