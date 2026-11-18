import { keepPreviousData, useQuery } from "@tanstack/react-query";

import type { FileFindResponse, FilesRootRef } from "../adapters/files-api";
import { filesErrorCode } from "../adapters/files-api";
import { parseGoToFileQuery, type GoToFileQuery } from "../lib/go-to-file-query";
import { fileFindOptions } from "../lib/query-options";

export type FilesFindState =
  | { kind: "recent" }
  | { kind: "loading" }
  | { kind: "unavailable" }
  | { kind: "error"; message: string }
  | { kind: "results"; response: FileFindResponse; refreshing: boolean };

export interface UseFilesFindResult {
  query: GoToFileQuery;
  state: FilesFindState;
}

/**
 * Go to file over the daemon's name index. Every query is its own cached request, so only the
 * latest query's answer renders (US-013.EC-4); while it loads, the previous results stay
 * visible (EC-5). An empty name means the recent list (AC-7).
 */
export function useFilesFind(root: FilesRootRef, input: string): UseFilesFindResult {
  const query = parseGoToFileQuery(input);
  const enabled = query.name !== "";
  const result = useQuery({
    ...fileFindOptions(root, query.name),
    enabled,
    placeholderData: keepPreviousData,
  });
  let state: FilesFindState;
  if (!enabled) state = { kind: "recent" };
  else if (result.data) {
    state = { kind: "results", response: result.data, refreshing: result.isPlaceholderData };
  } else if (result.isError) {
    state =
      filesErrorCode(result.error) === "root_unavailable"
        ? { kind: "unavailable" }
        : { kind: "error", message: result.error.message };
  } else state = { kind: "loading" };
  return { query, state };
}
