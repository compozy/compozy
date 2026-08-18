import type { LoopRunEventFrame } from "../types";
import { loopGenerationScore } from "./loop-generation-presentation";
import { isTerminalLoopStatus } from "./loop-formatters";
import { goalNodeIds } from "./loop-graph";
import { LIFECYCLE_KINDS_ENDING_ATTEMPT, lifecycleStoryRow } from "./loop-run-story-lifecycle-rows";
import { graphStoryRow } from "./loop-run-story-request-rows";
import {
  asRecord,
  branchKey,
  durableGenerationStartedRow,
  finalizeSuccessRow,
  gateVerdictRow,
  generationStartedRow,
  humanizeLoopNodeId,
  type MutableStoryRow,
  needsApprovalRow,
  nodeFailedRow,
  num,
  runningKey,
  statusChangedRow,
  str,
  taskLink,
} from "./loop-run-story-rows";
import type { LoopRunStory, LoopRunStoryContext, LoopStoryNow } from "./loop-run-story-types";

/**
 * The event → story orchestrator (redesign spec §5.3): folds the replayed SSE
 * frames into the newest-first story rows and the "Happening now" projection.
 * Per-kind row shapes live in `loop-run-story-rows`; the "What happens next"
 * note in `loop-run-next-note`. This module re-exports the public story API so
 * `lib/loop-run-story` stays the stable import surface for consumers.
 */

interface RunningEntry {
  nodeId: string;
  itemIndex?: number;
  generation: number;
  startedAt: string;
  seq: number;
  taskId?: string;
  taskRunId?: string;
}

interface StreamedBestProjection {
  bestGeneration: number;
  generation: number;
  gateId: string;
  seq: number;
}

function latestStreamedBestProjection(
  frames: readonly LoopRunEventFrame[]
): StreamedBestProjection | null {
  let latest: StreamedBestProjection | null = null;
  for (const frame of frames) {
    if (str(frame.kind) !== "gate_verdict") continue;
    const payload = asRecord(frame.payload);
    const bestGeneration = num(payload?.best_generation);
    const generation = num(payload?.generation);
    if (bestGeneration === undefined || generation === undefined) continue;
    const projection = {
      bestGeneration,
      generation,
      gateId: str(payload?.gate_id, str(payload?.node_id)),
      seq: num(frame.seq) ?? 0,
    };
    if (!latest || projection.seq >= latest.seq) latest = projection;
  }
  return latest;
}

/**
 * Builds the story timeline + "Happening now" from the retained frames. Frames
 * arrive in seq order; rows return newest-first. Consecutive branch successes of
 * one node collapse into a single row; `node_running` never renders a row — it
 * feeds the now-card until a terminal node event or a status change clears it.
 */
export function buildRunStory(
  frames: readonly LoopRunEventFrame[],
  context: LoopRunStoryContext
): LoopRunStory {
  const live = !isTerminalLoopStatus(context.status);
  const failedOnlyLive = live && context.reattemptStrategy === "failed_only";
  const goalIds = context.graph ? goalNodeIds(context.graph) : new Set<string>();
  const running = new Map<string, RunningEntry>();
  const branches = new Map<string, Set<number>>();
  const generationByNumber = new Map(
    context.generations?.map(generation => [generation.generation, generation] as const)
  );
  const streamedBest = latestStreamedBestProjection(frames);
  const streamedVerdictIsPersisted = streamedBest
    ? (generationByNumber
        .get(streamedBest.generation)
        ?.verdicts.some(
          verdict => streamedBest.gateId === "" || verdict.gate_id === streamedBest.gateId
        ) ?? false)
    : false;
  const bestGeneration =
    streamedBest && !streamedVerdictIsPersisted
      ? streamedBest.bestGeneration
      : (context.bestGeneration ?? streamedBest?.bestGeneration);
  const eventStartedGenerations = new Set<number>();
  for (const frame of frames) {
    if (str(frame.kind) !== "generation_started") continue;
    const generation = num(asRecord(frame.payload)?.generation);
    if (generation !== undefined) eventStartedGenerations.add(generation);
  }
  // A missing generation_started frame means it fell outside retained SSE history.
  // Durable generation rows precede the retained tail and keep score/provenance anchors alive.
  const rows: MutableStoryRow[] = [...generationByNumber.values()]
    .filter(generation => !eventStartedGenerations.has(generation.generation))
    .sort((left, right) => left.generation - right.generation)
    .map(generation =>
      durableGenerationStartedRow(generation, context.reattemptStrategy, bestGeneration)
    );
  const startedGenerations = new Set(rows.map(row => row.generation ?? 0));

  const trackBranch = (generation: number, nodeId: string, itemIndex: number | undefined) => {
    if (itemIndex === undefined) return;
    const key = branchKey(generation, nodeId);
    const set = branches.get(key) ?? new Set<number>();
    set.add(itemIndex);
    branches.set(key, set);
  };

  for (const frame of frames) {
    const kind = str(frame.kind);
    const payload = asRecord(frame.payload);
    if (!payload) continue;
    switch (kind) {
      case "generation_started": {
        const generation = num(payload.generation) ?? 0;
        if (startedGenerations.has(generation)) break;
        startedGenerations.add(generation);
        rows.push(
          generationStartedRow(
            frame,
            payload,
            context.reattemptStrategy,
            loopGenerationScore(generationByNumber.get(generation)),
            bestGeneration
          )
        );
        break;
      }
      case "gate_verdict":
        rows.push(gateVerdictRow(frame, payload));
        break;
      case "needs_approval":
        rows.push(needsApprovalRow(frame, payload));
        break;
      case "node_running": {
        const nodeId = str(payload.node_id);
        if (nodeId === "") break;
        const itemIndex = num(payload.item_index);
        const generation = num(payload.generation) ?? 0;
        trackBranch(generation, nodeId, itemIndex);
        const link = taskLink(payload);
        running.set(runningKey(nodeId, itemIndex), {
          nodeId,
          itemIndex,
          generation,
          startedAt: str(frame.at),
          seq: num(frame.seq) ?? 0,
          taskId: link?.taskId,
          taskRunId: link?.taskRunId,
        });
        break;
      }
      case "node_succeeded": {
        const nodeId = str(payload.node_id);
        if (nodeId === "") break;
        const itemIndex = num(payload.item_index);
        const generation = num(payload.generation) ?? 0;
        trackBranch(generation, nodeId, itemIndex);
        running.delete(runningKey(nodeId, itemIndex));
        const previous = rows[rows.length - 1];
        if (
          previous?.group &&
          previous.group.nodeId === nodeId &&
          previous.group.generation === generation
        ) {
          previous.group.count += 1;
          if (itemIndex !== undefined) previous.group.indexes.push(itemIndex);
          previous.at = str(frame.at);
          previous.seq = num(frame.seq) ?? previous.seq;
          break;
        }
        rows.push({
          key: `f-${frame.seq}`,
          kind: "node_succeeded",
          seq: num(frame.seq) ?? 0,
          at: str(frame.at),
          tone: "success",
          icon: "node-done",
          title: `Finished ${humanizeLoopNodeId(nodeId)}`,
          micro: `node_succeeded · ${nodeId}`,
          nodeId,
          generation,
          group: {
            nodeId,
            generation,
            count: 1,
            indexes: itemIndex === undefined ? [] : [itemIndex],
          },
        });
        break;
      }
      case "node_failed": {
        const nodeId = str(payload.node_id);
        if (nodeId === "") break;
        const itemIndex = num(payload.item_index);
        trackBranch(num(payload.generation) ?? 0, nodeId, itemIndex);
        running.delete(runningKey(nodeId, itemIndex));
        rows.push(nodeFailedRow(frame, payload, failedOnlyLive));
        break;
      }
      case "status_changed": {
        const to = str(payload.to, str(payload.status));
        if (to !== "running") running.clear();
        const row = statusChangedRow(frame, payload);
        if (row) rows.push(row);
        break;
      }
      default: {
        const graphRow = graphStoryRow(kind, frame);
        if (graphRow) {
          rows.push(graphRow);
          break;
        }
        // Node lifecycle kinds (Spec 1). A beat that ends the current attempt
        // also clears the node from "Happening now", so a paused, canceled, or
        // quarantined node never keeps reading as if it were still working.
        const row = lifecycleStoryRow(kind, frame, payload);
        if (!row) break;
        if (LIFECYCLE_KINDS_ENDING_ATTEMPT.has(kind)) {
          const nodeId = str(payload.node_id);
          if (nodeId !== "") running.delete(runningKey(nodeId, num(payload.item_index)));
        }
        rows.push(row);
        break;
      }
    }
  }

  // The persisted generation marker owns score/best once its detail projection
  // is available. Keep the SSE verdict metadata only as a live fallback while
  // the detail query catches up; otherwise adjacent rows would repeat the same
  // score and preserve a historical "best" after a later generation wins.
  const generationRows = new Map<number, MutableStoryRow>();
  for (const row of rows) {
    if (row.kind === "generation_started" && row.generation !== undefined) {
      generationRows.set(row.generation, row);
    }
  }
  for (const row of rows) {
    if (row.kind !== "gate_verdict" || row.generation === undefined) continue;
    const generationRow = generationRows.get(row.generation);
    if (generationRow?.score !== undefined) row.score = undefined;
    // Without a polled projection, each verdict retains its historical "best at
    // this point" meaning. Once detail supplies a best, reconcile every row to
    // the freshest stream-or-detail projection selected above.
    if (
      context.bestGeneration !== null &&
      context.bestGeneration !== undefined &&
      bestGeneration !== null &&
      bestGeneration !== undefined
    ) {
      row.isBest = row.generation === bestGeneration;
    }
    if (generationRow?.isBest) row.isBest = false;
  }

  // The latest round marker is the live one — accent while the run works,
  // neutral once superseded or parked (§5.3, prototype states file).
  if (context.status === "running") {
    const latestRound = [...rows].reverse().find(row => row.kind === "generation_started");
    if (latestRound) latestRound.tone = "accent";
  }

  const finalized = rows.map(row =>
    finalizeSuccessRow(
      row,
      row.group ? (branches.get(branchKey(row.group.generation, row.group.nodeId))?.size ?? 0) : 0
    )
  );

  // Copy-then-reverse keeps the finalize→return path non-mutating (matches the
  // file's `[...rows].reverse()` idiom); `Array.toReversed` is ES2023, this lib is ES2022.
  return {
    rows: [...finalized].reverse(),
    now: live ? projectNow(running, context, goalIds) : null,
  };
}

function projectNow(
  running: Map<string, RunningEntry>,
  context: LoopRunStoryContext,
  goalIds: Set<string>
): LoopStoryNow | null {
  let newest: RunningEntry | null = null;
  for (const entry of running.values()) {
    if (!newest || entry.seq > newest.seq) newest = entry;
  }
  if (!newest) return null;
  const current = newest;
  const childRunId = context.generations
    ?.flatMap(generation => generation.outputs)
    .find(
      output =>
        output.node_id === current.nodeId &&
        (output.item_index ?? undefined) === current.itemIndex &&
        output.status === "awaiting_child"
    )?.child_loop_run_id;
  return {
    nodeId: current.nodeId,
    label: humanizeLoopNodeId(current.nodeId),
    itemIndex: current.itemIndex,
    generation: current.generation,
    startedAt: current.startedAt,
    taskLink:
      current.taskId && current.taskRunId
        ? { taskId: current.taskId, taskRunId: current.taskRunId }
        : undefined,
    childRunId: childRunId || undefined,
    isGoalNode: goalIds.has(current.nodeId),
  };
}

export { humanizeLoopNodeId };
export { buildNextNote } from "./loop-run-next-note";
export type {
  LoopRunStory,
  LoopRunStoryContext,
  LoopStoryIcon,
  LoopStoryIssue,
  LoopStoryNow,
  LoopStoryRow,
  LoopStoryTaskLink,
} from "./loop-run-story-types";
