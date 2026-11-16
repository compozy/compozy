import type { StatusDotTone } from "@compozy/ui";

import type {
  SessionMCPResponse,
  SessionMCPServerRow,
  SessionRuntimeMCPRow,
} from "../adapters/session-mcp-api";
import type { SessionPayload } from "../types";

export type SessionMCPSummary = NonNullable<SessionPayload["mcp_summary"]>;

/**
 * The one action a CompozyOS row offers in the composer popover: sign in, review a project server
 * before allowing it, or see why it is not working and check it again.
 */
export type SessionMCPRowAction = "sign_in" | "allow" | "check";

/**
 * How a row reads. One vocabulary across the composer and Settings: Ready · Sign in · Review ·
 * Not working · Off. Ready rows show their tool count instead of a word.
 */
export interface SessionMCPStateView {
  label: string;
  /** A row that needs the operator leads with a dot of this tone. */
  attention?: StatusDotTone;
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

export function sessionMCPToolCount(count: number): string {
  return count === 1 ? "1 tool" : `${count} tools`;
}

export function sessionMCPStateView(state: string): SessionMCPStateView {
  switch (state) {
    case "ready":
      return { label: "Ready" };
    case "applies_next_message":
      return { label: "Next message" };
    case "needs_sign_in":
      return { label: "Sign in", attention: "warning", action: "sign_in" };
    case "waiting_approval":
      return { label: "Review", attention: "warning", action: "allow" };
    case "failed":
      return { label: "Not working", attention: "destructive", action: "check" };
    case "not_available":
      return { label: "Off", quiet: true };
    default:
      return { label: state };
  }
}

/** A runtime row's side word: nothing when it loads, where to approve it, or Off. */
export function sessionMCPRuntimeStateView(
  row: SessionRuntimeMCPRow,
  runtime: string
): SessionMCPStateView | null {
  switch (row.state) {
    case "loaded_by_runtime":
      return null;
    case "waiting_in_runtime":
      return { label: `Approve in ${runtime}`, attention: "warning" };
    case "off_in_compozy_sessions":
    case "off_in_runtime":
      return { label: "Off", quiet: true };
    default:
      return { label: row.state };
  }
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
    sessionMCPStateView(row.state).action !== undefined;
  const available = data.compozy_servers.filter(row => row.state !== "not_available");
  return {
    runtimeId: data.runtime,
    runtimeName,
    compozy: [...available.filter(needsYou), ...available.filter(row => !needsYou(row))],
    unavailable: data.compozy_servers.filter(row => row.state === "not_available"),
    runtimeServers: data.runtime_servers.servers,
    reloadFailed: data.pending_reload && data.last_reload?.outcome === "failed",
  };
}

/**
 * Whether the closed trigger reports a failed reload. A failed reload keeps its change pending until
 * a reload applies, so a summary without a pending change has recovered, whatever an earlier read
 * cached; with one, the session MCP read (made while a change is pending) carries the outcome.
 */
export function sessionMCPReloadFailed(
  summary: SessionMCPSummary,
  data: SessionMCPResponse | null
): boolean {
  return summary.pending_reload && data?.last_reload?.outcome === "failed";
}

export function sessionMCPControlLabel(
  summary: SessionMCPSummary,
  options: { reloadFailed?: boolean } = {}
): string {
  const parts = [`MCP servers: ${summary.usable} ready`];
  if (summary.attention > 0) parts.push(`${summary.attention} need you`);
  if (options.reloadFailed) parts.push("couldn't start with the new servers");
  if (summary.pending_reload) parts.push("changes apply on your next message");
  return parts.join(", ");
}

/** The popover header's note, in order of what the operator most needs to know. */
export type SessionMCPHeaderNote = "stale" | "pending" | null;

export function sessionMCPHeaderNote(input: {
  stale: boolean;
  pendingReload: boolean;
  reloadFailed: boolean;
}): SessionMCPHeaderNote {
  if (input.stale) return "stale";
  // A failed reload has its own alert below; the caption would promise what just failed.
  if (input.pendingReload && !input.reloadFailed) return "pending";
  return null;
}
