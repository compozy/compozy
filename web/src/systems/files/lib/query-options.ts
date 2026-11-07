import { desktopShellBridge } from "@/systems/os";
import { queryOptions } from "@tanstack/react-query";

import { listFileEntries, readFileContent, type FilesRootRef } from "../adapters/files-api";
import { filesKeys } from "./query-keys";

/** A fresh listing is reused on re-expand (US-001.AC-3); Refresh invalidates explicitly. */
const ENTRIES_STALE_MS = 30_000;

export function fileEntriesOptions(root: FilesRootRef, path: string, showIgnored: boolean) {
  return queryOptions({
    queryKey: filesKeys.entries(root, path, showIgnored),
    queryFn: ({ signal }) => listFileEntries(root, path, { showIgnored }, signal),
    staleTime: ENTRIES_STALE_MS,
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
    gcTime: reveal ? 0 : undefined,
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
