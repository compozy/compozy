import { useAuiState } from "@assistant-ui/react";
import { useState } from "react";

import {
  computeStableSessionRows,
  deriveSessionRows,
  EMPTY_STABLE_SESSION_ROWS,
  isStreamingState,
  type SessionRow,
  type SessionTimelinePart,
  type SessionTimelineWorkingPart,
  type StableSessionRowsState,
} from "../session-timeline.logic";
import { isRecord, stringField, toTimelineParts } from "../timeline-message-parts";
import type { GoalPromptMeta } from "@/systems/session/types";

// A turn is "working" while it streams: assistant-ui marks the message status
// `running`, and/or a part is still live — a text/reasoning part in a streaming
// state, or a tool call awaiting its result.
function messageStatusIsRunning(message: { status?: unknown }): boolean {
  const status = message.status;
  return isRecord(status) && stringField(status, "type") === "running";
}

function partIsLive(part: SessionTimelinePart): boolean {
  if (part.kind === "tool") {
    return part.status === "running";
  }
  return isStreamingState(part.state);
}

// The turn start anchors the "Working for Xs" timer. Parts carry ISO timestamps
// (data events, or re-attached turn metadata); take the earliest known one so the
// timer counts from when the turn began. Undefined falls back to a static label.
function earliestTimestampMs(parts: readonly SessionTimelinePart[]): number | undefined {
  let earliest: number | undefined;
  for (const part of parts) {
    if (!part.timestamp) continue;
    const ms = Date.parse(part.timestamp);
    if (!Number.isFinite(ms)) continue;
    if (earliest === undefined || ms < earliest) earliest = ms;
  }
  return earliest;
}

// When the turn is streaming, append a trailing `working` part so the timeline
// renders the streaming indicator below the turn's content; the fold keeps the
// live turn inline because a `working` row marks the group active.
function deriveWorkingPart(
  message: { id?: string; status?: unknown },
  parts: readonly SessionTimelinePart[]
): SessionTimelineWorkingPart | null {
  if (!messageStatusIsRunning(message) && !parts.some(partIsLive)) {
    return null;
  }
  const turnId = typeof message.id === "string" && message.id.length > 0 ? message.id : undefined;
  return {
    kind: "working",
    id: `${message.id ?? "message"}:working`,
    turnId,
    startedAt: earliestTimestampMs(parts),
  };
}

// The runtime signals an operator stop through a data-event `stop_reason`; map it
// to the message's turn so the fold labels the interruption and stays expanded.
// The runtime normalizes agh-event parts to `{ type: "data", name: "agh-event" }`.
const INTERRUPT_STOP_REASONS = new Set([
  "cancelled",
  "canceled",
  "interrupted",
  "aborted",
  "stopped",
]);

function isAghEventPart(part: Record<string, unknown>): boolean {
  const type = stringField(part, "type");
  if (type === "data-agh-event") return true;
  return type === "data" && stringField(part, "name") === "agh-event";
}

function interruptedTurnIds(
  content: unknown,
  fallbackTurnId: string | undefined
): ReadonlySet<string> | undefined {
  if (!Array.isArray(content)) return undefined;
  const ids = new Set<string>();
  for (const part of content) {
    if (!isRecord(part) || !isAghEventPart(part)) continue;
    const data = part.data;
    if (!isRecord(data)) continue;
    const stopReason = stringField(data, "stop_reason")?.toLowerCase();
    if (!stopReason || !INTERRUPT_STOP_REASONS.has(stopReason)) continue;
    const turnId = stringField(data, "turn_id") ?? stringField(part, "turnId") ?? fallbackTurnId;
    if (turnId) ids.add(turnId);
  }
  return ids.size > 0 ? ids : undefined;
}

function goalPromptMeta(content: unknown): GoalPromptMeta | null {
  if (!Array.isArray(content)) return null;
  for (const part of content) {
    if (!isRecord(part) || !isAghEventPart(part) || !isRecord(part.data)) continue;
    const goal = part.data.goal;
    if (!isRecord(goal)) continue;
    const kind = stringField(goal, "kind");
    const runId = stringField(goal, "run_id");
    const nodeId = stringField(goal, "node_id");
    const promptId = stringField(goal, "prompt_id");
    if (
      (kind !== "goal-work" && kind !== "goal-continuation" && kind !== "goal-compaction") ||
      !runId ||
      !nodeId ||
      !promptId
    ) {
      continue;
    }
    const numberField = (key: string) =>
      typeof goal[key] === "number" && Number.isFinite(goal[key]) ? goal[key] : 0;
    return {
      kind,
      run_id: runId,
      node_id: nodeId,
      generation: numberField("generation"),
      item_index: numberField("item_index"),
      turn: typeof goal.turn === "number" && Number.isFinite(goal.turn) ? goal.turn : null,
      prompt_attempt: numberField("prompt_attempt"),
      prompt_id: promptId,
    };
  }
  return null;
}

export function useAssistantMessageTimeline() {
  const message = useAuiState(
    state => state.message as { id?: string; content?: unknown; status?: unknown }
  );
  const baseParts = toTimelineParts(message);
  const workingPart = deriveWorkingPart(message, baseParts);
  const parts = workingPart ? [...baseParts, workingPart] : baseParts;
  const interruptedTurns = interruptedTurnIds(
    message.content,
    typeof message.id === "string" && message.id.length > 0 ? message.id : undefined
  );
  const goal = goalPromptMeta(message.content);
  const [expandedWorkGroups, setExpandedWorkGroups] = useState<Set<string>>(() => new Set());
  const [expandedTurns, setExpandedTurns] = useState<Set<string>>(() => new Set());
  const [expandedChangedFiles, setExpandedChangedFiles] = useState<Set<string>>(() => new Set());
  const [stableRows, setStableRows] = useState<StableSessionRowsState>(EMPTY_STABLE_SESSION_ROWS);

  const derivedRows = deriveSessionRows(parts, {
    foldSettledTurns: true,
    interruptedTurnIds: interruptedTurns,
    expandedWorkGroupIds: expandedWorkGroups,
    expandedTurnIds: expandedTurns,
    expandedChangedFilesIds: expandedChangedFiles,
  });
  const nextStableRows = computeStableSessionRows(derivedRows, stableRows);
  if (nextStableRows !== stableRows) {
    setStableRows(nextStableRows);
  }
  const rows = nextStableRows.result;

  return {
    expandedTurns,
    expandedWorkGroups,
    rows,
    goal,
    setExpandedChangedFiles,
    setExpandedTurns,
    setExpandedWorkGroups,
  } satisfies {
    expandedTurns: ReadonlySet<string>;
    expandedWorkGroups: ReadonlySet<string>;
    rows: SessionRow[];
    goal: GoalPromptMeta | null;
    setExpandedChangedFiles: typeof setExpandedChangedFiles;
    setExpandedTurns: typeof setExpandedTurns;
    setExpandedWorkGroups: typeof setExpandedWorkGroups;
  };
}
