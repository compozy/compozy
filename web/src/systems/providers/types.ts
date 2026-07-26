import type { OperationResponse } from "@/lib/api-contract";

export type ProviderListResponse = OperationResponse<"listProviders", 200>;
export type ProviderSummary = ProviderListResponse["providers"][number];
export type ProviderAuthStatus = ProviderSummary["auth_status"];
export type ProviderAuthProbeResponse = OperationResponse<"probeProviderAuth", 200>;
export type ProviderAuthProbeResult = NonNullable<ProviderAuthProbeResponse["probe"]>;
