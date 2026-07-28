import { useQuery } from "@tanstack/react-query";

import { directoryBrowseOptions } from "../lib/query-options";
import type { DirectoryBrowseQuery } from "../types";

export function useDirectoryBrowser(query: DirectoryBrowseQuery, enabled = true) {
  return useQuery(directoryBrowseOptions(query, enabled));
}
