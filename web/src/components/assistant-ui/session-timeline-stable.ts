// Structural sharing for the derived row list: reuses row references from the
// previous pass when a freshly derived row is content-equal, so unchanged rows
// stay referentially stable across derive passes (append leaves prior rows
// intact) and the virtualizer plus memoized children avoid needless re-renders.

import type {
  ChangedFileEntry,
  SessionChangedFilesRow,
  SessionDataRow,
  SessionReasoningRow,
  SessionRow,
  SessionTextRow,
  SessionTimelineDataPart,
  SessionTimelineToolPart,
  SessionTurnFoldRow,
  SessionWorkingRow,
  SessionWorkRow,
  SessionWorkToggleRow,
} from "./session-timeline.logic";

export interface StableSessionRowsState {
  byId: Map<string, SessionRow>;
  result: SessionRow[];
}

export const EMPTY_STABLE_SESSION_ROWS: StableSessionRowsState = {
  byId: new Map(),
  result: [],
};

export function computeStableSessionRows(
  rows: SessionRow[],
  previous: StableSessionRowsState
): StableSessionRowsState {
  const next = new Map<string, SessionRow>();
  let anyChanged = rows.length !== previous.byId.size;

  const result = rows.map((row, index) => {
    const previousRow = previous.byId.get(row.id);
    const stableRow = previousRow && sessionRowEqual(previousRow, row) ? previousRow : row;
    next.set(row.id, stableRow);
    if (!anyChanged && previous.result[index] !== stableRow) {
      anyChanged = true;
    }
    return stableRow;
  });

  return anyChanged ? { byId: next, result } : previous;
}

/** Shallow, per-variant content comparison — avoids serialization cost. */
export function sessionRowEqual(a: SessionRow, b: SessionRow): boolean {
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
        a.count === other.count &&
        a.timestamp === other.timestamp &&
        dataPartsEqual(a.parts, other.parts)
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
        (a.summary?.label ?? null) === (other.summary?.label ?? null) &&
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

function dataPartsEqual(
  a: readonly SessionTimelineDataPart[],
  b: readonly SessionTimelineDataPart[]
): boolean {
  if (a.length !== b.length) return false;
  return a.every((part, index) => {
    const other = b[index];
    return (
      other !== undefined &&
      part.name === other.name &&
      part.data === other.data &&
      part.state === other.state &&
      part.timestamp === other.timestamp
    );
  });
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

// CompozyOS tool inputs arrive complete (not token-streamed), so a tool's meaningful
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
    return other !== undefined && sessionRowEqual(row, other);
  });
}
