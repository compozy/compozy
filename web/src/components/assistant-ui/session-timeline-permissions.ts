// Allowed permission receipts travel with their tool (#1155). Grouping reads only the
// part's recorded decision and `tool_call_id`, never the actor resolved later, so a
// receipt keeps its place while that detail arrives. Rejected, expired, pending and
// unmatched permissions stay in the flow as standalone rows.

import {
  isAgentEventPayload,
  normalizePermissionDecision,
} from "@/systems/session/lib/message-parts";
import type {
  SessionTimelineDataPart,
  SessionTimelinePart,
  SessionTimelineToolPart,
} from "@/systems/session/lib/session-timeline-parts";
import type { CompozyPermissionData } from "@/systems/session/types";

import type { SessionRow, SessionWorkApprovals } from "./session-timeline-rows";

export interface PermissionPartition {
  /** The flow without the permissions a tool in that same flow now carries. */
  flow: readonly SessionTimelinePart[];
  /** The removed permissions by tool call id, in arrival order. */
  approvals: SessionWorkApprovals;
  /** Flow parts that directly follow a removed permission: prose on either side stays apart. */
  boundaries: ReadonlySet<SessionTimelinePart>;
}

const NO_APPROVALS: SessionWorkApprovals = new Map();
const NO_BOUNDARIES: ReadonlySet<SessionTimelinePart> = new Set();

export function isCompozyPermissionData(value: unknown): value is CompozyPermissionData {
  return isAgentEventPayload(value) && typeof value.request_id === "string";
}

function allowedToolCallId(part: SessionTimelineDataPart): string | null {
  if (part.name !== "data-compozy-permission" || !isCompozyPermissionData(part.data)) return null;
  const decision = normalizePermissionDecision(part.data.decision);
  if (decision !== "allow-once" && decision !== "allow-always") return null;
  return part.data.tool_call_id?.trim() ? part.data.tool_call_id : null;
}

/**
 * Matches across the whole flow, not by adjacency: the ACP request can arrive before its
 * `tool_call`. A call that left the flow for a card has no row to carry the receipt, so
 * its permission stays standalone.
 */
export function partitionAllowedPermissions(
  parts: readonly SessionTimelinePart[]
): PermissionPartition {
  const tools = new Map<string, SessionTimelineToolPart>();
  for (const part of parts) {
    if (part.kind === "tool" && !tools.has(part.toolCallId)) tools.set(part.toolCallId, part);
  }

  const flow: SessionTimelinePart[] = [];
  const approvals = new Map<string, SessionTimelineDataPart[]>();
  const boundaries = new Set<SessionTimelinePart>();
  let removed = false;
  for (const part of parts) {
    const toolCallId = part.kind === "data" ? allowedToolCallId(part) : null;
    const tool = toolCallId === null ? undefined : tools.get(toolCallId);
    if (
      part.kind === "data" &&
      tool &&
      (!tool.turnId || !part.turnId || tool.turnId === part.turnId)
    ) {
      const held = approvals.get(tool.toolCallId);
      if (held) held.push(part);
      else approvals.set(tool.toolCallId, [part]);
      removed = true;
      continue;
    }
    if (removed) boundaries.add(part);
    removed = false;
    flow.push(part);
  }
  if (approvals.size === 0) {
    return { flow: parts, approvals: NO_APPROVALS, boundaries: NO_BOUNDARIES };
  }
  return { flow, approvals, boundaries };
}

/**
 * Work rows carry the receipts of their calls. The live-tool row shows only the running
 * line, so a call's receipt appears once it stops running and lands in a work row.
 */
export function attachApprovals(rows: SessionRow[], approvals: SessionWorkApprovals): SessionRow[] {
  if (approvals.size === 0) return rows;
  return rows.map(row => {
    if (row.kind !== "work") return row;
    const own = new Map<string, readonly SessionTimelineDataPart[]>();
    for (const entry of row.entries) {
      const held = entry.kind === "tool" ? approvals.get(entry.toolCallId) : undefined;
      if (entry.kind === "tool" && held) own.set(entry.toolCallId, held);
    }
    return own.size > 0 ? { ...row, approvals: own } : row;
  });
}

/** Receipt lines a work row draws when its entries are visible. */
export function approvalCount(approvals: SessionWorkApprovals | undefined): number {
  let count = 0;
  for (const held of approvals?.values() ?? []) count += held.length;
  return count;
}
