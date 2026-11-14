import { z } from "zod";

import type { AgentEventPayload } from "../types";

/** The durable outcome of one MCP reload at a message boundary (`mcp-extension-hot-activation`). */
export const MCP_RELOADED_EVENT_TYPE = "session.mcp_reloaded";
/** A session's MCP summary changed; it carries no transcript line, only a reread. */
export const MCP_CHANGED_EVENT_TYPE = "session.mcp_changed";

export type SessionMCPReloadOutcome = "applied" | "replayed" | "failed";

export interface SessionMCPReloadView {
  outcome: SessionMCPReloadOutcome;
  added: readonly string[];
  removed: readonly string[];
  /** The runtime the reload started, by display name. */
  runtime?: string;
}

const reloadEventSchema = z.object({
  outcome: z.enum(["applied", "replayed", "failed"]),
  added: z.array(z.string()).nullish(),
  removed: z.array(z.string()).nullish(),
  runtime: z.string().nullish(),
});

export function isMCPReloadEventData(value: AgentEventPayload): boolean {
  return value.type === MCP_RELOADED_EVENT_TYPE;
}

/**
 * Reads the reload record the daemon keeps under `data.raw`. An unreadable record renders nothing
 * rather than a guess.
 */
export function parseMCPReloadEvent(data: AgentEventPayload): SessionMCPReloadView | null {
  const parsed = reloadEventSchema.safeParse(data.raw);
  if (!parsed.success) return null;
  return {
    outcome: parsed.data.outcome,
    added: parsed.data.added ?? [],
    removed: parsed.data.removed ?? [],
    ...(parsed.data.runtime ? { runtime: parsed.data.runtime } : {}),
  };
}

/** The detail after "MCP servers updated ·": what changed, or how the conversation continued. */
export function mcpReloadDetail(view: SessionMCPReloadView): string {
  if (view.outcome === "replayed") return "conversation continued from history";
  const parts = [
    view.added.length > 0 ? `${view.added.join(", ")} added` : null,
    view.removed.length > 0 ? `${view.removed.join(", ")} removed` : null,
  ].filter(part => part !== null);
  return parts.length > 0 ? parts.join(" · ") : "servers changed";
}
