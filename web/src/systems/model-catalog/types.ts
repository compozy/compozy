import type { OperationRequestBody, OperationResponse } from "@/lib/api-contract";

export type ProviderModelsListResponse = OperationResponse<"listProviderModelsByProvider", 200>;
export type ProviderModelPayload = ProviderModelsListResponse["models"][number];
export type ProviderModelSource = ProviderModelPayload["sources"][number];

/**
 * Aggregate catalog contract (`GET /api/model-catalog/models`). The daemon
 * returns every provider's rows in one response with the same per-row shape as
 * the by-provider endpoint, so the selector loads the whole cross-provider set
 * with a single request instead of fanning out per provider.
 */
export type AllModelsListResponse = OperationResponse<"listProviderModels", 200>;
export type AllModelsRefreshRequest = OperationRequestBody<"refreshProviderModels">;
export type AllModelsRefreshResponse = OperationResponse<"refreshProviderModels", 200>;

export type ProviderModelStatusResponse = OperationResponse<
  "getProviderModelStatusByProvider",
  200
>;
export type ProviderModelSourceStatus = ProviderModelStatusResponse["sources"][number];

export type ProviderModelsRefreshRequest = OperationRequestBody<"refreshProviderModelsByProvider">;
export type ProviderModelsRefreshResponse = OperationResponse<
  "refreshProviderModelsByProvider",
  200
>;

export const MODEL_AVAILABILITY_STATES = [
  "available_live",
  "available_stale",
  "unavailable_live",
  "unavailable_stale",
  "unknown",
] as const;

export type ModelAvailabilityState = (typeof MODEL_AVAILABILITY_STATES)[number];

export function isKnownAvailabilityState(value: string): value is ModelAvailabilityState {
  return (MODEL_AVAILABILITY_STATES as readonly string[]).includes(value);
}

export interface ProviderModelsQuery {
  providerId: string;
  sourceId?: string;
  includeStale?: boolean;
  /** Catalog view; the daemon defaults to `curated` when omitted. */
  view?: "curated" | "all";
}

export interface ProviderModelsRefreshInput {
  providerId: string;
  sourceId?: string;
  force?: boolean;
  requestId?: string;
}

/** Aggregate list query across every provider (`GET /api/model-catalog/models`). */
export interface AllModelsQuery {
  /** Optional single-provider narrowing; omitted returns every provider's rows. */
  providerId?: string;
  sourceId?: string;
  includeStale?: boolean;
  /** Refresh sources before listing (daemon-side); the selector leaves this off. */
  refresh?: boolean;
  /** Catalog view; the daemon defaults to `curated` when omitted. */
  view?: "curated" | "all";
}

/** Aggregate refresh across every provider (`POST /api/model-catalog/models/refresh`). */
export interface AllModelsRefreshInput {
  sourceId?: string;
  force?: boolean;
  requestId?: string;
}
