import { useQuery } from "@tanstack/react-query";

import { statusOptions } from "../lib/query-options";

export function useStatus(options: { enabled?: boolean } = {}) {
  return useQuery(statusOptions(options.enabled ?? true));
}
