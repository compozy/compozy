import type { FileTreeDecoration, FileTreeDecorations, FileTreeStatus } from "@compozy/ui";

import type { FileEntry } from "../adapters/files-api";

const STATUSES: ReadonlySet<string> = new Set<FileTreeStatus>([
  "added",
  "modified",
  "deleted",
  "renamed",
  "untracked",
  "conflicted",
  "type_changed",
]);

/** Tree paths mark directories with a trailing `/`; the daemon's listing paths never do. */
export function treePath(entry: Pick<FileEntry, "path" | "kind">): string {
  return entry.kind === "directory" ? `${entry.path}/` : entry.path;
}

/** `web/src/` → `web/src` (the listing request path); the root is `""`. */
export function listingPath(directory: string): string {
  return directory.endsWith("/") ? directory.slice(0, -1) : directory;
}

/** Every ancestor directory of a root-relative file path, as listing paths, shallowest first. */
export function ancestorListings(path: string): string[] {
  const parts = path.split("/").slice(0, -1);
  return parts.map((_, index) => parts.slice(0, index + 1).join("/"));
}

/** Unknown git status renders no badge — never a guess (`_uiux.md` Truthful UI). */
export function entryDecoration(entry: FileEntry): FileTreeDecoration | null {
  const status =
    entry.git_status && STATUSES.has(entry.git_status)
      ? (entry.git_status as FileTreeStatus)
      : entry.ignored
        ? "ignored"
        : undefined;
  const decoration: FileTreeDecoration = {
    ...(status ? { status } : {}),
    ...(entry.sensitive ? { sensitive: true } : {}),
    ...(entry.symlink ? { symlink: true } : {}),
  };
  return Object.keys(decoration).length > 0 ? decoration : null;
}

/**
 * Decorations for the loaded entries. A directory the daemon marks `contains_changes` gets the
 * dot until one of its changed descendants is loaded, when the tree draws the dot itself
 * (US-001.AC-4).
 */
export function entriesDecorations(entries: readonly FileEntry[]): FileTreeDecorations {
  const decorations: Record<string, FileTreeDecoration> = {};
  const changed = entries.filter(entry => entry.git_status && STATUSES.has(entry.git_status));
  for (const entry of entries) {
    const decoration = entryDecoration(entry) ?? {};
    if (entry.kind === "directory" && entry.contains_changes) {
      const prefix = `${entry.path}/`;
      if (!changed.some(item => item.path.startsWith(prefix))) decoration.containsChanges = true;
    }
    if (Object.keys(decoration).length > 0) decorations[treePath(entry)] = decoration;
  }
  return decorations;
}
