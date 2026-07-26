// Pure derivation of the assistant transcript into a flat `SessionRow` view.
//
// `deriveSessionRows` maps message parts to rows without mutating the runtime
// message array, and `computeStableSessionRows` reuses row references when content
// is unchanged so the virtualizer stays stable across derive passes. Consecutive
// tool parts fold into one `work` cluster; settled clusters show only the latest
// call behind a `+N previous tool calls` toggle, while the active turn keeps up
// to four visible. Consecutive reasoning parts fold into one `reasoning` row
// carrying an "N updates" count. The settled-turn "Worked for Xs" fold layer
// lives in `./session-timeline-fold`.

import { foldSettledTurns } from "./session-timeline-fold";

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
  /** How many reasoning updates this row groups (>= 1); drives the "N updates" count. */
  updateCount: number;
  /** True while any grouped part is still streaming; drives auto-open + the live indicator. */
  streaming: boolean;
}

export interface SessionDataRow extends SessionBaseRow {
  kind: "data";
  part: SessionTimelineDataPart;
}

export interface SessionWorkingRow extends SessionBaseRow {
  kind: "working";
  startedAt?: number;
}

export interface SessionWorkRow extends SessionBaseRow {
  kind: "work";
  groupId: string;
  /** Every tool part in the cluster, in order. The render slices to `visibleCount`. */
  entries: SessionTimelineToolPart[];
  /** How many trailing entries render while collapsed (settled=1, active turn=4). */
  visibleCount: number;
  /** True when the cluster overflows `visibleCount` and carries a toggle. */
  grouped: boolean;
  expanded: boolean;
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
// "Edited N files +a/-d" card that expands to the per-file list — display-only
// (AGH exposes no checkpoint/Undo semantics).
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
const SETTLED_WORK_VISIBLE_LIMIT = 1;

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
  let toolCluster: SessionTimelineToolPart[] = [];
  let reasoningCluster: SessionTimelineReasoningPart[] = [];

  const flushToolCluster = () => {
    if (toolCluster.length === 0) return;
    rows.push(...workRowsFromCluster(toolCluster, options));
    toolCluster = [];
  };

  const flushReasoningCluster = () => {
    if (reasoningCluster.length === 0) return;
    rows.push(reasoningRowFromCluster(reasoningCluster));
    reasoningCluster = [];
  };

  for (const part of parts) {
    if (part.kind === "tool") {
      flushReasoningCluster();
      const previous = toolCluster.at(-1);
      if (previous && previous.turnId !== part.turnId) {
        flushToolCluster();
      }
      toolCluster.push(part);
      continue;
    }

    if (part.kind === "reasoning") {
      flushToolCluster();
      const previous = reasoningCluster.at(-1);
      if (previous && previous.turnId !== part.turnId) {
        flushReasoningCluster();
      }
      reasoningCluster.push(part);
      continue;
    }

    flushToolCluster();
    flushReasoningCluster();
    rows.push(rowFromPart(part));
  }
  flushToolCluster();
  flushReasoningCluster();
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
      return {
        kind: "data",
        id: `data:${part.id}`,
        turnId: part.turnId,
        timestamp: part.timestamp,
        part,
      };
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

function workRowsFromCluster(
  tools: SessionTimelineToolPart[],
  options: DeriveSessionRowsOptions
): [SessionWorkRow] | [SessionWorkRow, SessionWorkToggleRow] {
  const activeTurnId = options.activeTurnId;
  const active =
    tools.some(tool => tool.status === "running") ||
    (activeTurnId !== undefined &&
      activeTurnId !== "" &&
      tools.some(tool => tool.turnId === activeTurnId));
  const limit = active ? ACTIVE_WORK_VISIBLE_LIMIT : SETTLED_WORK_VISIBLE_LIMIT;
  const first = tools[0];
  const groupId = `work:${first?.turnId ?? "none"}:${first?.id ?? "empty"}`;
  const grouped = tools.length > limit;
  const expanded = grouped ? (options.expandedWorkGroupIds?.has(groupId) ?? false) : false;
  const workRow: SessionWorkRow = {
    kind: "work",
    id: groupId,
    groupId,
    turnId: first?.turnId,
    timestamp: first?.timestamp,
    entries: [...tools],
    visibleCount: grouped ? limit : tools.length,
    grouped,
    expanded,
    active,
  };
  if (!grouped) {
    return [workRow];
  }
  return [
    workRow,
    {
      kind: "work-toggle",
      id: `${groupId}:toggle`,
      groupId,
      turnId: first?.turnId,
      timestamp: first?.timestamp,
      hiddenCount: tools.length - limit,
      expanded,
    },
  ];
}

// Folds consecutive reasoning parts (same turn, uninterrupted by other kinds)
// into one row: the grouped text is the parts joined by a blank line so nothing
// is lost, `updateCount` drives the "N updates" count, and `streaming` stays
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

/** Trailing entries shown while a grouped cluster is collapsed. */
export function visibleWorkEntries(row: SessionWorkRow): SessionTimelineToolPart[] {
  if (!row.grouped || row.expanded) {
    return row.entries;
  }
  return row.entries.slice(-row.visibleCount);
}

export interface StableSessionRowsState {
  byId: Map<string, SessionRow>;
  result: SessionRow[];
}

export const EMPTY_STABLE_SESSION_ROWS: StableSessionRowsState = {
  byId: new Map(),
  result: [],
};

/**
 * Reuses row references from `previous` when a freshly derived row is content-
 * equal. This keeps unchanged rows referentially stable across derive passes
 * (append leaves prior rows intact) so the virtualizer and memoized children
 * avoid needless re-renders.
 */
export function computeStableSessionRows(
  rows: SessionRow[],
  previous: StableSessionRowsState
): StableSessionRowsState {
  const next = new Map<string, SessionRow>();
  let anyChanged = rows.length !== previous.byId.size;

  const result = rows.map((row, index) => {
    const previousRow = previous.byId.get(row.id);
    const stableRow = previousRow && isRowUnchanged(previousRow, row) ? previousRow : row;
    next.set(row.id, stableRow);
    if (!anyChanged && previous.result[index] !== stableRow) {
      anyChanged = true;
    }
    return stableRow;
  });

  return anyChanged ? { byId: next, result } : previous;
}

/** Shallow, per-variant content comparison — avoids serialization cost. */
function isRowUnchanged(a: SessionRow, b: SessionRow): boolean {
  if (a.kind !== b.kind || a.id !== b.id) return false;

  switch (a.kind) {
    case "text": {
      const other = b as SessionTextRow;
      return (
        a.part.text === other.part.text &&
        a.part.state === other.part.state &&
        a.turnId === other.turnId &&
        a.timestamp === other.timestamp
      );
    }
    case "reasoning": {
      const other = b as SessionReasoningRow;
      return (
        a.text === other.text &&
        a.streaming === other.streaming &&
        a.updateCount === other.updateCount &&
        a.turnId === other.turnId &&
        a.timestamp === other.timestamp
      );
    }
    case "data": {
      const other = b as SessionDataRow;
      return (
        a.part.name === other.part.name &&
        a.part.data === other.part.data &&
        a.timestamp === other.timestamp
      );
    }
    case "working": {
      const other = b as SessionWorkingRow;
      return a.startedAt === other.startedAt && a.timestamp === other.timestamp;
    }
    case "work": {
      const other = b as SessionWorkRow;
      return (
        a.groupId === other.groupId &&
        a.active === other.active &&
        a.grouped === other.grouped &&
        a.expanded === other.expanded &&
        a.visibleCount === other.visibleCount &&
        toolEntriesEqual(a.entries, other.entries)
      );
    }
    case "work-toggle": {
      const other = b as SessionWorkToggleRow;
      return (
        a.groupId === other.groupId &&
        a.hiddenCount === other.hiddenCount &&
        a.expanded === other.expanded
      );
    }
    case "turn-fold": {
      const other = b as SessionTurnFoldRow;
      return (
        a.turnId === other.turnId &&
        a.label === other.label &&
        a.durationMs === other.durationMs &&
        a.interrupted === other.interrupted &&
        rowsEqual(a.rows, other.rows)
      );
    }
    case "changed-files": {
      const other = b as SessionChangedFilesRow;
      return (
        a.turnId === other.turnId &&
        a.additions === other.additions &&
        a.deletions === other.deletions &&
        a.expanded === other.expanded &&
        changedFilesEqual(a.files, other.files)
      );
    }
  }
}

function changedFilesEqual(
  a: readonly ChangedFileEntry[],
  b: readonly ChangedFileEntry[]
): boolean {
  if (a.length !== b.length) return false;
  return a.every((file, index) => {
    const other = b[index];
    return (
      other !== undefined &&
      file.path === other.path &&
      file.additions === other.additions &&
      file.deletions === other.deletions
    );
  });
}

// AGH tool inputs arrive complete (not token-streamed), so a tool's meaningful
// change is always accompanied by a status/state/error transition. Comparing
// those primitives — not the re-parsed `args`/`result` object references — keeps
// settled entries stable across the hook's per-message re-parse.
function toolEntriesEqual(
  a: readonly SessionTimelineToolPart[],
  b: readonly SessionTimelineToolPart[]
): boolean {
  if (a.length !== b.length) return false;
  return a.every((tool, index) => {
    const other = b[index];
    return (
      other !== undefined &&
      tool.id === other.id &&
      tool.toolCallId === other.toolCallId &&
      tool.toolName === other.toolName &&
      tool.status === other.status &&
      tool.state === other.state &&
      tool.isError === other.isError &&
      tool.timestamp === other.timestamp
    );
  });
}

function rowsEqual(a: readonly SessionRow[], b: readonly SessionRow[]): boolean {
  if (a.length !== b.length) return false;
  return a.every((row, index) => {
    const other = b[index];
    return other !== undefined && isRowUnchanged(row, other);
  });
}
