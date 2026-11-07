import { desktopShellBridge, type OpenTarget } from "@/systems/os";
import { queryOptions } from "@tanstack/react-query";

import {
  findFiles,
  listFileEntries,
  readFileContent,
  searchFileContents,
  type FileSearchParams,
  type FilesRootRef,
} from "../adapters/files-api";
import { consumeFreshListing } from "./fresh-listings";
import { withListingSlot } from "./listing-slots";
import { filesKeys } from "./query-keys";

/** A fresh listing is reused on re-expand (US-001.AC-3); Refresh invalidates explicitly. */
const ENTRIES_STALE_MS = 30_000;

/**
 * How long a listing or file nobody shows stays cached. Collapsed folders and closed files have
 * no observer, so a long session's cache is bounded by what was used in this window (P-5).
 */
export const FILES_CACHE_GC_MS = 5 * 60_000;

export function fileEntriesOptions(root: FilesRootRef, path: string, showIgnored: boolean) {
  return queryOptions({
    queryKey: filesKeys.entries(root, path, showIgnored),
    queryFn: ({ signal, queryKey }) =>
      withListingSlot(
        () =>
          listFileEntries(
            root,
            path,
            { showIgnored, fresh: consumeFreshListing(queryKey) },
            signal
          ),
        signal
      ),
    staleTime: ENTRIES_STALE_MS,
    gcTime: FILES_CACHE_GC_MS,
    retry: false,
  });
}

/**
 * One file's content. A revealed read is never kept once its view closes, so the sensitive
 * gate returns when the file is reopened (US-004.AC-4).
 */
export function fileContentOptions(root: FilesRootRef, path: string, reveal: boolean) {
  return queryOptions({
    queryKey: filesKeys.content(root, path, reveal),
    queryFn: ({ signal }) => readFileContent(root, path, { reveal }, signal),
    staleTime: reveal ? 0 : ENTRIES_STALE_MS,
    gcTime: reveal ? 0 : FILES_CACHE_GC_MS,
    retry: false,
  });
}

export function openTargetsOptions() {
  return queryOptions({
    queryKey: filesKeys.openTargets(),
    queryFn: async () => (await desktopShellBridge()?.files?.listOpenTargets?.()) ?? [],
    staleTime: 30_000,
  });
}

/**
 * The split button's primary target: the last-used one while it is still offered, else the
 * first editor, else the file manager (US-006.AC-1, AC-5).
 */
export function primaryOpenTarget(
  targets: readonly OpenTarget[],
  remembered: string
): OpenTarget | undefined {
  return (
    targets.find(target => target.id === remembered) ??
    targets.find(target => target.kind === "editor") ??
    targets.find(target => target.kind === "reveal")
  );
}

/** The explorer's index refreshes on the daemon every 15 s; a repeat query reuses its answer. */
const FIND_STALE_MS = 15_000;

/**
 * Go to file results for one query. Each query has its own key, so a slow answer to an older
 * query never replaces the newest one (US-013.EC-4); an abandoned request is aborted.
 */
export function fileFindOptions(root: FilesRootRef, q: string) {
  return queryOptions({
    queryKey: filesKeys.find(root, q),
    queryFn: ({ signal }) => findFiles(root, q, signal),
    staleTime: FIND_STALE_MS,
    retry: false,
  });
}

/** One content search; never stale-cached, since files change under an agent. */
export function fileSearchOptions(root: FilesRootRef, params: FileSearchParams) {
  return queryOptions({
    queryKey: filesKeys.search(root, params),
    queryFn: ({ signal }) => searchFileContents(root, params, signal),
    staleTime: 0,
    gcTime: 60_000,
    retry: false,
  });
}
