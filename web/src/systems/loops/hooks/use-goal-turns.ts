import { useInfiniteQuery } from "@tanstack/react-query";

import { goalTurnsOptions } from "../lib/query-options";
import type { GoalTurn, GoalTurnFilter } from "../types";
import type { LoopGoalTurnLive } from "../lib/loop-events";

interface UseGoalTurnsOptions extends Pick<GoalTurnFilter, "node" | "item" | "limit"> {
  enabled?: boolean;
}

export function useGoalTurns(
  workspaceId: string,
  runId: string,
  options: UseGoalTurnsOptions = {}
) {
  const { enabled = true, node, item, limit = 50 } = options;
  return useInfiniteQuery({
    ...goalTurnsOptions(workspaceId, runId, { node, item, limit }),
    enabled: enabled && workspaceId !== "" && runId !== "",
    select: data => ({
      ...data,
      turns: mergeGoalTurns(data.pages.flatMap(page => page.turns)),
    }),
  });
}

export function mergeGoalTurns(...groups: readonly GoalTurn[][]): GoalTurn[] {
  const turns = new Map<string, GoalTurn>();
  for (const turn of groups.flat()) {
    const key = `${turn.node_id}:${turn.item_index}:${turn.generation}:${turn.turn}:${turn.prompt_attempt}:${turn.prompt_id}`;
    const current = turns.get(key);
    if (!current || (current.result_status === null && turn.result_status !== null)) {
      turns.set(key, turn);
    }
  }
  return [...turns.values()].sort((left, right) => left.seq - right.seq);
}

export interface GoalTurnTimelineItem {
  key: string;
  seq: number;
  generation: number;
  nodeId: string;
  itemIndex: number;
  turn: number;
  promptAttempt: number;
  promptId: string;
  sessionId: string;
  resultStatus: string | null;
  stopReason: string | null;
  reasonCode: string | null;
  verdictOutcome: string | null;
  blockingIssues: { id: string; note: string }[];
  evidenceRef: string | null;
  tokensUsed: number | null;
  startedAt: string;
  endedAt: string | null;
}

function turnKey(turn: {
  nodeId: string;
  itemIndex: number;
  generation: number;
  turn: number;
  promptAttempt: number;
  promptId: string;
}) {
  return `${turn.nodeId}:${turn.itemIndex}:${turn.generation}:${turn.turn}:${turn.promptAttempt}:${turn.promptId}`;
}

function apiTimelineItem(turn: GoalTurn): GoalTurnTimelineItem {
  const item = {
    seq: turn.seq,
    generation: turn.generation,
    nodeId: turn.node_id,
    itemIndex: turn.item_index,
    turn: turn.turn,
    promptAttempt: turn.prompt_attempt,
    promptId: turn.prompt_id,
    sessionId: turn.session_id,
    resultStatus: turn.result_status,
    stopReason: turn.stop_reason,
    reasonCode: turn.reason_code,
    verdictOutcome: turn.verdict_outcome,
    blockingIssues: turn.blocking_issues,
    evidenceRef: turn.evidence_ref,
    tokensUsed: turn.tokens_used,
    startedAt: turn.started_at,
    endedAt: turn.ended_at,
  };
  return { ...item, key: turnKey(item) };
}

function liveTimelineItem(turn: LoopGoalTurnLive): GoalTurnTimelineItem {
  const item = {
    seq: turn.seq,
    generation: turn.generation,
    nodeId: turn.nodeId,
    itemIndex: turn.itemIndex,
    turn: turn.turn,
    promptAttempt: turn.promptAttempt,
    promptId: turn.promptId,
    sessionId: turn.sessionId,
    resultStatus: turn.resultStatus,
    stopReason: turn.stopReason,
    reasonCode: turn.reasonCode,
    verdictOutcome: turn.verdictOutcome,
    blockingIssues: turn.blockingIssues,
    evidenceRef: turn.evidenceRef,
    tokensUsed: turn.tokensUsed,
    startedAt: turn.startedAt,
    endedAt: turn.endedAt,
  };
  return { ...item, key: turnKey(item) };
}

export function mergeGoalTurnTimeline(
  persisted: readonly GoalTurn[],
  live: readonly LoopGoalTurnLive[]
): GoalTurnTimelineItem[] {
  const merged = new Map<string, GoalTurnTimelineItem>();
  for (const turn of persisted.map(apiTimelineItem)) merged.set(turn.key, turn);
  for (const turn of live.map(liveTimelineItem)) {
    const current = merged.get(turn.key);
    if (!current || current.resultStatus === null || turn.resultStatus !== null) {
      merged.set(turn.key, turn);
    }
  }
  return [...merged.values()].sort((left, right) => left.seq - right.seq);
}

export type { UseGoalTurnsOptions };
