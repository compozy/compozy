// Types
export type {
  DaemonStatusPayload,
  DoctorPayload,
  HealthPayload,
  RuntimeIdentityPayload,
  StatusPayload,
} from "./types";

export type { DaemonReadiness, DaemonRecovery } from "./lib/daemon-readiness";

// Adapters
export { fetchRuntimeIdentity, fetchStatus } from "./adapters/daemon-api";

// Query infrastructure
export { daemonReadiness, daemonRecovery } from "./lib/daemon-readiness";
export { statusKeys } from "./lib/query-keys";
export { runtimeIdentityOptions, statusOptions } from "./lib/query-options";

// Hooks
export {
  useDaemonConnectionStatus,
  type ConnectionStatus,
} from "./hooks/use-daemon-connection-status";
export { deriveDaemonConnectionStatus, useDaemonHealth } from "./hooks/use-daemon-health";
export { useDaemonStatus } from "./hooks/use-daemon-status";
export { useStatus } from "./hooks/use-status";
export { useRuntimeIdentity } from "./hooks/use-runtime-identity";
export { useDaemonRecovery } from "./hooks/use-daemon-recovery";
export { useRuntimeReconnectRefresh } from "./hooks/use-runtime-reconnect-refresh";

// Components
export {
  DaemonRecoveryNotice,
  DaemonRecoveryNoticeHost,
  type DaemonRecoveryNoticeProps,
} from "./components/daemon-recovery-notice";
