import { parsePatchFiles, type FileDiffMetadata } from "@pierre/diffs";

import { contentVersion } from "./code-viewer-lines";

export type DiffViewerChangeType = "change" | "new" | "deleted" | "rename-pure" | "rename-changed";

export interface DiffViewerItem {
  /** Stable identity, normally the root-relative path. */
  id: string;
  /** Bump when the item's content changes (for example the changes revision). */
  version: string | number;
  path: string;
  /** Old path for renames. */
  prevPath?: string;
  changeType?: DiffViewerChangeType;
  /**
   * `false` never fetches a patch (binary, too large, sensitive, generated):
   * only the header renders, and the `fileHeader` slot explains why.
   */
  loadable?: boolean;
  /**
   * `false` hides the expand-context control for this item even when the viewer has
   * `loadContents` (for example contents over the expand cap); the `fileHeader` slot
   * explains why.
   */
  expandable?: boolean;
  collapsed?: boolean;
}

/** Where an item's patch stands; passed to the header slot. */
export type DiffViewerItemStatus = "pending" | "ready" | "unavailable" | "error";

export interface DiffViewerPatchEntry {
  fileDiff?: FileDiffMetadata;
  status: DiffViewerItemStatus;
  /** The item version the entry belongs to; a newer version reloads. */
  version: string;
}

export function versionKey(item: DiffViewerItem): string {
  return String(item.version);
}

/** Header-only metadata for an item whose patch is not loaded (or never will be). */
export function placeholderDiff(item: DiffViewerItem): FileDiffMetadata {
  return {
    name: item.path,
    ...(item.prevPath ? { prevName: item.prevPath } : {}),
    type: item.changeType ?? "change",
    hunks: [],
    splitLineCount: 0,
    unifiedLineCount: 0,
    isPartial: true,
    deletionLines: [],
    additionLines: [],
    deletionCharacterCount: 0,
    additionCharacterCount: 0,
    cacheKey: `${item.id}:${versionKey(item)}:placeholder`,
  };
}

/**
 * Parses one file's unified patch. The cache key carries the item version so
 * the worker pool never reuses a highlight across revisions.
 */
export function parseItemPatch(item: DiffViewerItem, patch: string): FileDiffMetadata | null {
  let files: FileDiffMetadata[];
  try {
    files = parsePatchFiles(patch).flatMap(parsed => parsed.files);
  } catch {
    return null;
  }
  const fileDiff = files.find(file => file.name === item.path) ?? files[0];
  if (!fileDiff) return null;
  fileDiff.cacheKey = `${item.id}:${versionKey(item)}`;
  return fileDiff;
}

/**
 * Pierre hydrates collapsed context only for partial `change` and `rename-changed` diffs, and
 * takes one loader for the whole CodeView. A non-expandable item is handed over as
 * `rename-pure`: the same hunks render, with no expand control. Callers cache the result,
 * since Pierre compares file objects by reference.
 */
export function withoutContextExpansion(fileDiff: FileDiffMetadata): FileDiffMetadata {
  if (fileDiff.type !== "change" && fileDiff.type !== "rename-changed") return fileDiff;
  // Its own cache key: Pierre reuses a rendered diff by key, expand controls included.
  return { ...fileDiff, type: "rename-pure", cacheKey: `${fileDiff.cacheKey ?? ""}:no-expand` };
}

/** CodeView wants a numeric version; derive one from everything that re-renders an item. */
export function codeViewVersion(
  item: DiffViewerItem,
  entry: DiffViewerPatchEntry | undefined,
  collapsed: boolean
): number {
  const expandable = item.expandable === false ? 0 : 1;
  return contentVersion(
    `${versionKey(item)}|${entry?.status ?? "pending"}|${collapsed ? 1 : 0}|${expandable}`
  );
}
