import type { ConnectionStatus } from "@agh/ui";

import type { HealthPayload } from "@/systems/status";

export type RuntimeConnectionTone = "success" | "danger";

export interface RuntimeConnectionIndicatorState {
  tone: RuntimeConnectionTone;
  pulse: boolean;
  label: string;
}

export function resolveRuntimeConnectionState(
  status: ConnectionStatus,
  healthStatus: HealthPayload["status"] | undefined
): RuntimeConnectionIndicatorState {
  if (status === "connected") {
    if (healthStatus === "degraded") {
      return { tone: "success", pulse: true, label: "Degraded" };
    }
    return { tone: "success", pulse: false, label: "Connected" };
  }
  if (status === "connecting") {
    return { tone: "success", pulse: true, label: "Connecting" };
  }
  if (status === "error") {
    return { tone: "danger", pulse: false, label: "Connection error" };
  }
  return { tone: "danger", pulse: false, label: "Disconnected" };
}
