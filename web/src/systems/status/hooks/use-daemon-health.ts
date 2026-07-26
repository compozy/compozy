import { useQuery } from "@tanstack/react-query";
import type { ConnectionStatus } from "@agh/ui";

import { statusOptions } from "../lib/query-options";
import type { HealthPayload, StatusPayload } from "../types";

interface DaemonHealthResult {
  health: HealthPayload | undefined;
  connectionStatus: ConnectionStatus;
  isInitialLoading: boolean;
}

interface DaemonConnectionQueryState {
  data?: unknown;
  isError: boolean;
  isFetching: boolean;
  isPending: boolean;
  isSuccess: boolean;
}

function selectHealth(status: StatusPayload) {
  return status.health;
}

export function deriveDaemonConnectionStatus(query: DaemonConnectionQueryState): ConnectionStatus {
  if (query.isPending || (query.isFetching && query.data === undefined)) {
    return "connecting";
  }
  if (query.isSuccess) {
    return "connected";
  }
  if (query.isError) {
    return "error";
  }
  if (query.isFetching) {
    return "connecting";
  }
  return "disconnected";
}

export function useDaemonHealth(): DaemonHealthResult {
  const query = useQuery({
    ...statusOptions(),
    select: selectHealth,
  });

  return {
    health: query.data,
    connectionStatus: deriveDaemonConnectionStatus(query),
    isInitialLoading: query.isLoading,
  };
}
