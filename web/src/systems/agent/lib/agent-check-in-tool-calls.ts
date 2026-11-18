import type { AgentInstanceCheckIn } from "../types";

/** One audited tool call of a check-in, as `GET …/check-ins` returns it. */
export type CheckInToolCall = AgentInstanceCheckIn["tool_calls"][number];

/** The projected per-report counts (`tool_calls` on a report read). */
export interface CheckInToolSummary {
  total: number;
  writes_done: number;
  writes_maybe: number;
  denied: number;
}

/** Plain words for each closed deny code; anything else reads as blocked. */
const BLOCKED_REASON: Record<string, string> = {
  check_in_write_not_allowed: "Not allowed in check-ins",
  check_in_write_needs_approval: "Would need your approval",
  check_in_tool_not_writable: "Not available in check-ins",
  check_in_tool_budget_exhausted: "Tool limit reached",
};

export function blockedReason(code: string | undefined) {
  return BLOCKED_REASON[code ?? ""] ?? "Blocked";
}

function plural(count: number, one: string, many: string) {
  return `${count} ${count === 1 ? one : many}`;
}

/**
 * "Check-in used N tools · M changes · K blocked"; `null` when it used none, so the line hides.
 * Changes count writes that may have changed something too.
 */
export function toolCallLine(summary: CheckInToolSummary | null | undefined) {
  if (!summary || summary.total <= 0) return null;
  const changes = summary.writes_done + summary.writes_maybe;
  return [
    `Check-in used ${plural(summary.total, "tool", "tools")}`,
    changes > 0 ? plural(changes, "change", "changes") : "",
    summary.denied > 0 ? `${summary.denied} blocked` : "",
  ]
    .filter(Boolean)
    .join(" · ");
}

/** The same counts, read from an occurrence's rows; like the daemon, only allowed writes are changes. */
export function summarizeToolCalls(calls: readonly CheckInToolCall[]): CheckInToolSummary {
  const writes = calls.filter(call => call.access === "write" && call.decision === "allowed");
  return {
    total: calls.length,
    writes_done: writes.filter(call => call.effect === "done").length,
    writes_maybe: writes.filter(call => call.effect === "maybe").length,
    denied: calls.filter(call => call.decision === "denied").length,
  };
}

export interface ToolCallRowView {
  key: string;
  title: string;
  server?: string;
  access: "Read" | "Change";
  allowed: boolean;
  /** "Allowed", or the blocked reason in plain words. */
  decision: string;
  /** Set when a change may or may not have landed. */
  effect?: string;
  duration?: string;
}

/** `mcp__server__tool` and `compozy__tool` read as their tool name, spaced. */
function toolTitle(id: string) {
  const name = id.split("__").at(-1) ?? id;
  return name.replace(/_/g, " ");
}

function durationLabel(ms: number) {
  if (!ms || ms < 0) return undefined;
  return ms < 1000 ? `${ms} ms` : `${(ms / 1000).toFixed(1)} s`;
}

export function toolCallRows(calls: readonly CheckInToolCall[]): ToolCallRowView[] {
  return calls.map(call => {
    const allowed = call.decision !== "denied";
    return {
      key: String(call.seq),
      title: toolTitle(call.tool_id),
      server: call.mcp_server || undefined,
      access: call.access === "write" ? "Change" : "Read",
      allowed,
      decision: allowed ? "Allowed" : blockedReason(call.deny_reason),
      effect: call.effect === "maybe" ? "May have changed" : undefined,
      duration: durationLabel(call.duration_ms),
    };
  });
}

/** `tools_status` an occurrence ran with, in words; tools that ran normally say nothing. */
export function toolsStatusLabel(status: string | null | undefined) {
  if (status === "disabled") return "Tools off";
  if (status === "unavailable") return "Tools unavailable";
  return null;
}
