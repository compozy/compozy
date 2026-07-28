import type { OperationQuery, OperationRequestBody, OperationResponse } from "@/lib/api-contract";

export type BridgeListFilter = OperationQuery<"listBridges">;
export type BridgeCatalogFilter = Omit<BridgeListFilter, "cursor">;
export type BridgesListResponse = OperationResponse<"listBridges", 200>;
export type BridgeSummary = BridgesListResponse["bridges"][number];
export type BridgeHealthMap = NonNullable<BridgesListResponse["bridge_health"]>;
export type BridgeDetailResponse = OperationResponse<"getBridge", 200>;
export type BridgeHealth = BridgeDetailResponse["health"];
export type BridgeRoute = OperationResponse<"listBridgeRoutes", 200>["routes"][number];
export type BridgeTargetsQuery = OperationQuery<"listBridgeTargets">;
export type BridgeTargetsResponse = OperationResponse<"listBridgeTargets", 200>;
export type BridgeTarget = BridgeTargetsResponse["targets"][number];
export type BridgeResolveTargetRequest = OperationRequestBody<"resolveBridgeTarget">;
export type BridgeResolveTargetResponse = OperationResponse<"resolveBridgeTarget", 200 | 404 | 422>;
export type BridgeProvider = OperationResponse<"listBridgeProviders", 200>["providers"][number];
export type CreateBridgeRequest = OperationRequestBody<"createBridge">;
export type CreateBridgeResponse = OperationResponse<"createBridge", 201>;
export type UpdateBridgeRequest = OperationRequestBody<"updateBridge">;
export type UpdateBridgeResponse = OperationResponse<"updateBridge", 200>;
export type BridgeSecretBindingsResponse = OperationResponse<"listBridgeSecretBindings", 200>;
export type BridgeSecretBinding = BridgeSecretBindingsResponse["bindings"][number];
export type PutBridgeSecretBindingRequest = OperationRequestBody<"putBridgeSecretBinding">;
export type PutBridgeSecretBindingResponse = OperationResponse<"putBridgeSecretBinding", 200>;
export type TestBridgeDeliveryRequest = OperationRequestBody<"testBridgeDelivery">;
export type TestBridgeDeliveryResponse = OperationResponse<"testBridgeDelivery", 200>;
export type SlackBridgeManifestResponse = OperationResponse<"getSlackBridgeManifest", 200>;
export type SlackBridgeManifest = SlackBridgeManifestResponse["manifest"];
export type BridgeVerifyResponse = OperationResponse<"verifyBridge", 200>;
export type BridgeVerifyCheck = BridgeVerifyResponse["checks"][number];
export type BridgeCheckStatus = BridgeVerifyCheck["status"];
export type SendBridgeTestRequest = OperationRequestBody<"sendBridgeTest">;
export type SendBridgeTestResponse = OperationResponse<"sendBridgeTest", 200>;
export type BridgeWebhookRegistrationResponse = OperationResponse<"registerBridgeWebhook", 200>;
export type EnableBridgeResponse = OperationResponse<"enableBridge", 200>;
export type DisableBridgeResponse = OperationResponse<"disableBridge", 200>;
export type RestartBridgeResponse = OperationResponse<"restartBridge", 200>;

export type BridgeScope = BridgeSummary["scope"];
export type BridgeScopeFilter = "all" | BridgeScope;
export type BridgeStatus = BridgeSummary["status"];
export type BridgeRoutingPolicy = BridgeSummary["routing_policy"];
export type BridgeDeliveryMode = NonNullable<TestBridgeDeliveryRequest["target"]["mode"]>;
export type BridgeDmPolicy = NonNullable<CreateBridgeRequest["dm_policy"]>;
export type BridgeProviderConfig = NonNullable<CreateBridgeRequest["provider_config"]>;
export type BridgeProviderSecretSlot = NonNullable<BridgeProvider["secret_slots"]>[number];
export type BridgeProviderConfigSchemaHint = NonNullable<BridgeProvider["config_schema"]>;

type GeneratedBridgeDeliveryDefaults = NonNullable<CreateBridgeRequest["delivery_defaults"]>;

export type BridgeProgressConfig = NonNullable<GeneratedBridgeDeliveryDefaults["progress"]>;
export type BridgeProgressMode = BridgeProgressConfig["tool_progress"];
export type BridgeProgressGrouping = BridgeProgressConfig["grouping"];
export type BridgeDeliveryTargetDefaults = Omit<
  TestBridgeDeliveryRequest["target"],
  "bridge_instance_id"
>;
export type BridgeDeliveryDefaults = GeneratedBridgeDeliveryDefaults;

export interface BridgeCreateDraft {
  deliveryDefaults: BridgeDeliveryDefaults;
  dmPolicy: BridgeDmPolicy | "";
  displayName: string;
  /** `CreateBridgeRequest.enabled` — whether the daemon starts it on acceptance. */
  enabled: boolean;
  /** `CreateBridgeRequest.notification_suppress`. */
  notificationSuppress: boolean;
  /** Write-only slot values, keyed by the provider-declared slot name. */
  secretSlotValues: Record<string, string>;
  providerConfigText: string;
  routingPolicy: BridgeRoutingPolicy;
  scope: BridgeScope;
  selectedProviderKey: string;
}

export interface BridgeTestDeliveryDraft {
  message: string;
  target: BridgeDeliveryTargetDefaults;
}

export interface BridgeUpdateDraft {
  deliveryDefaults: BridgeDeliveryDefaults;
  dmPolicy: BridgeDmPolicy | "";
  displayName: string;
  providerConfigText: string;
  routingPolicy: BridgeRoutingPolicy;
}

export interface BridgeHealthStreamSnapshot {
  bridge_health: BridgeHealthMap;
  generated_at: string;
}
