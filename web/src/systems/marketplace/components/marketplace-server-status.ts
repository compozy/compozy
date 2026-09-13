import type { ExtensionEntry } from "@/systems/extensions";
import { formatStatusLabel, type SettingsMCPServerEntry } from "@/systems/settings";

import type { MarketplaceExtensionServer } from "../types";

type MarketplaceServerStatusTone = "success" | "warning" | "neutral";

interface MarketplaceServerStatusView {
  /** The daemon word this view was read from (`running`, `needs_authorization`, …). */
  key: string;
  label: string;
  tone: MarketplaceServerStatusTone;
}

/**
 * Daemon vocabulary for a published extension server (`MarketplaceServerPayload.status`), mapped
 * to the status-word grammar of the boards: running success · needs authorization warning ·
 * stopped/disabled hollow. Words the daemon adds later fall through neutral, never invented.
 */
const STATUS_WORDS: Record<string, Omit<MarketplaceServerStatusView, "key">> = {
  running: { label: "Running", tone: "success" },
  needs_authorization: { label: "Needs authorization", tone: "warning" },
  needs_configuration: { label: "Needs configuration", tone: "warning" },
  disabled: { label: "Disabled", tone: "neutral" },
  stopped: { label: "Stopped", tone: "neutral" },
  unknown: { label: "Unknown", tone: "neutral" },
};

function marketplaceServerStatus(status: string | undefined): MarketplaceServerStatusView | null {
  const key = status?.trim();
  if (!key) return null;
  const word = STATUS_WORDS[key];
  return word ? { key, ...word } : { key, label: formatStatusLabel(key), tone: "neutral" };
}

interface InstalledServerStatus {
  status: MarketplaceServerStatusView;
  /** The server the row's Authorize targets; null when no single server owns the word. */
  server: MarketplaceExtensionServer | null;
}

/**
 * One status word per Installed row. Readiness wins: `missing_inputs` is daemon truth that no
 * server can start; then the first server needing authorization (so Authorize targets it); then
 * the collective disabled/running words; else the first server's own word.
 */
function installedExtensionServerStatus(extension: ExtensionEntry): InstalledServerStatus | null {
  const servers = extension.mcp_servers;
  if (servers.length === 0) return null;
  if (extension.missing_inputs.length > 0) {
    return { status: marketplaceServerStatus("needs_configuration")!, server: null };
  }
  const needsAuthorization = servers.find(server => server.status === "needs_authorization");
  if (needsAuthorization) {
    return { status: marketplaceServerStatus("needs_authorization")!, server: needsAuthorization };
  }
  if (servers.every(server => server.status === "disabled")) {
    return { status: marketplaceServerStatus("disabled")!, server: servers[0] ?? null };
  }
  const running = servers.find(server => server.status === "running");
  if (running) return { status: marketplaceServerStatus("running")!, server: running };
  const first = servers[0]!;
  const status = marketplaceServerStatus(first.status);
  return status ? { status, server: first } : null;
}

const AUTH_RUNTIME_STATES = new Set([
  "auth_required",
  "auth_expired",
  "auth_invalid",
  "auth_refresh_failed",
]);

/**
 * The same projection the daemon applies to `MarketplaceServerPayload.status`, read from a live
 * Settings definition so the word moves as soon as an authorization lands.
 */
function liveExtensionServerStatus(
  entry: SettingsMCPServerEntry
): MarketplaceServerStatusView | null {
  const state = entry.runtime_status?.state;
  if (!state) return null;
  if (state === "ready") return marketplaceServerStatus("running");
  if (AUTH_RUNTIME_STATES.has(state)) return marketplaceServerStatus("needs_authorization");
  return marketplaceServerStatus("stopped");
}

export { installedExtensionServerStatus, liveExtensionServerStatus, marketplaceServerStatus };
export type { InstalledServerStatus, MarketplaceServerStatusTone, MarketplaceServerStatusView };
