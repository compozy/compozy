// Turn-fold derivation: collapses each settled turn's intra-turn work behind a
// single "Worked for {duration}" disclosure, keeping the terminal assistant
// answer visible below it. The live turn always stays inline, and an interrupted
// turn stays expanded with a "You stopped after {duration}" label. This is a pure
// view over the `SessionRow` list produced by the base derivation.

import { formatDuration } from "@agh/ui";

import { aggregateChangedFiles } from "./session-timeline-changed-files";
import {
  type DeriveSessionRowsOptions,
  isStreamingState,
  type SessionChangedFilesRow,
  type SessionRow,
  type SessionTimelineToolPart,
  type SessionTurnFoldRow,
} from "./session-timeline.logic";

export function foldSettledTurns(
  rows: readonly SessionRow[],
  options: DeriveSessionRowsOptions
): SessionRow[] {
  const folded: SessionRow[] = [];
  for (let index = 0; index < rows.length;) {
    const row = rows[index];
    const turnId = row?.turnId;
    if (!row || !turnId) {
      if (row) folded.push(row);
      index += 1;
      continue;
    }
    const group: SessionRow[] = [];
    while (index < rows.length && rows[index]?.turnId === turnId) {
      group.push(rows[index]!);
      index += 1;
    }
    folded.push(...foldTurnGroup(turnId, group, options));
    const changedFiles = changedFilesRowForTurn(turnId, group, options);
    if (changedFiles) folded.push(changedFiles);
  }
  return folded;
}

// The settled-turn "Changed files (N)" roll-up: aggregated from the turn's
// Edit/Write tool parts and appended once at the very tail of the turn (below the
// terminal answer / fold), so an editing turn closes with an auditable file
// summary even while its work is folded away. Live turns never get it (display-
// only settled).
function changedFilesRowForTurn(
  turnId: string,
  group: readonly SessionRow[],
  options: DeriveSessionRowsOptions
): SessionChangedFilesRow | null {
  if (turnGroupIsActive(group, turnId, options.activeTurnId)) {
    return null;
  }
  const summary = aggregateChangedFiles(collectTurnToolParts(group));
  if (!summary) return null;
  const id = `changed-files:${turnId}`;
  return {
    kind: "changed-files",
    id,
    turnId,
    files: summary.files,
    additions: summary.additions,
    deletions: summary.deletions,
    expanded: options.expandedChangedFilesIds?.has(id) ?? false,
  };
}

function collectTurnToolParts(group: readonly SessionRow[]): SessionTimelineToolPart[] {
  const tools: SessionTimelineToolPart[] = [];
  for (const row of group) {
    if (row.kind === "work") tools.push(...row.entries);
  }
  return tools;
}

function foldTurnGroup(
  turnId: string,
  group: SessionRow[],
  options: DeriveSessionRowsOptions
): SessionRow[] {
  // A lone row (or empty group) carries no intra-turn work to fold away.
  if (group.length < 2) {
    return group;
  }
  // The live turn always stays inline so streaming output is never hidden.
  if (turnGroupIsActive(group, turnId, options.activeTurnId)) {
    return group;
  }

  const interrupted = turnGroupIsInterrupted(group, turnId, options.interruptedTurnIds);
  const terminal = group.at(-1)!;
  const hasTerminalText = terminal.kind === "text";
  // A completed turn folds only when it has a terminal assistant answer to anchor
  // below the disclosure; a turn that is pure tool work stays a grouped work
  // cluster (task 26). An interrupted turn folds even without a terminal answer so
  // the "You stopped" label always stands in for the missing summary.
  if (!interrupted && !hasTerminalText) {
    return group;
  }
  const rowsBeforeTerminal = hasTerminalText ? group.slice(0, -1) : [...group];
  const rowsInsideFold: SessionRow[] = [];
  const persistentRows: SessionRow[] = [];
  for (const row of rowsBeforeTerminal) {
    if (isPersistentTurnRow(row)) {
      persistentRows.push(row);
    } else {
      rowsInsideFold.push(row);
    }
  }
  if (rowsInsideFold.length === 0) {
    return group;
  }

  const duration = turnDurationMs(group);
  const foldRow: SessionTurnFoldRow = {
    kind: "turn-fold",
    id: `turn-fold:${turnId}`,
    turnId,
    label: foldLabel(interrupted, duration),
    durationMs: duration ?? 0,
    interrupted,
    rows: rowsInsideFold,
  };
  return hasTerminalText ? [foldRow, ...persistentRows, terminal] : [foldRow, ...persistentRows];
}

// Permission rows remain operator-visible audit and decision surfaces after a
// turn settles; they are not transient reasoning or tool work.
function isPersistentTurnRow(row: SessionRow): boolean {
  return row.kind === "data" && row.part.name === "data-agh-permission";
}

// Label fallbacks: a settled turn folds even when its duration is unknown, and
// an interrupted turn swaps the "Worked for" verb for the operator-facing
// "You stopped" language.
function foldLabel(interrupted: boolean, durationMs: number | null): string {
  const duration = durationMs != null ? formatDuration(Math.max(1_000, durationMs)) : null;
  if (interrupted) {
    return duration ? `You stopped after ${duration}` : "You stopped this response";
  }
  return duration ? `Worked for ${duration}` : "Worked";
}

function turnGroupIsActive(
  group: readonly SessionRow[],
  turnId: string,
  activeTurnId: string | undefined
): boolean {
  if (activeTurnId !== undefined && activeTurnId !== "" && turnId === activeTurnId) {
    return true;
  }
  return group.some(row => {
    if (row.kind === "work") return row.active;
    if (row.kind === "working") return true;
    if (row.kind === "reasoning") return row.streaming;
    if (row.kind === "text" || row.kind === "data") {
      return isStreamingState(row.part.state);
    }
    return false;
  });
}

function turnGroupIsInterrupted(
  group: readonly SessionRow[],
  turnId: string,
  interruptedTurnIds: ReadonlySet<string> | undefined
): boolean {
  if (interruptedTurnIds?.has(turnId)) {
    return true;
  }
  return group.some(row => {
    if (row.kind === "work") {
      return row.entries.some(
        tool => tool.status === "interrupted" || isInterruptedState(tool.state)
      );
    }
    if (row.kind === "reasoning") {
      return row.parts.some(part => isInterruptedState(part.state));
    }
    if (row.kind === "text" || row.kind === "data") {
      return isInterruptedState(row.part.state);
    }
    return false;
  });
}

function isInterruptedState(state: string | undefined): boolean {
  return state === "interrupted" || state === "cancelled" || state === "canceled";
}

function turnDurationMs(group: readonly SessionRow[]): number | null {
  const values: number[] = [];
  for (const row of group) {
    for (const value of rowTimestamps(row)) {
      if (Number.isFinite(value)) values.push(value);
    }
  }
  if (values.length < 2) {
    return null;
  }
  return Math.max(0, Math.max(...values) - Math.min(...values));
}

function rowTimestamps(row: SessionRow): number[] {
  if (row.kind === "work") {
    return row.entries.map(tool => timestampMs(tool.timestamp));
  }
  if (row.kind === "turn-fold") {
    return row.rows.flatMap(rowTimestamps);
  }
  return [timestampMs(row.timestamp)];
}

function timestampMs(timestamp: string | undefined): number {
  if (!timestamp) return Number.NaN;
  const value = Date.parse(timestamp);
  return Number.isFinite(value) ? value : Number.NaN;
}
