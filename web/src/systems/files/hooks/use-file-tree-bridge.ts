import { useQueries, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import {
  filesErrorCode,
  type FileEntry,
  type FileRootPayload,
  type FilesRootRef,
} from "../adapters/files-api";
import {
  ancestorListings,
  entriesDecorations,
  listingPath,
  treePath,
} from "../lib/file-tree-paths";
import { requestFreshListing, requestFreshListings } from "../lib/fresh-listings";
import { pruneCollapsedListings } from "../lib/listing-cache";
import { filesKeys } from "../lib/query-keys";
import { fileEntriesOptions } from "../lib/query-options";

export type FileTreeRootState =
  | { status: "loading" }
  | { status: "unavailable" }
  | { status: "error"; message: string; retry: () => void }
  | { status: "ready"; root: FileRootPayload; statusUnavailable: boolean };

export interface FileTreeBridge {
  root: FileTreeRootState;
  paths: string[];
  loadingPaths: string[];
  decorations: ReturnType<typeof entriesDecorations>;
  /** Directories (listing paths) capped at the entry limit. */
  truncated: string[];
  /** Expanded directories that listed no entries. */
  empty: string[];
  /** Expanded directories whose listing failed, with nothing cached to show (listing paths). */
  failed: FileTreeFolderFailure[];
  /** The tree asked for a directory's children (`dir/`). */
  expand: (directory: string) => void;
  /** The tree's expanded directories (`dir/`) changed, collapses included. */
  expandedChanged: (directories: readonly string[]) => void;
  refresh: () => void;
}

export interface FileTreeFolderFailure {
  path: string;
  message: string;
  retry: () => void;
}

function addListings(current: readonly string[], next: readonly string[]): readonly string[] {
  const known = new Set(current);
  const missing = next.filter(path => !known.has(path));
  return missing.length === 0 ? current : [...current, ...missing];
}

/** Listing paths shown now: the root, the open file's ancestors, and every open requested folder. */
function shownListings(
  requested: readonly string[],
  open: ReadonlySet<string>,
  reveal: readonly string[]
): string[] {
  const shown = new Set(["", ...reveal]);
  for (const path of requested) {
    // A folder is shown only while it and every ancestor are expanded.
    if (path === "" || ancestorListings(`${path}/x`).every(dir => open.has(dir))) {
      shown.add(path);
    }
  }
  return [...shown];
}

/**
 * The lazy bridge between the daemon's one-level listings and `FileTree`'s flat path set:
 * each requested directory is one cached query, so re-expanding a fresh directory never
 * refetches (US-001.AC-3) and a reveal loads exactly the ancestors it needs (spec data flow 4).
 *
 * Only shown folders (open, with open ancestors) keep an active query. A collapsed folder's
 * listing stays in the cache without an observer: its rows stay filterable, Refresh does not
 * refetch it (it is marked stale and refetches when shown again), and the cache can evict it
 * (perf audit P-5).
 */
export function useFileTreeBridge(
  root: FilesRootRef,
  options: { showIgnored: boolean; revealPath?: string | null }
): FileTreeBridge {
  const { showIgnored, revealPath } = options;
  const queryClient = useQueryClient();
  const [requested, setRequested] = useState<readonly string[]>([""]);
  const [open, setOpen] = useState<ReadonlySet<string>>(() => new Set());
  // The open file's ancestors load with the tree, so a link click reveals it (US-005.AC-1).
  const shown = shownListings(requested, open, revealPath ? ancestorListings(revealPath) : []);
  const results = useQueries({
    queries: shown.map(path => fileEntriesOptions(root, path, showIgnored)),
  });

  const entries: FileEntry[] = [];
  const loadingPaths: string[] = [];
  const truncated: string[] = [];
  const empty: string[] = [];
  const failed: FileTreeFolderFailure[] = [];
  let statusUnavailable = false;
  shown.forEach((path, index) => {
    const result = results[index];
    if (!result) return;
    // A retry of a failed folder spins too; with `retry: false` it stays `error` until it answers.
    if ((result.isPending || (result.isError && result.isFetching)) && path !== "")
      loadingPaths.push(`${path}/`);
    if (result.isError && !result.data && path !== "" && !result.isFetching) {
      const queryKey = fileEntriesOptions(root, path, showIgnored).queryKey;
      failed.push({
        path,
        message: result.error.message,
        retry: () => {
          requestFreshListing(queryKey);
          void result.refetch();
        },
      });
    }
    if (!result.data) return;
    entries.push(...result.data.entries);
    if (result.data.truncated) truncated.push(path);
    if (result.data.entries.length === 0 && path !== "") empty.push(path);
    statusUnavailable ||= result.data.status_unavailable === true;
  });
  // Collapsed folders keep their cached rows (filter, re-expansion) without an observer.
  const shownSet = new Set(shown);
  for (const path of requested) {
    if (shownSet.has(path)) continue;
    const cached = queryClient.getQueryData(fileEntriesOptions(root, path, showIgnored).queryKey);
    if (cached) entries.push(...cached.entries);
  }

  const rootResult = results[0];
  let rootState: FileTreeRootState;
  if (!rootResult || rootResult.isPending) {
    rootState = { status: "loading" };
  } else if (rootResult.isError) {
    rootState =
      filesErrorCode(rootResult.error) === "root_unavailable"
        ? { status: "unavailable" }
        : {
            status: "error",
            message: rootResult.error.message,
            retry: () => void rootResult.refetch(),
          };
  } else {
    rootState = { status: "ready", root: rootResult.data.root, statusUnavailable };
  }

  return {
    root: rootState,
    paths: entries.map(treePath),
    loadingPaths,
    decorations: entriesDecorations(entries),
    truncated,
    empty,
    failed,
    expand: directory => {
      const path = listingPath(directory);
      // Re-expanding a folder whose listing failed asks again; `retry: false` never would.
      const failure = failed.find(item => item.path === path);
      if (failure) failure.retry();
      else setRequested(current => addListings(current, [path]));
    },
    expandedChanged: directories => {
      const next = new Set(directories.map(listingPath));
      setOpen(current => (sameSet(current, next) ? current : next));
      // Keep the collapsed-folder cache bounded; forgotten folders list again when reopened.
      const dropped = new Set(pruneCollapsedListings(queryClient, root));
      if (dropped.size > 0) setRequested(current => current.filter(path => !dropped.has(path)));
    },
    // Only shown folders refetch, each bypassing the daemon's status snapshot; collapsed ones
    // are just marked stale (`invalidateQueries` refetches active queries only).
    refresh: () => {
      requestFreshListings(queryClient, root);
      void queryClient.invalidateQueries({ queryKey: filesKeys.entriesRoot(root) });
    },
  };
}

function sameSet(left: ReadonlySet<string>, right: ReadonlySet<string>): boolean {
  if (left.size !== right.size) return false;
  for (const value of left) if (!right.has(value)) return false;
  return true;
}
