import { queryOptions } from "@tanstack/react-query";

import { gatewayApi } from "../adapters/gateway-api";
import { gatewayKeys } from "./query-keys";

const GATEWAY_STALE_TIME = 10_000;
const GATEWAY_REFETCH_INTERVAL = 30_000;

/**
 * Posture is polled because reconciliation is asynchronous: a tier can move
 * from `establishing` to `up` (or to `degraded`) without any operator action,
 * and the surface must not keep showing a state the runtime has left.
 */
export function gatewayStatusOptions() {
  return queryOptions({
    queryKey: gatewayKeys.status(),
    queryFn: ({ signal }) => gatewayApi.status(signal),
    staleTime: GATEWAY_STALE_TIME,
    refetchInterval: GATEWAY_REFETCH_INTERVAL,
  });
}

export function gatewayDevicesOptions() {
  return queryOptions({
    queryKey: gatewayKeys.devices(),
    queryFn: ({ signal }) => gatewayApi.listDevices(signal),
    staleTime: GATEWAY_STALE_TIME,
    refetchInterval: GATEWAY_REFETCH_INTERVAL,
  });
}

/**
 * The audit is an explicit operator action, not ambient state: it is fetched
 * only when asked for and never polled, so "last run" stays meaningful.
 */
export function gatewayAuditOptions() {
  return queryOptions({
    queryKey: gatewayKeys.audit(),
    queryFn: ({ signal }) => gatewayApi.audit(signal),
    enabled: false,
    staleTime: Number.POSITIVE_INFINITY,
    refetchOnWindowFocus: false,
  });
}
