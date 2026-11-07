import type {
  FileTreeBatchOperation,
  FileTreeRowDecoration,
  GitStatus,
  GitStatusEntry,
} from "@pierre/trees";

/**
 * Change vocabulary a tree row can carry. It is the project-files status set
 * plus `ignored`; Pierre only knows six of these, so the rest map onto the
 * closest built-in lane or onto the custom decoration lane.
 */
export type FileTreeStatus =
  | "added"
  | "modified"
  | "deleted"
  | "renamed"
  | "untracked"
  | "conflicted"
  | "type_changed"
  | "ignored";

export interface FileTreeDecoration {
  /** Unknown status means no badge — never a guess. */
  status?: FileTreeStatus;
  additions?: number;
  deletions?: number;
  sensitive?: boolean;
  symlink?: boolean;
  /**
   * A directory whose descendants changed but are not loaded yet. Pierre draws its own dot once
   * a changed descendant is in the tree, so set this only while none is.
   */
  containsChanges?: boolean;
}

interface DecorationPart {
  text: string;
  color?: string;
}

export type FileTreeDecorations = Readonly<Record<string, FileTreeDecoration>>;

export const TREE_SPINNER_ICON = "compozy-tree-spinner";
/** The same arc without motion, for reduced-motion users. */
export const TREE_SPINNER_STATIC_ICON = "compozy-tree-spinner-static";
export const TREE_SYMLINK_ICON = "compozy-tree-symlink";
const TREE_LOCK_ICON = "file-tree-icon-lock";
/**
 * Pierre marks a directory "contains changes" only from descendant status entries, and keeps
 * entries for paths it has not loaded. An unloaded changed directory gets one entry under this
 * name, so its dot is Pierre's own dot in the git lane. The name starts with U+2063 (invisible
 * separator) so it cannot collide with a real file in practice, and it never becomes a row.
 */
export const UNLOADED_CHANGES_ENTRY = "\u2063compozy-unloaded-changes";

// Custom decoration glyphs live in the tree's own shadow root, so they ship as
// a sprite sheet instead of React icons. The spinner animates through SMIL so
// the bridge never needs `unsafeCSS` for motion.
export const TREE_SPRITE_SHEET = [
  '<svg xmlns="http://www.w3.org/2000/svg" style="display:none">',
  `<symbol id="${TREE_SPINNER_ICON}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">`,
  '<path d="M21 12a9 9 0 1 1-6.219-8.56">',
  '<animateTransform attributeName="transform" type="rotate" from="0 12 12" to="360 12 12" dur="0.8s" repeatCount="indefinite"/>',
  "</path></symbol>",
  `<symbol id="${TREE_SPINNER_STATIC_ICON}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">`,
  '<path d="M21 12a9 9 0 1 1-6.219-8.56"/>',
  "</symbol>",
  `<symbol id="${TREE_SYMLINK_ICON}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">`,
  '<path d="M9 17H7A5 5 0 0 1 7 7h2"/><path d="M15 7h2a5 5 0 1 1 0 10h-2"/><path d="M8 12h8"/>',
  "</symbol></svg>",
].join("");

const PIERRE_STATUS: Record<FileTreeStatus, GitStatus | null> = {
  added: "added",
  modified: "modified",
  deleted: "deleted",
  renamed: "renamed",
  untracked: "untracked",
  ignored: "ignored",
  type_changed: "modified",
  // Conflicts get their own destructive "C" mark in the decoration lane.
  conflicted: null,
};

export function isDirectoryPath(path: string): boolean {
  return path.endsWith("/");
}

/** `src/a/b.ts` → `src/a/`; a top-level entry has no parent (`""`). */
export function parentDirectory(path: string): string {
  const trimmed = isDirectoryPath(path) ? path.slice(0, -1) : path;
  const slash = trimmed.lastIndexOf("/");
  return slash < 0 ? "" : trimmed.slice(0, slash + 1);
}

/** Every ancestor directory of `path`, shallowest first. */
export function ancestorDirectories(path: string): string[] {
  const ancestors: string[] = [];
  let parent = parentDirectory(path);
  while (parent) {
    ancestors.unshift(parent);
    parent = parentDirectory(parent);
  }
  return ancestors;
}

function depth(path: string): number {
  let count = 0;
  for (const char of path) if (char === "/") count += 1;
  return isDirectoryPath(path) ? count - 1 : count;
}

/**
 * Incremental model update from the previous path set to the next list:
 * removals deepest-first are unnecessary because a recursive remove of the
 * shallowest path covers its subtree, and additions go shallowest-first so a
 * parent always exists before its children.
 */
export function planPathSync(
  known: ReadonlySet<string>,
  next: readonly string[]
): FileTreeBatchOperation[] {
  const nextSet = new Set(next);
  const removed = [...known].filter(path => !nextSet.has(path));
  const removedDirs = removed.filter(isDirectoryPath);
  const operations: FileTreeBatchOperation[] = [];
  for (const path of removed.sort((left, right) => depth(left) - depth(right))) {
    if (removedDirs.some(dir => dir !== path && path.startsWith(dir))) continue;
    operations.push({ type: "remove", path, recursive: true });
  }
  const added = next.filter(path => !known.has(path));
  for (const path of added.sort((left, right) => depth(left) - depth(right))) {
    operations.push({ type: "add", path });
  }
  return operations;
}

export function toGitStatusEntries(decorations: FileTreeDecorations | undefined): GitStatusEntry[] {
  if (!decorations) return [];
  const entries: GitStatusEntry[] = [];
  for (const [path, decoration] of Object.entries(decorations)) {
    const status = decoration.status ? PIERRE_STATUS[decoration.status] : null;
    if (status) entries.push({ path, status });
    if (decoration.containsChanges && isDirectoryPath(path)) {
      entries.push({ path: `${path}${UNLOADED_CHANGES_ENTRY}`, status: "modified" });
    }
  }
  return entries;
}

function formatCount(value: number): string {
  return value.toLocaleString("en-US");
}

/**
 * The custom decoration lane: a loading spinner wins, then `+N −M` counts
 * (with a conflict mark appended), then the conflict mark alone, then the
 * sensitive lock, then the symlink glyph. Colours are token references so the
 * shadow root inherits the active theme.
 */
export function buildRowDecoration(
  decoration: FileTreeDecoration | undefined,
  loading: boolean,
  reducedMotion = false
): FileTreeRowDecoration | null {
  if (loading) {
    return { icon: reducedMotion ? TREE_SPINNER_STATIC_ICON : TREE_SPINNER_ICON, title: "Loading" };
  }
  if (!decoration) return null;

  const parts: DecorationPart[] = [];
  const hasCounts = decoration.additions !== undefined || decoration.deletions !== undefined;
  if (hasCounts) {
    parts.push({
      text: `+${formatCount(decoration.additions ?? 0)}`,
      color: "var(--color-success)",
    });
    parts.push({ text: " " });
    parts.push({
      text: `−${formatCount(decoration.deletions ?? 0)}`,
      color: "var(--color-destructive)",
    });
  }
  if (decoration.status === "conflicted") {
    if (parts.length > 0) parts.push({ text: " " });
    parts.push({ text: "C", color: "var(--color-destructive)" });
  }
  if (parts.length > 0) {
    const title = decoration.status === "conflicted" ? "Conflicted" : undefined;
    return { text: parts.map(part => part.text).join(""), parts, title };
  }
  if (decoration.sensitive) return { icon: TREE_LOCK_ICON, title: "Sensitive file" };
  if (decoration.symlink) return { icon: TREE_SYMLINK_ICON, title: "Symlink" };
  return null;
}
