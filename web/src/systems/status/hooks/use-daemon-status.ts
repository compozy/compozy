import { useQuery } from "@tanstack/react-query";

import type { StatusPayload } from "../types";
import { statusOptions } from "../lib/query-options";

function selectDaemon(status: StatusPayload) {
  return status.daemon;
}

export function useDaemonStatus(options: { enabled?: boolean } = {}) {
  return useQuery({
    ...statusOptions(options.enabled ?? true),
    select: selectDaemon,
  });
}
