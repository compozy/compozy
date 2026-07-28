import type { SettingsSandboxEntry } from "@/systems/settings";

export const SANDBOX_BACKENDS = ["local", "daytona", "e2b"] as const;
export type SandboxBackend = (typeof SANDBOX_BACKENDS)[number];

export const SANDBOX_PERSISTENCE_OPTIONS = ["transient", "reuse", "archive"] as const;
export type SandboxPersistence = (typeof SANDBOX_PERSISTENCE_OPTIONS)[number];

export function sandboxBackendLabel(backend: string): string {
  if (backend === "local") return "host process · no sandbox";
  if (backend === "daytona") return "cloud workspace · Daytona";
  return `custom backend · ${backend}`;
}

export function sandboxBackendTone(backend: string): "success" | "info" | "neutral" {
  if (backend === "local") return "success";
  if (backend === "daytona") return "info";
  return "neutral";
}

export function sandboxOrDash(value: string | undefined | null): string {
  const trimmed = value?.trim();
  return trimmed && trimmed.length > 0 ? trimmed : "--";
}

export function sandboxUsageLabel(count: number): string {
  return count === 1 ? "1 workspace" : `${count} workspaces`;
}

export function sandboxIsDeletable(entry: SettingsSandboxEntry): boolean {
  return entry.source_metadata.effective_source.kind !== "builtin-provider";
}
