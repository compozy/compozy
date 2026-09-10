// Semantic row equality for the memoized timeline renderer. Fresh derivations
// may allocate new row objects; this comparator keeps unchanged row subtrees
// from rendering again without feeding derived data back into React state.

import type {
  ChangedFileEntry,
  SessionChangedFilesRow,
  SessionDataRow,
  SessionLiveToolRow,
  SessionReasoningRow,
  SessionRow,
  SessionTextRow,
  SessionTimelineDataPart,
  SessionWorkEntry,
  SessionTurnFoldRow,
  SessionWorkingRow,
  SessionWorkRow,
} from "./session-timeline.logic";

/** Shallow, per-variant content comparison — avoids serialization cost. */
export function sessionRowEqual(a: SessionRow, b: SessionRow): boolean {
  if (a.kind !== b.kind || a.id !== b.id) return false;

  switch (a.kind) {
    case "text": {
      const other = b as SessionTextRow;
      return (
        a.part.text === other.part.text &&
        a.part.state === other.part.state &&
        a.parts.length === other.parts.length &&
        a.parts.every((part, index) => part.partIndex === other.parts[index]?.partIndex) &&
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
        a.expanded === other.expanded &&
        (a.summary?.label ?? null) === (other.summary?.label ?? null) &&
        (a.summary?.failedCount ?? 0) === (other.summary?.failedCount ?? 0) &&
        workEntriesEqual(a.entries, other.entries)
      );
    }
    case "live-tool": {
      const other = b as SessionLiveToolRow;
      return (
        a.agent === other.agent &&
        a.expanded === other.expanded &&
        workEntriesEqual(a.entries, other.entries)
      );
    }
    case "turn-fold": {
      const other = b as SessionTurnFoldRow;
      return (
        a.turnId === other.turnId &&
        a.label === other.label &&
        a.durationMs === other.durationMs &&
        a.cause === other.cause &&
        a.counts === other.counts &&
        a.open === other.open &&
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

/**
 * CompozyOS tool inputs arrive complete (not token-streamed), so a tool's meaningful
 * change is always accompanied by a status/state/error transition. Comparing
 * those primitives — not the re-parsed `args`/`result` object references — keeps
 * settled entries stable across the hook's per-message re-parse.
 */
function workEntriesEqual(a: readonly SessionWorkEntry[], b: readonly SessionWorkEntry[]): boolean {
  if (a.length !== b.length) return false;
  return a.every((tool, index) => {
    const other = b[index];
    if (
      !other ||
      tool.kind !== other.kind ||
      tool.id !== other.id ||
      tool.partIndex !== other.partIndex ||
      tool.turnId !== other.turnId
    )
      return false;
    if (tool.kind === "reasoning") {
      return (
        other.kind === "reasoning" &&
        tool.text === other.text &&
        tool.state === other.state &&
        tool.timestamp === other.timestamp
      );
    }
    return (
      other.kind === "tool" &&
      tool.toolCallId === other.toolCallId &&
      tool.toolName === other.toolName &&
      tool.toolTitle === other.toolTitle &&
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
