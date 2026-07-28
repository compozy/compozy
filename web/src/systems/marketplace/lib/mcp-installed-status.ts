import {
  authorizeLabel,
  composeMCPRowStatus,
  type MCPStatusCell,
  type SettingsMCPServerEntry,
} from "@/systems/settings";

export interface MarketplaceMCPInstalledStatus {
  authorize: boolean;
  label: string;
  tone: MCPStatusCell["tone"];
}

export function marketplaceMCPInstalledStatus(
  server: SettingsMCPServerEntry
): MarketplaceMCPInstalledStatus {
  if (authorizeLabel(server)) {
    return { authorize: true, label: "authorize", tone: "warning" };
  }

  const runtime = composeMCPRowStatus(server).runtime;
  return {
    authorize: false,
    label: runtime.label === "Ready" ? "running" : runtime.label.toLowerCase(),
    tone: runtime.tone,
  };
}
