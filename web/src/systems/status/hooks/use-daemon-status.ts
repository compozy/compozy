import { useQuery } from "@tanstack/react-query";

import type { StatusPayload } from "../types";
import { statusOptions } from "../lib/query-options";

function selectDaemon(status: StatusPayload) {
  return status.daemon;
}

export function useDaemonStatus() {
  return useQuery({
    ...statusOptions(),
    select: selectDaemon,
  });
}
