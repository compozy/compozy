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
  /** The tree asked for a directory's children (`dir/`). */
  expand: (directory: string) => void;
  refresh: () => void;
}

function addListings(current: readonly string[], next: readonly string[]): readonly string[] {
  const known = new Set(current);
  const missing = next.filter(path => !known.has(path));
  return missing.length === 0 ? current : [...current, ...missing];
}

/**
 * The lazy bridge between the daemon's one-level listings and `FileTree`'s flat path set:
 * each requested directory is one cached query, so re-expanding a fresh directory never
 * refetches (US-001.AC-3) and a reveal loads exactly the ancestors it needs (spec data flow 4).
 */
export function useFileTreeBridge(
  root: FilesRootRef,
  options: { showIgnored: boolean; revealPath?: string | null }
): FileTreeBridge {
  const { showIgnored, revealPath } = options;
  const queryClient = useQueryClient();
  const [expanded, setRequested] = useState<readonly string[]>([""]);
  // The open file's ancestors load with the tree, so a link click reveals it (US-005.AC-1).
  const requested = revealPath ? addListings(expanded, ancestorListings(revealPath)) : expanded;
  const results = useQueries({
    queries: requested.map(path => fileEntriesOptions(root, path, showIgnored)),
  });

  const entries: FileEntry[] = [];
  const loadingPaths: string[] = [];
  const truncated: string[] = [];
  const empty: string[] = [];
  let statusUnavailable = false;
  requested.forEach((path, index) => {
    const result = results[index];
    if (!result) return;
    if (result.isPending && path !== "") loadingPaths.push(`${path}/`);
    if (!result.data) return;
    entries.push(...result.data.entries);
    if (result.data.truncated) truncated.push(path);
    if (result.data.entries.length === 0 && path !== "") empty.push(path);
    statusUnavailable ||= result.data.status_unavailable === true;
  });

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
    expand: directory => setRequested(current => addListings(current, [listingPath(directory)])),
    refresh: () => void queryClient.invalidateQueries({ queryKey: filesKeys.entriesRoot(root) }),
  };
}
