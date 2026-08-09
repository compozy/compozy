import type { OperationRequestBody, OperationResponse } from "@/lib/api-contract";

export type GatewayStatus = OperationResponse<"getGatewayStatus", 200>;
export type GatewayAuditReport = OperationResponse<"getGatewayAudit", 200>;

export type GatewayTierStatus = GatewayStatus["tiers"][number];
export type GatewaySurfaceStatus = GatewayStatus["surfaces"][number];
export type GatewayProviderStatus = GatewayStatus["providers"][number];
export type GatewayAddressStatus = GatewayStatus["addresses"][number];
export type GatewayDevice = GatewayStatus["devices"][number];
export type GatewayIngressBinding = GatewayStatus["bindings"][number];
export type GatewayRefusal = NonNullable<GatewayStatus["refusal"]>;

export type GatewayAuditFinding = GatewayAuditReport["findings"][number];
export type GatewayDeviceHighlights = GatewayAuditReport["device_highlights"];

export type GatewayPairingArtifact = OperationResponse<"mintGatewayPairing", 201>;
export type GatewayRedeemRequest = OperationRequestBody<"redeemGatewayPairing">;
export type GatewayRedeemResult = OperationResponse<"redeemGatewayPairing", 200>;
export type GatewayRevokeResult = OperationResponse<"revokeGatewayDevice", 200>;
export type GatewaySurfaceRequest = OperationRequestBody<"setGatewaySurface">;
export type GatewayProviderEnableRequest = OperationRequestBody<"enableGatewayProvider">;

/** The two tiers the daemon manages independently. */
export type GatewayTier = "private" | "public";

/** The two route inventories whose reachability is managed as a unit. */
export type GatewaySurface = "operator_ui" | "webhook_ingress";

/** Operator intent, as opposed to what the runtime has actually reached. */
export type GatewayDesiredState = "enabled" | "disabled";

/**
 * What the operator is told about a tier. Derived from verified runtime state,
 * never from intent: see `lib/gateway-exposure-model.ts`.
 */
export type GatewayReachability = "off" | "pending" | "degraded" | "reachable" | "refused";

/** Honest public projection for one bindable ingress subject. */
export type GatewayIngressReachability =
  | "off"
  | "unconfirmed"
  | "live"
  | "broken"
  | "reconfirmation_required";
