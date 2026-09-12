import { useAuiState } from "@assistant-ui/react";
import { useSelector, useStore } from "@xstate/store-react";
import { useLayoutEffect } from "react";

import { workEntryIdentity } from "../session-timeline-group-identity";
import {
  deriveSessionRows,
  isStreamingState,
  type SessionRow,
  type SessionTimelinePart,
  type SessionTimelineWorkingPart,
  type SessionWorkGroupAnchor,
} from "../session-timeline.logic";
import {
  isRecord,
  stringField,
  toTimelineParts,
} from "@/systems/session/lib/timeline-message-parts";
import { timelineRowLogic } from "./use-timeline-row-context";
import { useSessionTurnOutcomes } from "@/systems/session/hooks/use-session-turn-outcomes";
import { isAgentEventPayload } from "@/systems/session/lib/message-parts";
import type { SessionTurnOutcomes } from "@/systems/session/lib/session-turn-outcomes";
import { isSessionErrorEvent } from "@/systems/session/lib/runtime-activity-notice";
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

function messageTurnId(
  message: { id?: string },
  parts: readonly SessionTimelinePart[]
): string | undefined {
  for (let index = parts.length - 1; index >= 0; index -= 1) {
    const turnId = parts[index]?.turnId;
    if (turnId) return turnId;
  }
  return typeof message.id === "string" && message.id.length > 0 ? message.id : undefined;
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
  const turnId = messageTurnId(message, parts);
  return {
    kind: "working",
    id: `${turnId ?? message.id ?? "message"}:working`,
    turnId,
    startedAt: earliestTimestampMs(parts),
  };
}

// The runtime signals an operator stop through a data-event `stop_reason`; map it
// to the message's turn so the fold labels the interruption and stays expanded.
// The runtime normalizes CompozyOS event parts to `{ type: "data", name: "compozy-event" }`.
const INTERRUPT_STOP_REASONS = new Set([
  "cancelled",
  "canceled",
  "interrupted",
  "aborted",
  "stopped",
]);

function isCompozyEventPart(part: Record<string, unknown>): boolean {
  const type = stringField(part, "type");
  if (type === "data-compozy-event") return true;
  return type === "data" && stringField(part, "name") === "compozy-event";
}

const STEER_FALLBACK_MARKER = "transcript_marker.prompt_steered";

interface TurnEndings {
  interrupted: ReadonlySet<string> | undefined;
  superseded: ReadonlySet<string> | undefined;
  failed: ReadonlySet<string> | undefined;
}

function markerKind(data: Record<string, unknown>): string | undefined {
  const marker = data.marker;
  return isRecord(marker) ? stringField(marker, "kind") : undefined;
}

function markerEvidence(data: Record<string, unknown>, key: string): string | undefined {
  const marker = data.marker;
  if (!isRecord(marker) || !isRecord(marker.evidence)) return undefined;
  return stringField(marker.evidence, key);
}

// How each turn of the message ended, from the daemon's own records: a stop
// reason names an operator stop; a steer marker whose delivery fell back to
// interrupt names a superseded turn (its true cause, US-022.EC-3); a session
// error event or the message's own error status names a failed turn.
function turnEndings(
  content: unknown,
  fallbackTurnId: string | undefined,
  messageFailed: boolean
): TurnEndings {
  const interrupted = new Set<string>();
  const superseded = new Set<string>();
  const failed = new Set<string>();
  if (messageFailed && fallbackTurnId) failed.add(fallbackTurnId);
  if (Array.isArray(content)) {
    for (const part of content) {
      if (!isRecord(part) || !isCompozyEventPart(part)) continue;
      const data = part.data;
      if (!isRecord(data)) continue;
      const turnId = stringField(data, "turn_id") ?? stringField(part, "turnId") ?? fallbackTurnId;
      if (!turnId) continue;
      const stopReason = stringField(data, "stop_reason")?.toLowerCase();
      if (stopReason && INTERRUPT_STOP_REASONS.has(stopReason)) interrupted.add(turnId);
      if (
        markerKind(data) === STEER_FALLBACK_MARKER &&
        markerEvidence(data, "steer_delivery") === "interrupt_fallback"
      ) {
        superseded.add(turnId);
      }
      if (isAgentEventPayload(data) && isSessionErrorEvent(data)) failed.add(turnId);
    }
  }
  return {
    interrupted: interrupted.size > 0 ? interrupted : undefined,
    superseded: superseded.size > 0 ? superseded : undefined,
    failed: failed.size > 0 ? failed : undefined,
  };
}

function messageStatusFailed(message: { status?: unknown }): boolean {
  const status = message.status;
  return (
    isRecord(status) &&
    stringField(status, "type") === "incomplete" &&
    stringField(status, "reason") === "error"
  );
}

function goalPromptMeta(content: unknown): GoalPromptMeta | null {
  if (!Array.isArray(content)) return null;
  for (const part of content) {
    if (!isRecord(part) || !isCompozyEventPart(part) || !isRecord(part.data)) continue;
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

/** Choose an order-independent entry anchor for later streaming projections. */
function canonicalWorkEntryId(row: Extract<SessionRow, { kind: "work" }>): string {
  let anchor = row.entries[0] ? workEntryIdentity(row.entries[0]) : row.id;
  for (const tool of row.entries.slice(1)) {
    const identity = workEntryIdentity(tool);
    if (identity < anchor) anchor = identity;
  }
  return anchor;
}

/** Retain anchors for work disclosures, including those nested inside turn folds. */
function workGroupAnchorsFromRows(
  rows: readonly SessionRow[],
  previousAnchors: ReadonlyMap<string, SessionWorkGroupAnchor>
): SessionWorkGroupAnchor[] {
  const anchors = new Map<string, SessionWorkGroupAnchor>();
  const visit = (nestedRows: readonly SessionRow[]) => {
    for (const row of nestedRows) {
      if (row.kind === "work") {
        const previous = previousAnchors.get(row.groupId) ?? anchors.get(row.groupId);
        anchors.set(row.groupId, {
          groupId: row.groupId,
          turnId: row.turnId,
          anchorEntryId: previous?.anchorEntryId ?? canonicalWorkEntryId(row),
        });
        continue;
      }
      if (row.kind === "turn-fold") visit(row.rows);
    }
  };
  visit(rows);
  return [...anchors.values()];
}

// The daemon may record a turn's end (its quiesced receipt, the operator's
// cancel) in a later projected message than the calls it cut short, or in this
// message's own events (a `stop_reason`, the fallback steer that replaced it).
// Once the loaded thread says the turn ended, a call still awaiting its result
// here is stopped — never "running" — and the turn reads as interrupted. A
// message still streaming is the live turn and keeps its running row.
function settleEndedTurn(
  message: { status?: unknown },
  parts: readonly SessionTimelinePart[],
  outcomes: SessionTurnOutcomes,
  recordedEnded: ReadonlySet<string> | undefined
): { parts: readonly SessionTimelinePart[]; interrupted: ReadonlySet<string> | undefined } {
  if (messageStatusIsRunning(message) || (outcomes.size === 0 && !recordedEnded?.size)) {
    return { parts, interrupted: undefined };
  }
  const interrupted = new Set<string>();
  const settled = parts.map(part => {
    if (part.kind !== "tool" || part.status !== "running" || !part.turnId) return part;
    if (!outcomes.has(part.turnId) && !recordedEnded?.has(part.turnId)) return part;
    interrupted.add(part.turnId);
    return { ...part, status: "interrupted" as const };
  });
  return {
    parts: interrupted.size > 0 ? settled : parts,
    interrupted: interrupted.size > 0 ? interrupted : undefined,
  };
}

function turnEndsFromOutcomes(outcomes: SessionTurnOutcomes): ReadonlyMap<string, number> {
  const ends = new Map<string, number>();
  for (const [turnId, outcome] of outcomes) {
    if (outcome.endedAtMs !== null) ends.set(turnId, outcome.endedAtMs);
  }
  return ends;
}

function unionTurnIds(
  left: ReadonlySet<string> | undefined,
  right: ReadonlySet<string> | undefined
): ReadonlySet<string> | undefined {
  if (!left) return right;
  if (!right) return left;
  return new Set([...left, ...right]);
}

export function useAssistantMessageTimeline() {
  const message = useAuiState(
    state => state.message as { id?: string; content?: unknown; status?: unknown }
  );
  const outcomes = useSessionTurnOutcomes();
  const recorded = turnEndings(
    message.content,
    typeof message.id === "string" && message.id.length > 0 ? message.id : undefined,
    messageStatusFailed(message)
  );
  const ended = settleEndedTurn(
    message,
    toTimelineParts(message),
    outcomes,
    unionTurnIds(recorded.interrupted, recorded.superseded)
  );
  const baseParts = ended.parts;
  const workingPart = deriveWorkingPart(message, baseParts);
  const parts = workingPart ? [...baseParts, workingPart] : baseParts;
  const endings = {
    ...recorded,
    interrupted: unionTurnIds(recorded.interrupted, ended.interrupted),
  };
  const goal = goalPromptMeta(message.content);
  const timelineStore = useStore(timelineRowLogic, undefined);
  const expandedWorkGroups = useSelector(timelineStore, state => state.context.expandedWorkGroups);
  const expandedChangedFiles = useSelector(
    timelineStore,
    state => state.context.expandedChangedFiles
  );
  const workGroupAnchors = useSelector(timelineStore, state => state.context.workGroupAnchors);
  const rows = deriveSessionRows(parts, {
    activeTurnId: workingPart?.turnId,
    foldSettledTurns: true,
    interruptedTurnIds: endings.interrupted,
    supersededTurnIds: endings.superseded,
    failedTurnIds: endings.failed,
    turnEndedAtMs: turnEndsFromOutcomes(outcomes),
    expandedWorkGroupIds: expandedWorkGroups,
    workGroupAnchors,
    expandedChangedFilesIds: expandedChangedFiles,
  });
  const observedWorkGroupAnchors = workGroupAnchorsFromRows(rows, workGroupAnchors);
  useLayoutEffect(() => {
    for (const anchor of observedWorkGroupAnchors) {
      timelineStore.trigger.workGroupAnchorObserved(anchor);
    }
  }, [observedWorkGroupAnchors, timelineStore]);

  return {
    rows,
    goal,
    timelineStore,
  } satisfies {
    rows: SessionRow[];
    goal: GoalPromptMeta | null;
    timelineStore: typeof timelineStore;
  };
}
