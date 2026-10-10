// The rows the transcript projection derives (`deriveSessionRows`). Kept apart
// from the derivation so each row kind's shape reads in one place.

import type {
  SessionTimelineDataPart,
  SessionTimelinePart,
  SessionTimelineReasoningPart,
  SessionTimelineTextPart,
  SessionTimelineToolPart,
} from "@/systems/session/lib/session-timeline-parts";

import type { SessionToolGroupSummary } from "./session-timeline-summary";

/** Why a turn ended the way it did; the fold layer derives it from stop markers and failures. */
export type SessionTurnFoldCause = "settled" | "stopped" | "steer_fallback" | "failed";

export type SessionRow =
  | SessionTextRow
  | SessionReasoningRow
  | SessionDataRow
  | SessionWorkingRow
  | SessionWorkRow
  | SessionLiveToolRow
  | SessionTurnFoldRow
  | SessionChangedFilesRow
  | SessionSubagentRow
  | SessionSentMessageRow;

interface SessionBaseRow {
  id: string;
  turnId?: string;
  timestamp?: string;
}

export interface SessionTextRow extends SessionBaseRow {
  kind: "text";
  part: SessionTimelineTextPart;
  parts: SessionTimelineTextPart[];
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

export type SessionWorkEntry = SessionTimelineToolPart | SessionTimelineReasoningPart;

/** Ordered activity with an optional compact disclosure; terminal calls stay inline. */
export interface SessionWorkRow extends SessionBaseRow {
  kind: "work";
  groupId: string;
  /** Original reasoning and tool parts in chronological order. */
  entries: SessionWorkEntry[];
  /** Non-null when the run rests as a collapsed semantic summary line. */
  summary: SessionToolGroupSummary | null;
  expanded: boolean;
  /** True for the completed-tools group of the live turn (ADR-006 rule 1). */
  active: boolean;
}

/**
 * The one live tool row of the active turn (ADR-006): the calls still running
 * right now. One call reads "Running {tool} — {preview}"; several read
 * "Running N tools…" and expand to the in-flight list. A subagent is never a
 * live tool row: its card replaces the call.
 */
export interface SessionLiveToolRow extends SessionBaseRow {
  kind: "live-tool";
  /** Running calls, in order; length is the honest parallel count. */
  entries: SessionTimelineToolPart[];
  /** Parallel rows expand to list the in-flight calls. */
  expanded: boolean;
}

/** A subagent card or same-turn group (S1, S2); state lives in the roster, native inner parts nest (S8). */
export interface SessionSubagentRow extends SessionBaseRow {
  kind: "subagents";
  parts: SessionTimelineDataPart[];
  /** Subagent ids in delegation order. */
  subagentIds: string[];
  /** A member is live in the roster: the row never folds (UT-W04). */
  live: boolean;
  /** Group disclosure state (`subagent-group:<first id>`). */
  expanded: boolean;
  nested: ReadonlyMap<string, readonly SessionTimelinePart[]>;
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

/**
 * A `compozy__session_prompt` call drawn as the "Sent to" card (S3), never as a
 * tool row: the daemon's `data-compozy-session-message` part names the target,
 * and the call it owns (when loaded) carries the message text and any error.
 */
export interface SessionSentMessageRow extends SessionBaseRow {
  kind: "session-message";
  part: SessionTimelineDataPart;
  toolPart: SessionTimelineToolPart | null;
}
