import { useQuery } from "@tanstack/react-query";

import { gatewayAuditOptions } from "../lib/query-options";
import type { GatewayAuditReport } from "../types";

export interface GatewayAuditViewModel {
  hasRun: boolean;
  isRunning: boolean;
  error: Error | null;
  report: GatewayAuditReport | null;
  run: () => void;
}

/**
 * The audit is an operator action with a result, not ambient state: it is
 * fetched only when asked for, and re-running is an explicit refetch so
 * "cleared after remediation" is something the person caused and can see.
 */
export function useGatewayAudit(): GatewayAuditViewModel {
  const query = useQuery(gatewayAuditOptions());

  return {
    hasRun: query.isFetchedAfterMount && !query.isFetching && query.data !== undefined,
    isRunning: query.isFetching,
    error: query.error,
    report: query.data ?? null,
    run: () => void query.refetch(),
  };
}
