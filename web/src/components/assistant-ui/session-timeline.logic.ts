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
// layer lives in `./session-timeline-fold`; structural sharing lives in
// `./session-timeline-stable`; the run summarizer in `./session-timeline-summary`.

import { foldSettledTurns } from "./session-timeline-fold";
import { markerClusterKey } from "./session-timeline-markers";
import {
  MIN_COLLAPSIBLE_TOOL_GROUP_SIZE,
  summarizeToolGroup,
  type SessionToolGroupSummary,
} from "./session-timeline-summary";

export { markerClusterKey } from "./session-timeline-markers";

export {
  computeStableSessionRows,
  EMPTY_STABLE_SESSION_ROWS,
  sessionRowEqual,
  type StableSessionRowsState,
} from "./session-timeline-stable";
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
  expandedTurnIds?: ReadonlySet<string>;
  expandedChangedFilesIds?: ReadonlySet<string>;
  foldSettledTurns?: boolean;
}

const ACTIVE_WORK_VISIBLE_LIMIT = 4;

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
  const liveTailStartId = findLiveTailClusterStart(parts, options.activeTurnId);
  let toolCluster: SessionTimelineToolPart[] = [];
  let reasoningCluster: SessionTimelineReasoningPart[] = [];
  let markerCluster: SessionTimelineDataPart[] = [];
  let markerKey: string | null = null;

  const flushToolCluster = () => {
    if (toolCluster.length === 0) return;
    rows.push(...workRowsFromCluster(toolCluster, options, liveTailStartId));
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

function workGroupId(first: SessionTimelineToolPart): string {
  return `work:${first.turnId ?? "none"}:${first.id}`;
}

function workRowsFromCluster(
  tools: SessionTimelineToolPart[],
  options: DeriveSessionRowsOptions,
  liveTailStartId: string | null
): SessionRow[] {
  const first = tools[0];
  if (!first) return [];
  const live = first.id === liveTailStartId || tools.some(tool => tool.status === "running");
  return live ? liveWorkRows(tools, options) : settledWorkRows(tools, options);
}

// The live tail: one open run, the newest ACTIVE_WORK_VISIBLE_LIMIT rows
// visible, a "+N previous tool calls" toggle above them when the run overflows.
function liveWorkRows(
  tools: SessionTimelineToolPart[],
  options: DeriveSessionRowsOptions
): SessionRow[] {
  const first = tools[0]!;
  const groupId = workGroupId(first);
  const grouped = tools.length > ACTIVE_WORK_VISIBLE_LIMIT;
  const expanded = grouped ? (options.expandedWorkGroupIds?.has(groupId) ?? false) : false;
  const workRow: SessionWorkRow = {
    kind: "work",
    id: groupId,
    groupId,
    turnId: first.turnId,
    timestamp: first.timestamp,
    entries: [...tools],
    summary: null,
    visibleCount: grouped ? ACTIVE_WORK_VISIBLE_LIMIT : tools.length,
    grouped,
    expanded,
    active: true,
  };
  if (!grouped) {
    return [workRow];
  }
  return [
    {
      kind: "work-toggle",
      id: `${groupId}:toggle`,
      groupId,
      turnId: first.turnId,
      timestamp: first.timestamp,
      hiddenCount: tools.length - ACTIVE_WORK_VISIBLE_LIMIT,
      expanded,
    },
    workRow,
  ];
}

// Settled runs rest collapsed: summarizable stretches of 2+ fold into one
// semantic summary row, while failed/interrupted calls (and lone survivors
// between them) stay individually visible — a summary never hides a failure.
function settledWorkRows(
  tools: SessionTimelineToolPart[],
  options: DeriveSessionRowsOptions
): SessionRow[] {
  const chunks: { summarizable: boolean; entries: SessionTimelineToolPart[] }[] = [];
  for (const tool of tools) {
    const summarizable = tool.isError !== true && tool.status !== "interrupted";
    const lastChunk = chunks.at(-1);
    if (lastChunk && lastChunk.summarizable === summarizable) {
      lastChunk.entries.push(tool);
    } else {
      chunks.push({ summarizable, entries: [tool] });
    }
  }
  return chunks.map(chunk => {
    const summary =
      chunk.summarizable && chunk.entries.length >= MIN_COLLAPSIBLE_TOOL_GROUP_SIZE
        ? summarizeToolGroup(chunk.entries)
        : null;
    return settledWorkRow(chunk.entries, summary, options);
  });
}

function settledWorkRow(
  entries: SessionTimelineToolPart[],
  summary: SessionToolGroupSummary | null,
  options: DeriveSessionRowsOptions
): SessionWorkRow {
  const first = entries[0]!;
  const groupId = workGroupId(first);
  return {
    kind: "work",
    id: groupId,
    groupId,
    turnId: first.turnId,
    timestamp: first.timestamp,
    entries: [...entries],
    summary,
    visibleCount: entries.length,
    grouped: false,
    expanded: summary ? (options.expandedWorkGroupIds?.has(groupId) ?? false) : false,
    active: false,
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
