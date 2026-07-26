import { useQuery } from "@tanstack/react-query";
import type { ConnectionStatus } from "@agh/ui";

import { statusOptions } from "../lib/query-options";
import { deriveDaemonConnectionStatus } from "./use-daemon-health";

function useDaemonConnectionStatus(): ConnectionStatus {
  return deriveDaemonConnectionStatus(useQuery(statusOptions()));
}

export { useDaemonConnectionStatus };
export type { ConnectionStatus };
