// Types
export type {
  GatewayAddressStatus,
  GatewayAuditFinding,
  GatewayAuditReport,
  GatewayDesiredState,
  GatewayDevice,
  GatewayDeviceHighlights,
  GatewayIngressBinding,
  GatewayIngressReachability,
  GatewayPairingArtifact,
  GatewayProviderStatus,
  GatewayReachability,
  GatewayRedeemResult,
  GatewayRefusal,
  GatewayRevokeResult,
  GatewayStatus,
  GatewaySurface,
  GatewaySurfaceRequest,
  GatewaySurfaceStatus,
  GatewayTier,
  GatewayTierStatus,
} from "./types";

// Hooks
export { useGatewayPage, type GatewayPageViewModel } from "./hooks/use-gateway-page";
export { useGatewayAudit, type GatewayAuditViewModel } from "./hooks/use-gateway-audit";
export {
  useGatewaySettingsPage,
  type GatewayPairingViewModel,
  type GatewaySettingsViewModel,
} from "./hooks/use-gateway-settings-page";
export {
  useGatewayAccessState,
  useGatewayAccessTier,
  useRedeemGatewayPairing,
  type RedeemPairingInput,
} from "./hooks/use-gateway-access";
export {
  useDisableGatewayProvider,
  useEnableGatewayProvider,
  useMintGatewayPairing,
  useRenameGatewayDevice,
  useRevokeGatewayDevice,
  useSetGatewaySurface,
  type EnableGatewayProviderInput,
} from "./hooks/use-gateway-mutations";
export { useGatewayProviders, type GatewayProvidersViewModel } from "./hooks/use-gateway-providers";
export {
  activationForTier,
  buildGatewayProviderModel,
  connectivityProviderExtensions,
  providerGeneration,
  CONNECTIVITY_PROVIDER_CAPABILITY,
  type GatewayProviderBlocker,
  type GatewayProviderCandidate,
  type GatewayProviderModel,
} from "./lib/gateway-provider-model";

// Components
export { GatewayAccessBoundary } from "./components/gateway-access-boundary";
export { GatewayAccessEnded } from "./components/gateway-access-ended";
export { GatewayAddressesList } from "./components/gateway-addresses-list";
export { GatewayAuditPanel } from "./components/gateway-audit-panel";
export { GatewayDeviceList } from "./components/gateway-device-list";
export { GatewayDeviceRow } from "./components/gateway-device-row";
export { GatewayExposureSection } from "./components/gateway-exposure-section";
export { GatewayIngressStatus } from "./components/gateway-ingress-status";
export { GatewayPairingDialog } from "./components/gateway-pairing-dialog";
export { GatewayPairingGate } from "./components/gateway-pairing-gate";
export { GatewayProviderHealthChip } from "./components/gateway-provider-health-chip";
export { GatewayProviderRow } from "./components/gateway-provider-row";
export { GatewayProviderSection } from "./components/gateway-provider-section";
export { GatewayPublicConsentDialog } from "./components/gateway-public-consent-dialog";
export { GatewayStatusChip } from "./components/gateway-status-chip";

// Utilities
export {
  buildGatewayExposureModel,
  isLocalOnly,
  liveAddresses,
  surfaceReachability,
  tierReachability,
  type GatewayExposureModel,
  type GatewayExposureRow,
} from "./lib/gateway-exposure-model";
export {
  auditSeverityCopy,
  deviceActorLabel,
  deviceOriginLabel,
  ingressReachabilityCopy,
  DELIVERY_DURABILITY_NOTE,
  EXPOSURE_ROW_COPY,
  PUBLIC_OPERATOR_CONSENT_DISCLOSURE,
  REACHABILITY_COPY,
  SURFACE_LABEL,
  TIER_LABEL,
  type GatewaySignalTone,
} from "./lib/gateway-copy";
export {
  buildPairingLink,
  clearPairingFragment,
  readPairingFragment,
} from "./lib/pairing-fragment";

// Query keys & options
export { gatewayKeys } from "./lib/query-keys";
export {
  gatewayAuditOptions,
  gatewayDevicesOptions,
  gatewayStatusOptions,
} from "./lib/query-options";

// Stores
export {
  gatewayAccessStore,
  startGatewayAccessObserver,
  type GatewayAccessState,
} from "./stores/gateway-access-store";

// API
export { gatewayApi } from "./adapters/gateway-api";
export { GatewayApiError } from "./adapters/gateway-api-error";
