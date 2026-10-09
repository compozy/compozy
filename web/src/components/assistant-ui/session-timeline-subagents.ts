// Subagent projection for the parent transcript (`_uiux.md` S1, S2, S8). The
// daemon projects one `data-compozy-subagent` part per delegation or
// provider-native Agent/Task call; the part carries ids and display hints, the
// live roster carries state. Three client-side rules live here:
// - a tool call that has a card is drawn as the card, never as a tool row (a
//   failed delegate has no card and stays a failed tool row);
// - parts attributed to a provider-native card (`parentToolCallId`) leave the
//   parent flow and render inside that card;
// - adjacent cards of one turn form one group.

import type { SubagentView } from "@/systems/session/components/subagents/types";
import type { SubagentRoster } from "@/systems/session/lib/subagent-roster";
import { isRecord, stringField } from "@/systems/session/lib/timeline-message-parts";

import type {
  SessionTimelineDataPart,
  SessionTimelinePart,
} from "@/systems/session/lib/session-timeline-parts";

import type { SessionSubagentRow } from "./session-timeline.logic";

export const SUBAGENT_PART_NAME = "data-compozy-subagent";

/** `UISubagentPayload` (`internal/transcript/ui_messages_subagent.go`). */
export interface SubagentPartData {
  subagent_id: string;
  tool_call_id: string;
  origin?: string;
  title?: string;
  runtime_provider?: string;
  runtime_agent?: string;
  runtime_model?: string;
}

export function subagentPartData(part: SessionTimelinePart): SubagentPartData | null {
  if (part.kind !== "data" || part.name !== SUBAGENT_PART_NAME || !isRecord(part.data)) {
    return null;
  }
  const subagentId = stringField(part.data, "subagent_id")?.trim();
  if (!subagentId) return null;
  return {
    ...(part.data as Partial<SubagentPartData>),
    subagent_id: subagentId,
    tool_call_id: stringField(part.data, "tool_call_id")?.trim() ?? "",
  };
}

export function isSubagentPart(part: SessionTimelinePart): boolean {
  return subagentPartData(part) !== null;
}

export interface SubagentPartition {
  /** The parent flow: card-owned tool calls and native inner parts removed. */
  flow: readonly SessionTimelinePart[];
  /** Inner parts of each provider-native card, keyed by subagent id, in order. */
  nested: ReadonlyMap<string, readonly SessionTimelinePart[]>;
}

const NO_NESTED: ReadonlyMap<string, readonly SessionTimelinePart[]> = new Map();

/**
 * Split card-owned parts out of the parent flow. A part whose
 * `parentToolCallId` names no card here stays in the parent flow
 * (US-014.EC-2).
 */
export function partitionSubagentParts(parts: readonly SessionTimelinePart[]): SubagentPartition {
  const cardToolCalls = new Set<string>();
  const nativeByToolCall = new Map<string, string>();
  for (const part of parts) {
    const data = subagentPartData(part);
    if (!data || data.tool_call_id === "") continue;
    cardToolCalls.add(data.tool_call_id);
    if (data.origin === "provider_native")
      nativeByToolCall.set(data.tool_call_id, data.subagent_id);
  }
  if (cardToolCalls.size === 0) return { flow: parts, nested: NO_NESTED };

  const flow: SessionTimelinePart[] = [];
  const nested = new Map<string, SessionTimelinePart[]>();
  for (const part of parts) {
    const owner = part.parentToolCallId ? nativeByToolCall.get(part.parentToolCallId) : undefined;
    if (owner) {
      const inner = nested.get(owner);
      if (inner) inner.push(part);
      else nested.set(owner, [part]);
      continue;
    }
    if (part.kind === "tool" && part.isError !== true && cardToolCalls.has(part.toolCallId)) {
      continue;
    }
    flow.push(part);
  }
  return { flow, nested };
}

/** One card, or the group's disclosure id keyed by its first member (UT-W03). */
export function subagentRowId(subagentIds: readonly string[]): string {
  const first = subagentIds[0] ?? "none";
  return subagentIds.length > 1 ? `subagent-group:${first}` : `subagent:${first}`;
}

export interface SubagentRowOptions {
  settledSubagentIds?: ReadonlySet<string>;
  expandedWorkGroupIds?: ReadonlySet<string>;
}

/** A card or same-turn group; `live` exempts it from turn folds (UT-W04). */
export function subagentRowFromParts(
  parts: readonly SessionTimelineDataPart[],
  nested: ReadonlyMap<string, readonly SessionTimelinePart[]>,
  options: SubagentRowOptions
): SessionSubagentRow {
  const first = parts[0]!;
  const subagentIds = parts.map(part => subagentPartData(part)?.subagent_id ?? part.id);
  const id = subagentRowId(subagentIds);
  const ownNested = new Map<string, readonly SessionTimelinePart[]>();
  for (const subagentId of subagentIds) {
    const inner = nested.get(subagentId);
    if (inner) ownNested.set(subagentId, inner);
  }
  return {
    kind: "subagents",
    id,
    turnId: first.turnId,
    timestamp: first.timestamp,
    parts: [...parts],
    subagentIds,
    live: subagentIds.some(subagentId => !(options.settledSubagentIds?.has(subagentId) ?? false)),
    expanded: options.expandedWorkGroupIds?.has(id) ?? false,
    nested: ownNested,
  };
}

export interface SubagentCardModel {
  subagent: SubagentView;
  /** Not confirmed by the stream: a reconnect gap, or no roster row yet. */
  stale: boolean;
}

/**
 * The card's row: the roster's when it has one; otherwise the part's hints
 * drawn as unconfirmed (no tick, no pulse, no terminal state it never
 * received, no drill-in it cannot prove — US-009.EC-2).
 */
export function subagentCardModel(
  part: SessionTimelineDataPart,
  roster: SubagentRoster
): SubagentCardModel {
  const data = subagentPartData(part);
  const id = data?.subagent_id ?? part.id;
  const row = roster.rows.find(candidate => candidate.id === id);
  if (row) return { subagent: row, stale: roster.staleIds.has(id) };
  return {
    stale: true,
    subagent: {
      id,
      parent_session_id: "",
      child_session_id: null,
      origin: data?.origin === "provider_native" ? "provider_native" : "delegated",
      title: data?.title?.trim() || "Subagent",
      status: "running",
      progress: "",
      result_preview: "",
      error: null,
      runtime: {
        agent: data?.runtime_agent ?? "",
        provider: data?.runtime_provider ?? "",
        model: data?.runtime_model ?? "",
        reasoning_effort: "",
        speed: "",
      },
      started_at: null,
      settled_at: null,
      created_at: part.timestamp ?? "",
      updated_at: part.timestamp ?? "",
    },
  };
}
