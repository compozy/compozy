import {
  isInterruptedState,
  type SessionTimelinePart,
  type SessionTimelineReasoningPart,
  type SessionTimelineToolPart,
  type SessionTimelineDataPart,
} from "@/systems/session/lib/session-timeline-parts";
export {
  isInterruptedState,
  type SessionTimelinePart,
  type SessionTimelineTextPart,
  type SessionTimelineReasoningPart,
  type SessionTimelineToolPart,
  type SessionTimelineDataPart,
  type SessionTimelineWorkingPart,
} from "@/systems/session/lib/session-timeline-parts";
// Pure transcript projection. Reasoning and tools share a same-turn work
// segment; text, visible events, and deliberate terminal interactions bound it.
// Persisted parts are never changed by presentation grouping.

import { isAgentEventPayload } from "@/systems/session/lib/message-parts";
import { CLARIFY_EVENT_TYPE } from "@/systems/session/lib/clarify-event";
import {
  isRuntimeActivityEvent,
  isSessionErrorEvent,
  isTranscriptMarkerEvent,
} from "@/systems/session/lib/runtime-activity-notice";

import { foldSettledTurns } from "./session-timeline-fold";
import type { SessionWorkGroupAnchor } from "./session-timeline-group-identity";
import { markerClusterKey } from "./session-timeline-markers";
import { workRowsFromCluster } from "./session-timeline-work";
import {
  isSubagentPart,
  partitionSubagentParts,
  subagentRowFromParts,
} from "./session-timeline-subagents";
import {
  isSessionMessagePart,
  partitionSessionMessageParts,
  sentMessageRow,
} from "./session-timeline-session-messages";

export { markerClusterKey } from "./session-timeline-markers";

export { sessionRowEqual } from "./session-row-equality";
export {
  classifyToolSummaryCategory,
  isAbsorbedToolFailure,
  isSummarizableToolPart,
  MIN_COLLAPSIBLE_TOOL_GROUP_SIZE,
  summarizeToolGroup,
  summaryFailureSuffix,
  type SessionToolGroupSummary,
  type SessionToolGroupSummaryPart,
  type SessionToolSummaryCategory,
} from "./session-timeline-summary";
export { liveToolRowId } from "./session-timeline-work";

export type {
  SessionTurnFoldCause,
  SessionRow,
  SessionTextRow,
  SessionReasoningRow,
  SessionDataRow,
  SessionWorkingRow,
  SessionWorkEntry,
  SessionWorkRow,
  SessionLiveToolRow,
  SessionSubagentRow,
  SessionTurnFoldRow,
  ChangedFileEntry,
  SessionChangedFilesRow,
  SessionSentMessageRow,
} from "./session-timeline-rows";
import type {
  SessionRow,
  SessionReasoningRow,
  SessionDataRow,
  SessionWorkEntry,
} from "./session-timeline-rows";
export interface DeriveSessionRowsOptions {
  activeTurnId?: string;
  /** Turns the operator stopped (Stop / Interrupt / cancel): open, "You stopped after". */
  interruptedTurnIds?: ReadonlySet<string>;
  /** Turns a fallback steer interrupted and replaced: fold normally, cause in words. */
  supersededTurnIds?: ReadonlySet<string>;
  /** Turns that ended in a turn-level failure: open, danger label. */
  failedTurnIds?: ReadonlySet<string>;
  expandedWorkGroupIds?: ReadonlySet<string>;
  workGroupAnchors?: ReadonlyMap<string, SessionWorkGroupAnchor>;
  expandedTurnIds?: ReadonlySet<string>;
  expandedChangedFilesIds?: ReadonlySet<string>;
  foldSettledTurns?: boolean;
  /**
   * When the daemon recorded each turn's end on an event projected into a later
   * message (its quiesced receipt, the operator's cancel): the fold's duration
   * reaches to it instead of stopping at the last call this message holds.
   */
  turnEndedAtMs?: ReadonlyMap<string, number>;
  /** Roster ids reported settled: only their cards fold; unconfirmed ones read as running (UT-W04). */
  settledSubagentIds?: ReadonlySet<string>;
}
export type { SessionWorkGroupAnchor } from "./session-timeline-group-identity";

// A streaming/live part state. The runtime emits `streaming` on the wire
// (`internal/transcript/ui_messages.go`) while assistant-ui's own status object
// reads `running`; both mark a live turn. Shared by the turn-fold active check
// and the streaming-indicator injection (task 30) so the two never disagree.
export function isStreamingState(state: string | undefined): boolean {
  return state === "running" || state === "streaming";
}

export function deriveSessionRows(
  parts: readonly SessionTimelinePart[],
  options: DeriveSessionRowsOptions = {}
): SessionRow[] {
  const sent = partitionSessionMessageParts(markInterruptedCalls(parts, options));
  const { flow, nested, boundaries } = partitionSubagentParts(sent.flow);
  const separated =
    sent.boundaries.size === 0 ? boundaries : new Set([...boundaries, ...sent.boundaries]);
  const rows = deriveBaseRows(
    flow,
    { nested, boundaries: separated, sentTools: sent.tools },
    options
  );
  return options.foldSettledTurns
    ? foldSettledTurns(rows, options, turnRecordedTimes(parts, options.turnEndedAtMs))
    : rows;
}

// Every instant the daemon recorded for a turn: the calls, the rows, and the
// status events that never become rows (usage, tool_call ticks) all bound the
// turn's span — a fold reads "Worked for" over the turn's own recorded times,
// never over the rows that happened to survive derivation.
function turnRecordedTimes(
  parts: readonly SessionTimelinePart[],
  turnEndedAtMs: ReadonlyMap<string, number> | undefined
): ReadonlyMap<string, readonly number[]> {
  const times = new Map<string, number[]>();
  const record = (turnId: string, value: number) => {
    const list = times.get(turnId);
    if (list) list.push(value);
    else times.set(turnId, [value]);
  };
  for (const part of parts) {
    if (!part.turnId || !part.timestamp) continue;
    const value = Date.parse(part.timestamp);
    if (Number.isFinite(value)) record(part.turnId, value);
  }
  if (turnEndedAtMs) {
    for (const [turnId, endedAt] of turnEndedAtMs) record(turnId, endedAt);
  }
  return times;
}

/**
 * Normalize explicit cancellation states and pending calls in stopped turns.
 * A turn-level stop does not interrupt a call in the current active turn.
 */
function markInterruptedCalls(
  parts: readonly SessionTimelinePart[],
  options: DeriveSessionRowsOptions
): readonly SessionTimelinePart[] {
  const stopped = new Set<string>([
    ...(options.interruptedTurnIds ?? []),
    ...(options.supersededTurnIds ?? []),
  ]);
  let changed = false;
  const marked = parts.map(part => {
    if (part.kind !== "tool" || part.status === "interrupted") return part;
    const turnStopped =
      part.status === "running" &&
      part.turnId !== undefined &&
      stopped.has(part.turnId) &&
      part.turnId !== options.activeTurnId;
    if (!isInterruptedState(part.state) && !turnStopped) return part;
    changed = true;
    return { ...part, status: "interrupted" as const };
  });
  return changed ? marked : parts;
}

/** Group same-turn work until a visible narrative or interaction boundary. */
interface PartitionedParts {
  nested: ReadonlyMap<string, readonly SessionTimelinePart[]>;
  boundaries: ReadonlySet<SessionTimelinePart>;
  sentTools: ReadonlyMap<string, SessionTimelineToolPart>;
}

function deriveBaseRows(
  parts: readonly SessionTimelinePart[],
  { nested, boundaries, sentTools }: PartitionedParts,
  options: DeriveSessionRowsOptions
): SessionRow[] {
  const rows: SessionRow[] = [];
  const usedWorkGroupIds = new Set<string>();
  const liveTailStartId = findLiveTailClusterStart(parts, options.activeTurnId);
  let workCluster: SessionWorkEntry[] = [];
  let markerCluster: SessionTimelineDataPart[] = [];
  let markerKey: string | null = null;

  const flushWorkCluster = () => {
    if (workCluster.length === 0) return;
    rows.push(...workRowsFromCluster(workCluster, options, liveTailStartId, usedWorkGroupIds));
    workCluster = [];
  };

  const flushMarkerCluster = () => {
    if (markerCluster.length === 0) return;
    rows.push(dataRowFromCluster(markerCluster));
    markerCluster = [];
    markerKey = null;
  };

  let hiddenProgress = false;
  // A part that left the flow (card-owned call, native inner work) separates prose like its row would.
  let separated = false;
  for (const part of parts) {
    if (boundaries.has(part)) separated = true;
    if (part.kind === "data" && isProgressTick(part)) {
      hiddenProgress = true;
      continue;
    }
    const joinsProse = hiddenProgress && !separated;
    hiddenProgress = false;
    separated = false;
    if (part.kind === "tool" || part.kind === "reasoning") {
      flushMarkerCluster();
      const previous = workCluster.at(-1);
      if (previous && previous.turnId !== part.turnId) {
        flushWorkCluster();
      }
      workCluster.push(part);
      continue;
    }

    if (part.kind === "data" && isSubagentPart(part)) {
      flushWorkCluster();
      flushMarkerCluster();
      const previous = rows.at(-1);
      const adjacent = previous?.kind === "subagents" && previous.turnId === part.turnId;
      const members = adjacent ? [...previous.parts, part] : [part];
      const row = subagentRowFromParts(members, nested, options);
      if (adjacent) rows[rows.length - 1] = row;
      else rows.push(row);
      continue;
    }

    if (part.kind === "data" && isSessionMessagePart(part)) {
      flushWorkCluster();
      flushMarkerCluster();
      rows.push(sentMessageRow(part, sentTools));
      continue;
    }

    if (part.kind === "data") {
      const key = markerClusterKey(part);
      if (key !== null) {
        flushWorkCluster();
        const previous = markerCluster.at(-1);
        if (previous && (previous.turnId !== part.turnId || markerKey !== key)) {
          flushMarkerCluster();
        }
        markerCluster.push(part);
        markerKey = key;
        continue;
      }
    }

    flushWorkCluster();
    flushMarkerCluster();
    const previous = rows.at(-1);
    if (
      joinsProse &&
      part.kind === "text" &&
      previous?.kind === "text" &&
      previous.turnId === part.turnId
    ) {
      rows[rows.length - 1] = {
        ...previous,
        part: { ...previous.part, text: previous.part.text + part.text, state: part.state },
        parts: [...previous.parts, part],
      };
    } else {
      rows.push(rowFromPart(part));
    }
  }
  flushWorkCluster();
  flushMarkerCluster();
  return rows;
}

// Status events are not narrative (ADR-006): progress ticks, usage, tool-call
// receipts, hook dispatches, system notes and the like render nothing, so they
// never append a row — and never split a tool run into single rows. What stays
// is what the reader sees: markers, errors, decision asks, goal prompts and
// runtime warnings.
function isProgressTick(part: SessionTimelineDataPart): boolean {
  if (part.name !== "data-compozy-event") return false;
  const data = part.data;
  if (!isAgentEventPayload(data)) return false;
  if (data.type === "runtime_progress") return true;
  if (data.type === CLARIFY_EVENT_TYPE || data.goal) return false;
  if (isTranscriptMarkerEvent(data) || isSessionErrorEvent(data)) return false;
  if (isRuntimeActivityEvent(data)) return false;
  return true;
}

function rowFromPart(
  part: Exclude<SessionTimelinePart, SessionTimelineToolPart | SessionTimelineReasoningPart>
): SessionRow {
  switch (part.kind) {
    case "text":
      return {
        kind: "text",
        id: `text:${part.id}`,
        turnId: part.turnId,
        timestamp: part.timestamp,
        part,
        parts: [part],
      };
    case "data":
      return dataRowFromCluster([part]);
    case "working":
      return {
        kind: "working",
        id: `working:${part.id}`,
        turnId: part.turnId,
        timestamp: part.timestamp,
        startedAt: part.startedAt,
      };
  }
}

function dataRowFromCluster(parts: SessionTimelineDataPart[]): SessionDataRow {
  const first = parts[0]!;
  return {
    kind: "data",
    id: `data:${first.id}`,
    turnId: first.turnId,
    timestamp: first.timestamp,
    part: first,
    parts: [...parts],
    count: parts.length,
  };
}

/**
 * Find the same-turn work run that can still receive activity, ignoring internal
 * progress ticks and the working indicator. Earlier settled segments may fold.
 */
function findLiveTailClusterStart(
  parts: readonly SessionTimelinePart[],
  activeTurnId: string | undefined
): string | null {
  let index = parts.length - 1;
  while (index >= 0) {
    const part = parts[index]!;
    if (part.kind === "working" || (part.kind === "data" && isProgressTick(part))) {
      index -= 1;
      continue;
    }
    break;
  }
  const last = index >= 0 ? parts[index] : undefined;
  if (!last || (last.kind !== "tool" && last.kind !== "reasoning")) return null;
  const cluster: SessionWorkEntry[] = [last];
  for (let scan = index - 1; scan >= 0; scan -= 1) {
    const previous = parts[scan]!;
    if (previous.kind === "data" && isProgressTick(previous)) continue;
    if (
      (previous.kind !== "tool" && previous.kind !== "reasoning") ||
      previous.turnId !== last.turnId
    )
      break;
    cluster.unshift(previous);
  }
  const hasRunning = cluster.some(part =>
    part.kind === "tool" ? part.status === "running" : isStreamingState(part.state)
  );
  const activeTurn =
    activeTurnId !== undefined && activeTurnId !== "" && last.turnId === activeTurnId;
  return hasRunning || activeTurn ? (cluster[0]?.id ?? null) : null;
}

/**
 * Folds consecutive reasoning parts (same turn, uninterrupted by other kinds)
 * into one row: the grouped text is the parts joined by a blank line so nothing
 * is lost, `updateCount` counts the grouped updates, and `streaming` stays
 * true while any part is still live.
 */
export function reasoningRowFromCluster(
  parts: SessionTimelineReasoningPart[]
): SessionReasoningRow {
  const first = parts[0];
  const textParts: string[] = [];
  for (const part of parts) {
    if (part.text.length > 0) textParts.push(part.text);
  }
  const text = textParts.join("\n\n");
  return {
    kind: "reasoning",
    id: `reasoning:${first?.id ?? "empty"}`,
    turnId: first?.turnId,
    timestamp: first?.timestamp,
    parts: [...parts],
    text,
    updateCount: parts.length,
    streaming: parts.some(part => isStreamingState(part.state)),
  };
}
