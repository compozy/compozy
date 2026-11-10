import { readdirSync, readFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import type { Folder, Node, Root } from "fumadocs-core/page-tree";
import { loader, type MetaData, type VirtualFile } from "fumadocs-core/source";
import { describe, expect, it } from "vitest";
import rootMeta from "../../content/docs/meta.json";
import { DOCS_ICONS } from "../docs-icons";
import {
  assertDocsTreeComplete,
  collectPageUrls,
  currentSectionId,
  DOCS_GROUP_BY_FOLDER,
  docsGroupForUrl,
  docsSections,
  sectionOf,
} from "../docs-navigation";
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
  it("Should map every top-level folder to its sidebar group", () => {
    expect(DOCS_GROUP_BY_FOLDER.get("getting-started")).toBe("Start here");
    expect(DOCS_GROUP_BY_FOLDER.get("guides")).toBe("Guides & examples");
    expect(DOCS_GROUP_BY_FOLDER.get("sessions")).toBe("Core concepts");
    expect(DOCS_GROUP_BY_FOLDER.get("loops")).toBe("Automation");
    expect(DOCS_GROUP_BY_FOLDER.get("marketplace")).toBe("Extensibility");
    expect(DOCS_GROUP_BY_FOLDER.get("configuration")).toBe("Operations");
    expect(DOCS_GROUP_BY_FOLDER.get("cli")).toBe("Reference");
    expect(DOCS_GROUP_BY_FOLDER.get("api")).toBe("Reference");
  });

  it("Should resolve page URLs to their sidebar group with Docs as the ungrouped fallback", () => {
    expect(docsGroupForUrl("/docs")).toBe("Docs");
    expect(docsGroupForUrl("/docs/how-to-use-these-docs")).toBe("Docs");
    expect(docsGroupForUrl("/docs/sessions/lifecycle")).toBe("Core concepts");
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
      meta("sessions/meta.json", { title: "Sessions", pages: ["lifecycle", "resume"] }),
      page("sessions/index.mdx", "Sessions Overview", ["sessions"]),
      page("sessions/lifecycle.mdx", "Lifecycle", ["sessions", "lifecycle"]),
      page("sessions/resume.mdx", "Resume", ["sessions", "resume"]),
    ]);

    const sessions = findFolder(tree.children, "sessions");
    if (!sessions) throw new Error("expected sessions folder");

    expect(sessions.index?.url).toBe("/docs/sessions");
    expect(childNames(sessions)).toEqual(["Overview", "Lifecycle", "Resume"]);
    const overview = sessions.children[0];
    expect(overview).toBe(sessions.index);
    expect(overview.type === "page" ? overview.url : "").toBe("/docs/sessions");
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
      meta("profiles/meta.json", { title: "Profiles", pages: ["profiles"] }),
      page("profiles/profiles.mdx", "Profiles", ["profiles", "profiles"]),
    ]);

    const profiles = findFolder(tree.children, "profiles");
    if (!profiles) throw new Error("expected profiles folder");

    expect(profiles.index).toBeUndefined();
    expect(childNames(profiles)).toEqual(["Profiles"]);
  });
});

// UT-007: the section tabs split the root tree on its labeled separators.
describe("docs sections", () => {
  function sectionedTree(): Root {
    return buildCategoryTree([
      meta("meta.json", {
        pages: [
          "index",
          "intro",
          "---Start here---",
          "getting-started",
          "migration",
          "---Reference---",
          "cli",
        ],
      }),
      page("index.mdx", "Docs", []),
      page("intro.mdx", "How to use these docs", ["intro"]),
      meta("getting-started/meta.json", { title: "Getting Started", pages: ["install", "quick"] }),
      page("getting-started/index.mdx", "Getting Started", ["getting-started"]),
      page("getting-started/install.mdx", "Installation", ["getting-started", "install"]),
      page("getting-started/quick.mdx", "Quick Start", ["getting-started", "quick"]),
      page("migration.mdx", "Migration", ["migration"]),
      meta("cli/meta.json", { title: "CLI", pages: ["---Sessions---", "session"] }),
      page("cli/index.mdx", "CLI", ["cli"]),
      meta("cli/session/meta.json", { title: "Session", pages: ["prompt"] }),
      page("cli/session/index.mdx", "compozy session", ["cli", "session"]),
      page("cli/session/prompt.mdx", "compozy session prompt", ["cli", "session", "prompt"]),
    ]);
  }

  it("Should split on separators with distinct page counts, folder landing pages counted once", () => {
    const sections = docsSections(sectionedTree());
    expect(sections.map(section => [section.id, section.label, section.count])).toEqual([
      ["start-here", "Start here", 4],
      ["reference", "Reference", 3],
    ]);
  });

  it("Should place every sectioned page in exactly one section and leave the intro pages out", () => {
    const tree = sectionedTree();
    const sections = docsSections(tree);
    const seen = new Map<string, number>();
    for (const section of sections) {
      for (const url of collectPageUrls(section.items)) seen.set(url, (seen.get(url) ?? 0) + 1);
    }
    expect([...seen.values()].every(times => times === 1)).toBe(true);
    expect(seen.size).toBe(sections.reduce((sum, section) => sum + section.count, 0));

    expect(sectionOf(tree, "/docs/getting-started/quick/")?.label).toBe("Start here");
    expect(sectionOf(tree, "/docs/migration")?.label).toBe("Start here");
    expect(sectionOf(tree, "/docs/cli/session/prompt")?.label).toBe("Reference");
    expect(sectionOf(tree, "/docs")).toBeNull();
    expect(sectionOf(tree, "/docs/intro")).toBeNull();
  });

  it("Should match a pathname to the section owning its top-level folder", () => {
    const tabs = [
      { id: "start-here", heads: ["getting-started", "migration"] },
      { id: "reference", heads: ["cli"] },
    ];
    expect(currentSectionId(tabs, "/docs/cli/session/prompt/")).toBe("reference");
    expect(currentSectionId(tabs, "/docs/migration")).toBe("start-here");
    expect(currentSectionId(tabs, "/docs/")).toBeNull();
    expect(currentSectionId(tabs, "/docs/intro")).toBeNull();
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
