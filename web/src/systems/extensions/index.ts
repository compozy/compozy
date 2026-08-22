export type {
  ExtensionEnablement,
  ExtensionEntry,
  ExtensionInstallRequest,
  ExtensionInstallPreview,
  ExtensionInstanceScope,
  ExtensionKitInventory,
  ExtensionKitItem,
  ExtensionLogEntry,
  ExtensionLogsSnapshot,
  ExtensionProvenance,
  ExtensionUpdateRequest,
  InstalledExtensionView,
} from "./types";
export {
  useExtensionDetail,
  useExtensionInstanceScope,
  useExtensionInventory,
  useExtensionKitInventory,
  useExtensionProvenance,
} from "./hooks/use-extensions";
export {
  useRemoveExtension,
  useToggleExtension,
  useUpdateExtension,
} from "./hooks/use-extension-actions";
export type {
  RemoveExtensionVariables,
  ToggleExtensionVariables,
  UpdateExtensionVariables,
} from "./hooks/use-extension-actions";
export { useExtensionDetailState } from "./hooks/use-extension-detail-state";
export type { ExtensionNetworkConfirm } from "./hooks/use-extension-detail-state";
export {
  EXTENSION_LOG_EVENT_NAME,
  EXTENSION_LOG_RESET_EVENT_NAME,
  useExtensionLogs,
} from "./hooks/use-extension-logs";
export type {
  ExtensionLogEventSource,
  ExtensionLogsModel,
  ExtensionLogStreamStatus,
} from "./hooks/use-extension-logs";
export {
  ExtensionNetworkConfirmDialog,
  ExtensionProvenanceDialog,
  RemoveExtensionDialog,
  VerifiedMark,
} from "./components/extension-dialogs";
export { ExtensionKitInventoryPanel } from "./components/extension-kit-inventory-panel";
export { ExtensionDeclaredProfiles } from "./components/extension-declared-profiles";
export { ExtensionLogPanel } from "./components/extension-log-panel";
export { ExtensionSkippedComponents } from "./components/extension-skipped-components";
export type { ExtensionSkippedComponentsProps } from "./components/extension-skipped-components";
export { selectExtensionSkippedDiagnostics } from "./lib/extension-skipped-diagnostics";
export type { ExtensionInventoryDiagnostic } from "./lib/extension-skipped-diagnostics";
export {
  EXTENSION_AGENT_PLUGIN_FORMAT,
  EXTENSION_AGENT_PLUGIN_LABEL,
  EXTENSION_CHECKSUM_VERIFIED_LABEL,
  EXTENSION_DEV_LABEL,
  EXTENSION_DIGEST_MATCHED_LABEL,
  EXTENSION_OVERRIDES_PUBLISHED_LABEL,
  ExtensionFormatBadge,
  ExtensionTrustBadges,
} from "./components/extension-trust-badges";
export {
  EXTENSION_NETWORK_CONFIRMATION_CODE,
  extensionNetworkConfirmation,
} from "./lib/extension-network-confirmation";
export {
  appendExtensionLogEntries,
  buildExtensionLogsStreamUrl,
  extensionLogCursor,
} from "./lib/extension-log-stream";
export { extensionSourceKindLabel } from "./lib/extension-source-kind";
export { extensionTrustFacts } from "./lib/extension-trust-facts";
export type { ExtensionTrustFacts, ExtensionTrustSource } from "./lib/extension-trust-facts";
export {
  EXTENSION_GLOBAL_WORKSPACE_KEY,
  extensionKeys,
  extensionProfileKey,
  extensionWorkspaceKey,
} from "./lib/query-keys";
export { extensionsListOptions } from "./lib/query-options";
export { previewExtensionInstall } from "./adapters/extensions-api";
