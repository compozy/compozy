import { isAgentSessionFailure } from "./session-status";
import type { SessionPayload } from "@/systems/session";

export type AgentFleetStatus = "active" | "idle";

export interface AgentFleetSignals {
  status: AgentFleetStatus;
  active: number;
  total: number;
  failed: number;
  runtimeSeconds: number;
  lastActivityMs: number | null;
}

/**
 * Single-sourced fleet-signal derivation for Overview, Sessions tab, list rows,
 * and status pills (design-spec §3.2). Do not re-derive per surface.
 */
export function deriveAgentFleetSignals(sessions: readonly SessionPayload[]): AgentFleetSignals {
  let active = 0;
  let failed = 0;
  let runtimeSeconds = 0;
  let lastActivityMs: number | null = null;

  for (const session of sessions) {
    if (session.state === "active") active += 1;
    if (isAgentSessionFailure(session)) failed += 1;

    const elapsed = session.activity?.elapsed_seconds;
    if (typeof elapsed === "number" && Number.isFinite(elapsed) && elapsed > 0) {
      runtimeSeconds += elapsed;
    }

    const candidate = session.activity?.last_activity_at ?? session.updated_at;
    if (candidate) {
      const ts = new Date(candidate).getTime();
      if (Number.isFinite(ts) && (lastActivityMs === null || ts > lastActivityMs)) {
        lastActivityMs = ts;
      }
    }
  }

  return {
    status: active > 0 ? "active" : "idle",
    active,
    total: sessions.length,
    failed,
    runtimeSeconds,
    lastActivityMs,
  };
}
