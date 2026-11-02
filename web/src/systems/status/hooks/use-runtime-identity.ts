import { useQuery } from "@tanstack/react-query";

import { runtimeIdentityOptions } from "../lib/query-options";

export function useRuntimeIdentity(options: { enabled?: boolean } = {}) {
  return useQuery(runtimeIdentityOptions(options.enabled ?? true));
}
