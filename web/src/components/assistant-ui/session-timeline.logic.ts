// Pure derivation of the assistant transcript into a flat `SessionRow` view.
//
// `deriveSessionRows` maps message parts to rows without mutating the runtime
// message array. Consecutive tool parts form one run per turn; the live tail run
// (running calls, or the trailing run of the active turn) stays open with the
// newest `ACTIVE_WORK_VISIBLE_LIMIT` rows behind a "+N previous tool calls"
// toggle, while every settled run collapses — the moment it settles, even
// mid-turn — into a semantic summary row ("Ran 2 commands · Edited 3 files").
// Failed and interrupted calls never disappear into a summary. Consecutive
// reasoning parts fold into one row; consecutive same-kind runtime markers merge
// into one data row carrying a ×N count. The settled-turn "Worked for Xs" fold
// layer lives in `./session-timeline-fold`; semantic row equality lives in
// `./session-row-equality`; the run summarizer in `./session-timeline-summary`.

import { foldSettledTurns } from "./session-timeline-fold";
import { type SessionWorkGroupAnchor } from "./session-timeline-group-identity";
import { markerClusterKey } from "./session-timeline-markers";
import { type SessionToolGroupSummary } from "./session-timeline-summary";
import { findLiveTailClusterStart, workRowsFromCluster } from "./session-timeline-work-rows";

export { markerClusterKey } from "./session-timeline-markers";

export { sessionRowEqual } from "./session-row-equality";
export {
  classifyToolSummaryCategory,
  isSummarizableToolPart,
  MIN_COLLAPSIBLE_TOOL_GROUP_SIZE,
  summarizeToolGroup,
  type SessionToolGroupSummary,
  type SessionToolGroupSummaryPart,
  type SessionToolSummaryCategory,
} from "./session-timeline-summary";

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
  | SessionWorkToggleRow
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

export interface SessionWorkRow extends SessionBaseRow {
  kind: "work";
  groupId: string;
  /** Every tool part in this run, in order. The render slices to `visibleCount`. */
  entries: SessionTimelineToolPart[];
  /** Non-null when the settled run rests as a collapsed semantic summary line. */
  summary: SessionToolGroupSummary | null;
  /** Trailing entries visible while the live tail is collapsed. */
  visibleCount: number;
  /** True when the live tail overflows and carries a "+N previous" toggle. */
  grouped: boolean;
  expanded: boolean;
  /** True for the live tail run only (running calls, or trailing run of the active turn). */
  active: boolean;
}

export interface SessionWorkToggleRow extends SessionBaseRow {
  kind: "work-toggle";
  groupId: string;
  hiddenCount: number;
  expanded: boolean;
}

export interface SessionTurnFoldRow extends SessionBaseRow {
  kind: "turn-fold";
  label: string;
  durationMs: number;
  /** True when the turn was stopped by the operator; the fold stays expanded. */
  interrupted: boolean;
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
  interruptedTurnIds?: ReadonlySet<string>;
  expandedWorkGroupIds?: ReadonlySet<string>;
  workGroupAnchors?: ReadonlyMap<string, SessionWorkGroupAnchor>;
  expandedTurnIds?: ReadonlySet<string>;
  expandedChangedFilesIds?: ReadonlySet<string>;
  foldSettledTurns?: boolean;
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
  const rows = deriveBaseRows(parts, options);
  return options.foldSettledTurns ? foldSettledTurns(rows, options) : rows;
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

/** Trailing entries shown while the live tail is collapsed. */
export function visibleWorkEntries(row: SessionWorkRow): SessionTimelineToolPart[] {
  if (!row.grouped || row.expanded) {
    return row.entries;
  }
  return row.entries.slice(-row.visibleCount);
}
