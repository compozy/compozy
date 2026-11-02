import type { ConnectionStatus } from "@compozy/ui";

import { useRuntimeIdentity } from "./use-runtime-identity";
import { deriveDaemonConnectionStatus } from "./use-daemon-health";

function useDaemonConnectionStatus(): ConnectionStatus {
  return deriveDaemonConnectionStatus(useRuntimeIdentity());
}

export { useDaemonConnectionStatus };
export type { ConnectionStatus };
