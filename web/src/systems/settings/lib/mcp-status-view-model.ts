import type { PillTone } from "@agh/ui";

import type { SettingsMCPAuthFilter, SettingsMCPServerEntry } from "../types";
import { deriveMCPManagementFilter } from "./mcp-management-target";

/**
 * Pure composition of the four INDEPENDENT MCP status signals
 * (configured x auth_status.status x runtime_status.{state,probe,tool_count})
 * into truthful, tone-mapped cells. This module owns the "no false green"
 * invariant: every rendered cell traces to a daemon payload field, and unknown
 * future daemon values render neutral while preserving the raw diagnostic.
 *
 * Vocabulary source of truth (shipped daemon, not prose):
 *  - auth:    internal/mcp/auth/types.go
 *  - runtime: internal/settings/models.go (transitional state is `auth_refresh_failed`)
 *  - probe:   internal/settings/models.go
 * The generated OpenAPI types flatten these to `string`; the local unions below
 * are the rendering contract, and the neutral fallback keeps the surface
 * forward-compatible with values the daemon may add later.
 */

export const MCP_AUTH_STATUSES = [
  "unconfigured",
  "needs_login",
  "authenticated",
  "expired",
  "invalid",
] as const;
export type MCPAuthStatus = (typeof MCP_AUTH_STATUSES)[number];

export const MCP_RUNTIME_STATES = [
  "ready",
  "config_error",
  "auth_required",
  "auth_expired",
  "auth_invalid",
  "auth_refresh_failed",
  "permission_denied",
  "runtime_unavailable",
  "dead",
] as const;
export type MCPRuntimeState = (typeof MCP_RUNTIME_STATES)[number];

export const MCP_PROBE_STATES = ["skipped", "succeeded", "failed"] as const;
export type MCPProbeState = (typeof MCP_PROBE_STATES)[number];

const DANGER_RUNTIME_STATES = new Set<string>([
  "config_error",
  "auth_invalid",
  "auth_refresh_failed",
  "permission_denied",
  "runtime_unavailable",
  "dead",
]);

export interface MCPStatusCell {
  label: string;
  tone: PillTone;
  /** Secondary mono line under the pill; a raw daemon detail, never fabricated. */
  code?: string;
}

export type MCPAuthorizeLabel = "Authorize" | "Reauthorize";

export interface MCPRowStatus {
  config: MCPStatusCell;
  auth: MCPStatusCell;
  runtime: MCPStatusCell;
  probe: MCPStatusCell;
  /** True only for OAuth-capable remotes that need repair (see isOAuthRepairable). */
  repairable: boolean;
  /** null unless `repairable`; "Authorize" for a first login, else "Reauthorize". */
  authorizeLabel: MCPAuthorizeLabel | null;
}

/** snake_case daemon value -> "Sentence case" label (matches the reference). */
export function formatStatusLabel(value: string): string {
  const spaced = value.replace(/_/g, " ").trim();
  if (spaced.length === 0) return value;
  return spaced.charAt(0).toUpperCase() + spaced.slice(1);
}

export function authTone(status: string, tokenPresent: boolean): PillTone {
  if (status === "authenticated" && tokenPresent) return "success";
  if (status === "invalid") return "danger";
  if (status === "needs_login" || status === "expired") return "warning";
  return "neutral";
}

export function runtimeTone(state: string): PillTone {
  if (state === "ready") return "success";
  if (state === "auth_required" || state === "auth_expired") return "warning";
  if (DANGER_RUNTIME_STATES.has(state)) return "danger";
  return "neutral";
}

export function probeTone(probe: string): PillTone {
  if (probe === "succeeded") return "success";
  if (probe === "failed") return "danger";
  return "neutral";
}

/**
 * A server is OAuth-capable when it is a remote (http/sse) transport carrying an
 * `oauth2_pkce` auth block. stdio and non-OAuth/header-auth remotes are never
 * OAuth-capable and never receive a browser authorize action.
 */
export function isOAuthCapable(server: SettingsMCPServerEntry): boolean {
  const remote = server.transport === "http" || server.transport === "sse";
  return remote && server.auth?.type === "oauth2_pkce";
}

/**
 * Repair (Authorize/Reauthorize) is offered only to OAuth-capable remotes that
 * are not confirmed authenticated, plus the transitional `auth_refresh_failed`
 * runtime state where a prior token is retained but a refresh failed.
 */
export function isOAuthRepairable(server: SettingsMCPServerEntry): boolean {
  if (!isOAuthCapable(server)) return false;
  const authenticated = server.auth_status?.status === "authenticated";
  const refreshFailed = server.runtime_status?.state === "auth_refresh_failed";
  return !authenticated || refreshFailed;
}

export function authorizeLabel(server: SettingsMCPServerEntry): MCPAuthorizeLabel | null {
  if (!isOAuthRepairable(server)) return null;
  return server.auth_status?.status === "needs_login" ? "Authorize" : "Reauthorize";
}

/**
 * The auth filter targets the effective definition source. A workspace collection
 * can include global definitions, so the collection row scope is not authoritative.
 */
export function deriveMCPAuthFilter(server: SettingsMCPServerEntry): SettingsMCPAuthFilter | null {
  const management = deriveMCPManagementFilter(server);
  if (!management) return null;
  return management.scope === "workspace"
    ? { scope: "workspace", workspace_id: management.workspace_id }
    : { scope: "global" };
}

/**
 * The probe/tools code line. Tool count renders ONLY on a succeeded probe with a
 * real count; a skipped probe is never a failure and never fabricates a count.
 */
export function probeToolLabel(
  probe: string | undefined,
  toolCount: number | undefined
): string | undefined {
  if (probe === "succeeded") {
    const count = typeof toolCount === "number" ? toolCount : 0;
    return `${count} ${count === 1 ? "tool" : "tools"}`;
  }
  if (probe === "skipped") return "no probe attempted";
  if (probe === "failed") return "probe attempted";
  return undefined;
}

function authCell(server: SettingsMCPServerEntry): MCPStatusCell {
  const status = server.auth_status;
  if (!status) {
    // No auth block resolved -> the server has no OAuth configuration. This is a
    // truthful neutral, not a failure (matches the reference stdio rows).
    return { label: "Unconfigured", tone: "neutral", code: "no OAuth configuration" };
  }
  const tokenPresent = Boolean(status.token_present);
  return {
    label: formatStatusLabel(status.status),
    tone: authTone(status.status, tokenPresent),
    code: status.diagnostic?.trim() || (tokenPresent ? "token present" : "token absent"),
  };
}

function runtimeCell(server: SettingsMCPServerEntry): MCPStatusCell {
  const runtime = server.runtime_status;
  if (!runtime) {
    return { label: "Unknown", tone: "neutral", code: "no runtime status reported" };
  }
  return {
    label: runtime.state === "dead" ? "Unavailable" : formatStatusLabel(runtime.state),
    tone: runtimeTone(runtime.state),
    code: runtime.diagnostic?.trim() || runtime.reason?.trim() || runtime.state,
  };
}

function probeCell(server: SettingsMCPServerEntry): MCPStatusCell {
  const runtime = server.runtime_status;
  if (!runtime) {
    return { label: "Not run", tone: "neutral" };
  }
  return {
    label: formatStatusLabel(runtime.probe),
    tone: probeTone(runtime.probe),
    code: probeToolLabel(runtime.probe, runtime.tool_count),
  };
}

/**
 * Compose one server row's four independent status cells plus its repair action.
 * The `config` signal is always "Configured" — a rendered row IS a configured
 * settings entry — but auth/runtime/probe remain distinct and never collapse
 * into a single plausible green.
 */
export function composeMCPRowStatus(server: SettingsMCPServerEntry): MCPRowStatus {
  return {
    config: { label: "Configured", tone: "neutral", code: "configured=true" },
    auth: authCell(server),
    runtime: runtimeCell(server),
    probe: probeCell(server),
    repairable: isOAuthRepairable(server),
    authorizeLabel: authorizeLabel(server),
  };
}
