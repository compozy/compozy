// Turn-fold derivation (ADR-006 rule 2): collapses each settled turn's
// intra-turn work behind a single "Worked for {duration} · {counts}" disclosure,
// keeping the terminal assistant answer visible below. The live turn always
// stays inline. A turn the operator stopped stays open under "You stopped after
// {duration}"; a turn a fallback steer interrupted folds normally but names its
// cause; a turn that failed stays open under "Failed after {duration}" — the one
// danger the fold ever earns (ADR-009). Rich rows (decision asks, permissions,
// errors) never fold. This is a pure view over the `SessionRow` list produced by
// the base derivation.

import { formatDuration } from "@compozy/ui";

import { isAgentEventPayload } from "@/systems/session/lib/message-parts";
import {
  isSessionErrorEvent,
  isFileMutationUnverifiedEvent,
} from "@/systems/session/components/runtime-activity-notice.logic";
import { CLARIFY_EVENT_TYPE } from "@/systems/session/lib/clarify-event";
import { isDeliberateTerminalTool } from "@/systems/session/lib/session-terminal-tools";
import { aggregateChangedFiles } from "./session-timeline-changed-files";
import { summarizeToolGroup } from "./session-timeline-summary";
import {
  type DeriveSessionRowsOptions,
  isStreamingState,
  type SessionChangedFilesRow,
  type SessionRow,
  type SessionTimelineToolPart,
  type SessionTurnFoldCause,
  type SessionTurnFoldRow,
} from "./session-timeline.logic";

export function foldSettledTurns(
  rows: readonly SessionRow[],
  options: DeriveSessionRowsOptions,
  recordedTimes: ReadonlyMap<string, readonly number[]> = new Map()
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
    folded.push(...foldTurnGroup(turnId, group, options, recordedTimes.get(turnId) ?? []));
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
    if (row.kind === "work" || row.kind === "live-tool") tools.push(...row.entries);
  }
  return tools;
}

function foldTurnGroup(
  turnId: string,
  group: SessionRow[],
  options: DeriveSessionRowsOptions,
  recordedTimes: readonly number[]
): SessionRow[] {
  if (group.length === 0) {
    return group;
  }
  // The live turn always stays inline so streaming output is never hidden.
  if (turnGroupIsActive(group, turnId, options.activeTurnId)) {
    return group;
  }

  const cause = turnFoldCause(group, turnId, options);
  // A lone settled row carries no intra-turn work to fold away; a lone row of a
  // stopped, superseded or failed turn still earns its label.
  if (cause === "settled" && group.length < 2) {
    return group;
  }
  const terminal = group.at(-1)!;
  const hasTerminalText = terminal.kind === "text";
  // A completed turn folds only when it has a terminal assistant answer to anchor
  // below the disclosure; a turn that is pure tool work stays a grouped work
  // cluster (task 26). A stopped, superseded or failed turn folds even without
  // a terminal answer so its label always stands in for the missing summary.
  if (cause === "settled" && !hasTerminalText) {
    return group;
  }
  const rowsBeforeTerminal = hasTerminalText ? group.slice(0, -1) : [...group];
  const rowsInsideFold: SessionRow[] = [];
  for (const row of rowsBeforeTerminal) {
    if (!isPersistentTurnRow(row)) rowsInsideFold.push(row);
  }
  if (rowsInsideFold.length === 0) {
    return group;
  }

  const duration = turnDurationMs(group, recordedTimes);
  const counts = summarizeToolGroup(collectTurnToolParts(rowsInsideFold))?.label ?? null;
  const foldRow: SessionTurnFoldRow = {
    kind: "turn-fold",
    id: `turn-fold:${turnId}`,
    turnId,
    label: foldLabel(cause, duration, counts),
    durationMs: duration ?? 0,
    cause,
    counts,
    open: cause === "stopped" || cause === "failed",
    // The fold's own sentence already carries the counts: inside it the settled
    // calls read as their ToolCallRows, never behind a second disclosure
    // (task_07 VC-05 open body).
    rows: rowsInsideFold.map(row =>
      row.kind === "work" && row.summary ? { ...row, summary: null, expanded: false } : row
    ),
  };
  const visibleRows: SessionRow[] = [];
  let foldInserted = false;
  for (const row of rowsBeforeTerminal) {
    if (isPersistentTurnRow(row)) {
      visibleRows.push(row);
    } else if (!foldInserted) {
      visibleRows.push(foldRow);
      foldInserted = true;
    }
  }
  if (hasTerminalText) visibleRows.push(terminal);
  return visibleRows;
}

// Text, decision asks, permissions, errors and terminal evidence remain
// operator-visible after a turn settles (rich rows never fold, ADR-006).
// Deliberate terminal tool rows are their own surface, not transient work.
function isPersistentTurnRow(row: SessionRow): boolean {
  if (row.kind === "text") return true;
  if (row.kind === "work") {
    return (
      row.summary === null &&
      row.entries.length > 0 &&
      row.entries.every(entry => isDeliberateTerminalTool(entry.toolName))
    );
  }
  if (row.kind !== "data") return false;
  if (row.part.name === "data-compozy-permission") return true;
  if (row.part.name !== "data-compozy-event") return false;
  return row.parts.some(part => {
    const data = part.data;
    if (!isAgentEventPayload(data)) return false;
    if (data.type === CLARIFY_EVENT_TYPE || isSessionErrorEvent(data)) return true;
    // The verifier contradicts a completion claim: keep that unresolved failure
    // visible beside the answer even when ordinary work is folded away.
    return isFileMutationUnverifiedEvent(data);
  });
}

// Label vocabulary (artboard §05/§06/§08): the settled sentence carries the
// frozen duration and the work counts; the operator's stop swaps the verb for
// "You stopped"; a fallback steer names its cause; a failure earns danger.
function foldLabel(
  cause: SessionTurnFoldCause,
  durationMs: number | null,
  counts: string | null
): string {
  const duration = durationMs != null ? formatDuration(Math.max(1_000, durationMs)) : null;
  switch (cause) {
    case "stopped":
      return duration ? `You stopped after ${duration}` : "You stopped this response";
    case "failed":
      return duration ? `Failed after ${duration}` : "Failed";
    case "steer_fallback":
      return [
        duration ? `Interrupted after ${duration}` : "Interrupted",
        "replaced by your steer",
        ...(counts ? [counts] : []),
      ].join(" · ");
    case "settled":
      return [duration ? `Worked for ${duration}` : "Worked", ...(counts ? [counts] : [])].join(
        " · "
      );
  }
}

function turnFoldCause(
  group: readonly SessionRow[],
  turnId: string,
  options: DeriveSessionRowsOptions
): SessionTurnFoldCause {
  if (options.failedTurnIds?.has(turnId)) return "failed";
  if (options.supersededTurnIds?.has(turnId)) return "steer_fallback";
  if (turnGroupIsInterrupted(group, turnId, options.interruptedTurnIds)) return "stopped";
  return "settled";
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
    if (row.kind === "live-tool") return true;
    if (row.kind === "working") return true;
    if (row.kind === "reasoning") return row.streaming;
    if (row.kind === "text") return isStreamingState(row.part.state);
    if (row.kind === "data") return row.parts.some(part => isStreamingState(part.state));
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
    if (row.kind === "text") return isInterruptedState(row.part.state);
    if (row.kind === "data") return row.parts.some(part => isInterruptedState(part.state));
    return false;
  });
}

function isInterruptedState(state: string | undefined): boolean {
  return state === "interrupted" || state === "cancelled" || state === "canceled";
}

// The turn's span: every instant the daemon recorded for it (rows here plus the
// status events and later receipts the caller collected), first to last.
function turnDurationMs(
  group: readonly SessionRow[],
  recordedTimes: readonly number[]
): number | null {
  const values: number[] = recordedTimes.filter(value => Number.isFinite(value));
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
  if (row.kind === "work" || row.kind === "live-tool") {
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
