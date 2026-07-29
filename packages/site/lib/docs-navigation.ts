import type { Root } from "fumadocs-core/page-tree";
import rootMeta from "@/content/docs/meta.json";

type ApiSection = { label: string; ids: string[] };

/** Section grouping consumed by `scripts/generate-openapi.ts` when it writes the api meta.json. */
export const API_SECTIONS: ApiSection[] = [
  { label: "Workspace", ids: ["sessions", "workspaces", "agents", "memory", "skills"] },
  {
    label: "Tools & automation",
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

function isSeparatorId(id: string): boolean {
  return id.startsWith("---") && id.endsWith("---");
}

function separatorLabel(id: string): string {
  return id.slice(3, -3);
}

/** Fallback group for the ungrouped ids ahead of the first separator (`index`, `how-to-use-…`). */
export const DOCS_ROOT_GROUP = "Docs";

function buildGroupByFolder(): ReadonlyMap<string, string> {
  const groups = new Map<string, string>();
  let current = DOCS_ROOT_GROUP;
  for (const id of rootMeta.pages) {
    if (isSeparatorId(id)) {
      current = separatorLabel(id);
      continue;
    }
    groups.set(id, current);
  }
  return groups;
}

/**
 * D14 sidebar group of each top-level folder/page id, derived from the root meta.json so search
 * grouping can never drift from the sidebar.
 */
export const DOCS_GROUP_BY_FOLDER: ReadonlyMap<string, string> = buildGroupByFolder();

/** Resolves the D14 group for a built docs URL (`/docs/sessions/lifecycle` → `Core concepts`). */
export function docsGroupForUrl(url: string): string {
  const [head] = url.replace(/^\/docs\/?/, "").split("/");
  if (!head) return DOCS_ROOT_GROUP;
  return DOCS_GROUP_BY_FOLDER.get(head) ?? DOCS_ROOT_GROUP;
}

/**
 * Every non-separator id in the root `content/docs/meta.json` must resolve to a top-level folder or
 * page of the built tree. The retired code-synthesized navigation dropped unresolved folders
 * silently (`if (!coreFolder) return pageTree`); this assertion turns that failure mode into a
 * build error.
 */
export function assertDocsTreeComplete(tree: Root): void {
  const resolved = new Set<string>();
  for (const node of tree.children) {
    if (node.type === "folder" && node.$id) {
      resolved.add(node.$id);
      continue;
    }
    if (node.type === "page") {
      const path = node.url.replace(/^\/docs\/?/, "").replace(/\/$/, "");
      resolved.add(path === "" ? "index" : path);
    }
  }

  const missing = rootMeta.pages.filter(id => !isSeparatorId(id) && !resolved.has(id));
  if (missing.length > 0) {
    throw new Error(
      `content/docs/meta.json ids missing from the built page tree: ${missing.join(", ")}. ` +
        "Every listed id must be a top-level folder or page under content/docs."
    );
  }
}
