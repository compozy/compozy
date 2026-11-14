import type { RuntimeMCPServer, RuntimeMCPSnapshot, SettingsMCPServerEntry } from "../types";
import { deriveMCPManagementFilter } from "./mcp-management-target";

/** Why a CompozyOS row waits for Allow: a new project-file server, or one changed since allowed. */
export type MCPApprovalReason = "added" | "changed";

/** Why the daemon says a row waits; a refused Allow can still learn "changed" first. */
export function mcpApprovalReason(server: SettingsMCPServerEntry): MCPApprovalReason {
  return server.approval.reason === "changed" ? "changed" : "added";
}

/** Per-row approval note: the reason it waits and the last refused Allow. */
export interface MCPApprovalNote {
  reason: MCPApprovalReason;
  error?: string | null;
}

/** Why the last copy attempt was refused, by the daemon's error code. */
export interface MCPCopyFailure {
  code?: string;
  message: string;
}

/** A group shows this many rows before "Show all N" (US-007.EC-8). */
export const MCP_GROUP_FOLD_AT = 8;

/** The runtime filter value for every group, and for the CompozyOS group alone. */
export const MCP_FILTER_ALL = "all";
export const MCP_FILTER_COMPOZY = "compozy";

/** The provider config key that owns a runtime's `runtime_mcp` (the inventory names Qwen by CLI). */
export function runtimeProviderName(runtime: string): string {
  return runtime === "qwen-code" ? "qwen" : runtime;
}

/** Providers whose own MCP servers CompozyOS sessions can turn off (`internal/config`). */
const RUNTIME_MCP_SWITCHABLE_PROVIDERS = ["claude", "copilot", "qwen"] as const;

export function providerCanDisableRuntimeMCP(provider: string): boolean {
  return (RUNTIME_MCP_SWITCHABLE_PROVIDERS as readonly string[]).includes(provider);
}

/** Stable identity for a runtime row: names repeat across scopes and files of one runtime. */
export function runtimeServerKey(server: RuntimeMCPServer): string {
  return JSON.stringify([server.runtime, server.scope, server.name, server.source_path]);
}

export function matchesMCPQuery(values: ReadonlyArray<string | undefined>, query: string): boolean {
  const normalized = query.trim().toLowerCase();
  if (normalized === "") return true;
  return values.some(value => value?.toLowerCase().includes(normalized));
}

/** A runtime is offered (filter pill and group) when it has servers or a file it couldn't read. */
export function isRuntimeOffered(snapshot: RuntimeMCPSnapshot): boolean {
  return snapshot.servers.length > 0 || snapshot.read_state !== "ok";
}

export function isRuntimeUnreadable(snapshot: RuntimeMCPSnapshot): boolean {
  return snapshot.read_state !== "ok";
}

/** The group's rows narrowed by the search text; the daemon already scoped and ordered them. */
export function searchRuntimeSnapshot(
  snapshot: RuntimeMCPSnapshot,
  query: string
): RuntimeMCPSnapshot {
  if (query.trim() === "") return snapshot;
  return {
    ...snapshot,
    servers: snapshot.servers.filter(server =>
      matchesMCPQuery([server.name, server.source_label], query)
    ),
  };
}

export interface MCPRuntimeFilterOption {
  value: string;
  /** Runtime id for the logo; absent for "All" and "CompozyOS". */
  runtime?: string;
  label: string;
  count: number;
}

/** "All N · CompozyOS N · <runtime> N …", only for runtimes with servers or an unreadable file. */
export function runtimeFilterOptions(
  compozyCount: number,
  snapshots: readonly RuntimeMCPSnapshot[]
): MCPRuntimeFilterOption[] {
  const offered = snapshots.filter(isRuntimeOffered);
  const runtimeCount = offered.reduce((sum, snapshot) => sum + snapshot.servers.length, 0);
  return [
    { value: MCP_FILTER_ALL, label: "All", count: compozyCount + runtimeCount },
    { value: MCP_FILTER_COMPOZY, label: "CompozyOS", count: compozyCount },
    ...offered.map(snapshot => ({
      value: snapshot.runtime,
      runtime: snapshot.runtime,
      label: snapshot.display_name,
      count: snapshot.servers.length,
    })),
  ];
}

/** A filter that names a runtime no longer offered falls back to "All". */
export function resolveRuntimeFilter(
  filter: string,
  options: readonly MCPRuntimeFilterOption[]
): string {
  return options.some(option => option.value === filter) ? filter : MCP_FILTER_ALL;
}

/** The `runtime_mcp` switch exists only where the runtime can turn its servers off, and is installed. */
export function runtimeSwitchAvailable(snapshot: RuntimeMCPSnapshot): boolean {
  return snapshot.runtime_mcp.can_disable && snapshot.installed;
}

export function runtimeLoadsInSessions(snapshot: RuntimeMCPSnapshot): boolean {
  return snapshot.runtime_mcp.mode !== "off";
}

/** The header's status phrase; states the fact where the switch is absent (US-009.AC-3). */
export function runtimeHeaderPhrase(snapshot: RuntimeMCPSnapshot): string {
  if (!snapshot.installed) return `${snapshot.display_name} isn't installed`;
  if (!snapshot.runtime_mcp.can_disable) {
    return `${snapshot.display_name} always loads its own servers`;
  }
  return runtimeLoadsInSessions(snapshot)
    ? "Loaded in CompozyOS sessions"
    : "Off in CompozyOS sessions";
}

const SCOPE_LABEL: Record<string, string> = {
  user: "Everywhere",
  local: "This project, only you",
  project: "This project",
  managed: "Managed",
  plugin: "Plugin",
  connector: "Connector",
};

export function runtimeScopeLabel(scope: string): string {
  return SCOPE_LABEL[scope] ?? scope;
}

const TRANSPORT_LABEL: Record<string, string> = {
  stdio: "Local",
  http: "HTTP",
  sse: "SSE",
  ws: "WebSocket",
  websocket: "WebSocket",
};

export function runtimeTransportLabel(transport: string): string {
  return TRANSPORT_LABEL[transport] ?? transport;
}

/** The file named by an unreadable group (US-007.EC-2). */
export function unreadableFileLabel(snapshot: RuntimeMCPSnapshot): string {
  return snapshot.config_files[0] ?? `${snapshot.display_name}'s settings`;
}

/** A project-file server waits for the operator's Allow (also after its definition changed). */
export function isMCPApprovalWaiting(server: SettingsMCPServerEntry): boolean {
  return server.approval?.state === "waiting";
}

/**
 * The approval identity for a waiting row: the project it was declared in and, for a profile
 * layer, that profile. Null when the daemon returned no project for it (never guessed).
 */
export function mcpApprovalTarget(
  server: SettingsMCPServerEntry
): { workspace_id: string; profile?: string } | null {
  const filter = deriveMCPManagementFilter(server);
  const workspaceId =
    (filter && "workspace_id" in filter ? filter.workspace_id : undefined) ?? server.workspace_id;
  if (!workspaceId?.trim()) return null;
  const profile = filter && "profile" in filter ? filter.profile : server.profile;
  return {
    workspace_id: workspaceId.trim(),
    ...(profile?.trim() ? { profile: profile.trim() } : {}),
  };
}
