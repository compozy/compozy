import { useQuery } from "@tanstack/react-query";

import { statusOptions } from "../lib/query-options";

export function useStatus() {
  return useQuery(statusOptions());
}
