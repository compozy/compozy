import type { PillTone } from "@compozy/ui";

import type {
  SessionMCPResponse,
  SessionMCPServerRow,
  SessionRuntimeMCPRow,
} from "../adapters/session-mcp-api";
import type { SessionPayload } from "../types";

export type SessionMCPSummary = NonNullable<SessionPayload["mcp_summary"]>;

/** The one action a CompozyOS row offers in the composer popover. */
export type SessionMCPRowAction = "sign_in" | "allow";

export interface SessionMCPStateView {
  label: string;
  tone: PillTone;
  action?: SessionMCPRowAction;
  /** Rows the session cannot use right now read quieter. */
  quiet?: boolean;
}

// Provider logos are keyed by provider, which differs from the runtime id for Qwen.
const RUNTIME_PROVIDER_KEYS: Readonly<Record<string, string>> = { "qwen-code": "qwen" };

export function sessionMCPRuntimeProvider(runtime: string): string {
  return RUNTIME_PROVIDER_KEYS[runtime] ?? runtime;
}

/** The composer shows its MCP control only for a session whose payload carries a summary. */
export function sessionMCPSummary(
  session: Pick<SessionPayload, "mcp_summary">
): SessionMCPSummary | null {
  return session.mcp_summary ?? null;
}

export function sessionMCPStateView(state: string, runtime: string): SessionMCPStateView {
  switch (state) {
    case "ready":
      return { label: "Ready", tone: "success" };
    case "applies_next_message":
      return { label: "Applies on your next message", tone: "info" };
    case "needs_sign_in":
      return { label: "Needs sign-in", tone: "warning", action: "sign_in" };
    case "waiting_approval":
      return { label: "Waiting for approval", tone: "warning", action: "allow" };
    case "failed":
      return { label: "Couldn't connect", tone: "destructive" };
    case "not_available":
      return { label: "Not available here", tone: "neutral", quiet: true };
    case "loaded_by_runtime":
      return { label: `Loaded by ${runtime}`, tone: "neutral" };
    case "off_in_compozy_sessions":
      return { label: "Off in CompozyOS sessions", tone: "neutral", quiet: true };
    default:
      return { label: state, tone: "neutral" };
  }
}

/** The second line of a row: the daemon's reason when it gave one, else the tool count. */
export function sessionMCPRowDetail(row: SessionMCPServerRow): string | undefined {
  if (row.reason.trim() !== "") return row.reason;
  if (row.state === "ready" || row.state === "applies_next_message") {
    return row.tool_count === 1 ? "1 tool" : `${row.tool_count} tools`;
  }
  return undefined;
}

export interface SessionMCPControlView {
  runtimeId: string;
  runtimeName: string;
  /** Rows the session can use or the operator can fix; rows needing the operator come first. */
  compozy: readonly SessionMCPServerRow[];
  /** Rows the session's restriction excludes. */
  unavailable: readonly SessionMCPServerRow[];
  runtimeServers: readonly SessionRuntimeMCPRow[];
  /** The last reload at a message boundary could not start the runtime. */
  reloadFailed: boolean;
}

export function sessionMCPControlView(data: SessionMCPResponse): SessionMCPControlView {
  const runtimeName = data.runtime_display_name || data.runtime;
  const needsYou = (row: SessionMCPServerRow) =>
    sessionMCPStateView(row.state, runtimeName).action !== undefined;
  const available = data.compozy.filter(row => row.state !== "not_available");
  return {
    runtimeId: data.runtime,
    runtimeName,
    compozy: [...available.filter(needsYou), ...available.filter(row => !needsYou(row))],
    unavailable: data.compozy.filter(row => row.state === "not_available"),
    runtimeServers: data.runtime_servers.servers,
    reloadFailed: data.last_reload?.outcome === "failed",
  };
}

export function sessionMCPControlLabel(summary: SessionMCPSummary): string {
  const ready = `MCP servers: ${summary.usable} ready`;
  return summary.attention > 0 ? `${ready}, ${summary.attention} need you` : ready;
}

/** The popover header's caption, in order of what the operator most needs to know. */
export function sessionMCPHeaderCaption(input: { stale: boolean; pendingReload: boolean }): string {
  if (input.stale) return "Status may be out of date";
  if (input.pendingReload) return "Applies on your next message";
  return "What this session can use";
}
