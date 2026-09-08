// Pure derivation of the assistant transcript into a flat `SessionRow` view.
//
// `deriveSessionRows` maps message parts to rows without mutating the runtime
// message array (ADR-006 quiet transcript). Consecutive tool parts form one run
// per turn. The live tail run — the trailing run of the active turn — derives
// exactly one live tool row for its running calls (a parallel run stays one row
// and counts) while its completed calls collapse into an expandable group; each
// running child agent is its own live row. Every settled run collapses — the
// moment it settles, even mid-turn — into a semantic summary row ("Ran 2
// commands, edited 3 files"), absorbed failures included. Interrupted calls
// stay individually visible. Consecutive reasoning parts fold into one row;
// consecutive same-kind runtime markers merge into one data row carrying a ×N
// count. Progress ticks never become rows. The settled-turn "Worked for Xs"
// fold layer lives in `./session-timeline-fold`; semantic row equality lives in
// `./session-row-equality`; the run summarizer in `./session-timeline-summary`.

import { isAgentEventPayload } from "@/systems/session/lib/message-parts";
import { CLARIFY_EVENT_TYPE } from "@/systems/session/lib/clarify-event";
import {
  isRuntimeActivityEvent,
  isSessionErrorEvent,
  isTranscriptMarkerEvent,
} from "@/systems/session/components/runtime-activity-notice.logic";

import { foldSettledTurns } from "./session-timeline-fold";
import type { SessionWorkGroupAnchor } from "./session-timeline-group-identity";
import { markerClusterKey } from "./session-timeline-markers";
import { type SessionToolGroupSummary } from "./session-timeline-summary";
import { workRowsFromCluster } from "./session-timeline-work";

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
export { isAgentToolPart, liveToolRowId } from "./session-timeline-work";

/** Why a turn ended the way it did; the fold layer derives it from stop markers and failures. */
export type SessionTurnFoldCause = "settled" | "stopped" | "steer_fallback" | "failed";

export type SessionTimelinePart =
  | SessionTimelineTextPart
  | SessionTimelineReasoningPart
  | SessionTimelineToolPart
  | SessionTimelineDataPart
  | SessionTimelineWorkingPart;

interface SessionTimelineBasePart {
  id: string;
  turnId?: string;
  timestamp?: string;
  state?: string;
  /** Position in the daemon's projected `message.parts`; search results name it (`part_index`). */
  partIndex?: number;
}

export interface SessionTimelineTextPart extends SessionTimelineBasePart {
  kind: "text";
  text: string;
}

export interface SessionTimelineReasoningPart extends SessionTimelineBasePart {
  kind: "reasoning";
  text: string;
}

export interface SessionTimelineToolPart extends SessionTimelineBasePart {
  kind: "tool";
  toolCallId: string;
  toolName: string;
  args: Record<string, unknown>;
  result?: unknown;
  isError?: boolean;
  status: "running" | "settled" | "interrupted";
}

export interface SessionTimelineDataPart extends SessionTimelineBasePart {
  kind: "data";
  name: string;
  data: unknown;
}

export interface SessionTimelineWorkingPart extends SessionTimelineBasePart {
  kind: "working";
  startedAt?: number;
}

export type SessionRow =
  | SessionTextRow
  | SessionReasoningRow
  | SessionDataRow
  | SessionWorkingRow
  | SessionWorkRow
  | SessionLiveToolRow
  | SessionTurnFoldRow
  | SessionChangedFilesRow;

interface SessionBaseRow {
  id: string;
  turnId?: string;
  timestamp?: string;
}

export interface SessionTextRow extends SessionBaseRow {
  kind: "text";
  part: SessionTimelineTextPart;
}

export interface SessionReasoningRow extends SessionBaseRow {
  kind: "reasoning";
  /** The consecutive reasoning parts folded into this row, in order. */
  parts: SessionTimelineReasoningPart[];
  /** Grouped reasoning text (parts joined by a blank line), rendered as markdown. */
  text: string;
  /** How many reasoning updates this row groups (>= 1). */
  updateCount: number;
  /** True while any grouped part is still streaming; drives the live shimmer. */
  streaming: boolean;
}

export interface SessionDataRow extends SessionBaseRow {
  kind: "data";
  /** First clustered part — the render anchor for every data payload. */
  part: SessionTimelineDataPart;
  /** Consecutive same-kind marker parts folded into this row, in order (>= 1). */
  parts: SessionTimelineDataPart[];
  /** How many same-kind events this row clusters; drives the "×N" marker count. */
  count: number;
}

export interface SessionWorkingRow extends SessionBaseRow {
  kind: "working";
  startedAt?: number;
}

/**
 * Settled tool work: a run resting as one semantic summary line (`summary`
 * non-null, expandable to its rows) or the individual rows of a run too small
 * or too particular to summarize (a lone call, interrupted calls, terminal
 * blocks). `active` marks the completed-tools group of the live turn.
 */
export interface SessionWorkRow extends SessionBaseRow {
  kind: "work";
  groupId: string;
  /** Every tool part in this run, in order. */
  entries: SessionTimelineToolPart[];
  /** Non-null when the run rests as a collapsed semantic summary line. */
  summary: SessionToolGroupSummary | null;
  expanded: boolean;
  /** True for the completed-tools group of the live turn (ADR-006 rule 1). */
  active: boolean;
}

/**
 * The one live tool row of the active turn (ADR-006): the calls still running
 * right now. One call reads "Running {tool} — {preview}"; several read
 * "Running N tools…" and expand to the in-flight list. A running child agent is
 * its own live row (`agent`), never counted into the parallel row.
 */
export interface SessionLiveToolRow extends SessionBaseRow {
  kind: "live-tool";
  /** Running calls, in order; length is the honest parallel count. */
  entries: SessionTimelineToolPart[];
  /** True when this row is a single running child agent (bot glyph, never grouped). */
  agent: boolean;
  /** Parallel rows expand to list the in-flight calls. */
  expanded: boolean;
}

export interface SessionTurnFoldRow extends SessionBaseRow {
  kind: "turn-fold";
  label: string;
  durationMs: number;
  /** Why the turn ended the way it did; drives the label, the tone, and whether it can collapse. */
  cause: SessionTurnFoldCause;
  /** Summary sentence of the tool work behind the fold; `null` when the fold hides no tools. */
  counts: string | null;
  /** True when the fold never collapses: the operator keeps their place (stopped/failed). */
  open: boolean;
  rows: SessionRow[];
}

/** One file modified by a settled turn, with derived line-diff stats. */
export interface ChangedFileEntry {
  path: string;
  additions: number;
  deletions: number;
}

// Per-turn audit summary of the files an assistant turn modified (Edit/Write).
// Rendered once at the tail of a settled editing turn as a collapsed
// "Edited N files +a −d" line that expands to the per-file list — display-only
// (CompozyOS exposes no checkpoint/Undo semantics).
export interface SessionChangedFilesRow extends SessionBaseRow {
  kind: "changed-files";
  /** Distinct modified files in first-touch order; same path edited twice is one entry. */
  files: ChangedFileEntry[];
  additions: number;
  deletions: number;
  expanded: boolean;
}

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
  const rows = deriveBaseRows(markInterruptedCalls(parts, options), options);
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

// A call that was still running when the operator stopped its turn never gets a
// result. It reads "stopped" (ADR-009), not "running" and not "failed": the
// stop was the operator's, nothing broke. Only turns the fold layer already
// knows as interrupted/superseded qualify; the live turn keeps its running row.
function markInterruptedCalls(
  parts: readonly SessionTimelinePart[],
  options: DeriveSessionRowsOptions
): readonly SessionTimelinePart[] {
  const stopped = new Set<string>([
    ...(options.interruptedTurnIds ?? []),
    ...(options.supersededTurnIds ?? []),
  ]);
  if (stopped.size === 0) return parts;
  let changed = false;
  const marked = parts.map(part => {
    if (
      part.kind !== "tool" ||
      part.status !== "running" ||
      !part.turnId ||
      !stopped.has(part.turnId) ||
      part.turnId === options.activeTurnId
    ) {
      return part;
    }
    changed = true;
    return { ...part, status: "interrupted" as const };
  });
  return changed ? marked : parts;
}

function deriveBaseRows(
  parts: readonly SessionTimelinePart[],
  options: DeriveSessionRowsOptions
): SessionRow[] {
  const rows: SessionRow[] = [];
  const usedWorkGroupIds = new Set<string>();
  const liveTailStartId = findLiveTailClusterStart(parts, options.activeTurnId);
  let toolCluster: SessionTimelineToolPart[] = [];
  let reasoningCluster: SessionTimelineReasoningPart[] = [];
  let markerCluster: SessionTimelineDataPart[] = [];
  let markerKey: string | null = null;

  const flushToolCluster = () => {
    if (toolCluster.length === 0) return;
    rows.push(...workRowsFromCluster(toolCluster, options, liveTailStartId, usedWorkGroupIds));
    toolCluster = [];
  };

  const flushReasoningCluster = () => {
    if (reasoningCluster.length === 0) return;
    rows.push(reasoningRowFromCluster(reasoningCluster));
    reasoningCluster = [];
  };

  const flushMarkerCluster = () => {
    if (markerCluster.length === 0) return;
    rows.push(dataRowFromCluster(markerCluster));
    markerCluster = [];
    markerKey = null;
  };

  for (const part of parts) {
    if (part.kind === "data" && isProgressTick(part)) {
      continue;
    }
    if (part.kind === "tool") {
      flushReasoningCluster();
      flushMarkerCluster();
      const previous = toolCluster.at(-1);
      if (previous && previous.turnId !== part.turnId) {
        flushToolCluster();
      }
      toolCluster.push(part);
      continue;
    }

    if (part.kind === "reasoning") {
      flushToolCluster();
      flushMarkerCluster();
      const previous = reasoningCluster.at(-1);
      if (previous && previous.turnId !== part.turnId) {
        flushReasoningCluster();
      }
      reasoningCluster.push(part);
      continue;
    }

    if (part.kind === "data") {
      const key = markerClusterKey(part);
      if (key !== null) {
        flushToolCluster();
        flushReasoningCluster();
        const previous = markerCluster.at(-1);
        if (previous && (previous.turnId !== part.turnId || markerKey !== key)) {
          flushMarkerCluster();
        }
        markerCluster.push(part);
        markerKey = key;
        continue;
      }
    }

    flushToolCluster();
    flushReasoningCluster();
    flushMarkerCluster();
    rows.push(rowFromPart(part));
  }
  flushToolCluster();
  flushReasoningCluster();
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

// The live tail is the trailing tool run of the part list — the run new calls
// still append to. It stays open while every earlier run collapses the moment
// it settles, even mid-turn. A trailing run qualifies when (ignoring the
// working indicator) it ends the transcript and either belongs to the active
// turn or still has running calls.
function findLiveTailClusterStart(
  parts: readonly SessionTimelinePart[],
  activeTurnId: string | undefined
): string | null {
  let index = parts.length - 1;
  while (index >= 0 && parts[index]!.kind === "working") index -= 1;
  const last = index >= 0 ? parts[index] : undefined;
  if (!last || last.kind !== "tool") return null;
  const cluster: SessionTimelineToolPart[] = [last];
  for (let scan = index - 1; scan >= 0; scan -= 1) {
    const previous = parts[scan]!;
    if (previous.kind !== "tool" || previous.turnId !== last.turnId) break;
    cluster.unshift(previous);
  }
  const hasRunning = cluster.some(tool => tool.status === "running");
  const activeTurn =
    activeTurnId !== undefined && activeTurnId !== "" && last.turnId === activeTurnId;
  return hasRunning || activeTurn ? (cluster[0]?.id ?? null) : null;
}

// Folds consecutive reasoning parts (same turn, uninterrupted by other kinds)
// into one row: the grouped text is the parts joined by a blank line so nothing
// is lost, `updateCount` counts the grouped updates, and `streaming` stays
// true while any part is still live.
function reasoningRowFromCluster(parts: SessionTimelineReasoningPart[]): SessionReasoningRow {
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
