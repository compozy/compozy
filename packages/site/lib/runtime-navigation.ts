import { createElement } from "react";
import { Book } from "lucide-react";
import type { Folder, Item, Node, Root, Separator } from "fumadocs-core/page-tree";

const CORE_FOLDER_ID = "core";
const GUIDES_FOLDER_ID = "guides";
const USE_CASES_FOLDER_ID = "use-cases";
const CLI_REF_FOLDER_ID = "cli-reference";
const API_REF_FOLDER_ID = "api-reference";
const HOW_TO_USE_URL = "/runtime/how-to-use-these-docs";
const OVERVIEW_PAGE_ID = "runtime-overview";

function isFolder(node: Node): node is Folder {
  return node.type === "folder";
}

function isPage(node: Node): node is Item {
  return node.type === "page";
}

function findRootFolder(pageTree: Root, id: string): Folder | undefined {
  return pageTree.children.find((node): node is Folder => isFolder(node) && node.$id === id);
}

function findPage(nodes: Node[], predicate: (item: Item) => boolean): Item | undefined {
  for (const node of nodes) {
    if (isPage(node) && predicate(node)) return node;
  }
  return undefined;
}

function buildOverviewPage(): Item {
  return {
    type: "page",
    $id: OVERVIEW_PAGE_ID,
    name: "Overview",
    url: "/runtime",
    icon: createElement(Book),
  };
}

function buildSeparator(id: string, name: string): Separator {
  return { type: "separator", $id: id, name };
}

type CoreSection = { label: string; ids: string[] };

const CORE_SECTIONS: CoreSection[] = [
  { label: "Foundation", ids: ["sessions", "agents", "network", "autonomy", "memory"] },
  {
    label: "Capabilities",
    ids: ["marketplace", "tools", "skills", "resources", "automation", "loops", "bridges"],
  },
  { label: "Workspace", ids: ["sandbox", "workspaces"] },
  { label: "Settings", ids: ["operations", "configuration", "extensions", "hooks"] },
];

export const API_SECTIONS: CoreSection[] = [
  { label: "Workspace", ids: ["sessions", "workspaces", "agents", "memory", "skills"] },
  {
    label: "Capabilities",
    ids: [
      "marketplace",
      "tools",
      "toolsets",
      "resources",
      "bundles",
      "automation",
      "loops",
      "bridges",
      "notifications",
    ],
  },
  { label: "Network", ids: ["network", "observe", "hooks"] },
  {
    label: "Operations",
    ids: [
      "daemon",
      "diagnostics",
      "onboarding",
      "filesystem",
      "logs",
      "settings",
      "support",
      "providers",
      "extensions",
      "vault",
      "agent",
      "tasks",
      "openai",
      "roles",
    ],
  },
];

function indexCoreChildren(coreFolder: Folder): Map<string, Node> {
  const lookup = new Map<string, Node>();
  for (const child of coreFolder.children) {
    if (!child.$id) continue;
    const slug = child.$id.startsWith(`${CORE_FOLDER_ID}/`)
      ? child.$id.slice(CORE_FOLDER_ID.length + 1)
      : child.$id;
    lookup.set(slug, child);
  }
  return lookup;
}

function buildUnifiedCore(pageTree: Root): Folder | undefined {
  const coreFolder = findRootFolder(pageTree, CORE_FOLDER_ID);
  if (!coreFolder) return undefined;

  const guidesFolder = findRootFolder(pageTree, GUIDES_FOLDER_ID);
  const useCasesFolder = findRootFolder(pageTree, USE_CASES_FOLDER_ID);
  const howToUse = findPage(pageTree.children, item => item.url === HOW_TO_USE_URL);

  const lookup = indexCoreChildren(coreFolder);
  const gettingStarted = lookup.get("getting-started");

  const learnFolders: Folder[] = [];
  if (guidesFolder) learnFolders.push(guidesFolder);
  if (useCasesFolder) learnFolders.push(useCasesFolder);

  const overview = buildOverviewPage();
  const children: Node[] = [overview];
  if (howToUse) children.push(howToUse);
  if (gettingStarted) children.push(gettingStarted);

  for (const section of CORE_SECTIONS) {
    const items = section.ids.map(id => lookup.get(id)).filter((node): node is Node => !!node);
    if (items.length === 0) continue;
    const sepId = `sep-${section.label.toLowerCase()}`;
    children.push(buildSeparator(sepId, section.label), ...items);
  }

  if (learnFolders.length > 0) {
    children.push(buildSeparator("sep-learn", "Learn"), ...learnFolders);
  }

  return {
    ...coreFolder,
    name: "Core Concepts",
    root: true,
    index: overview,
    children,
  };
}

export function createRuntimeLayoutTree(pageTree: Root): Root {
  const unifiedCore = buildUnifiedCore(pageTree);
  if (!unifiedCore) return pageTree;

  const newChildren: Node[] = [unifiedCore];
  for (const id of [CLI_REF_FOLDER_ID, API_REF_FOLDER_ID]) {
    const folder = findRootFolder(pageTree, id);
    if (folder) newChildren.push(folder);
  }

  return {
    ...pageTree,
    $id: `${pageTree.$id ?? "runtime"}-layout`,
    children: newChildren,
  };
}
