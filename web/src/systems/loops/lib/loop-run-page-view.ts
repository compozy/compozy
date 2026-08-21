import type {
  LoopDefinition,
  LoopNodeControl,
  LoopNodeWait,
  LoopRequest,
  LoopRunEventFrame,
  LoopRunGeneration,
  LoopRunRecord,
} from "../types";
import { type LoopNodeLifecycle, projectNodeLifecycles, waitingNodes } from "./loop-node-lifecycle";
import type { LoopApprovalFact, LoopApprovalRequest, LoopRunLiveState } from "./loop-events";
import { isTerminalLoopStatus } from "./loop-formatters";
import { type LoopGraph, readLoopGraph } from "./loop-graph";
import { type LoopRunInputRow, buildInputRows, humanizeStartOrigin } from "./loop-run-about";
import { projectLoopRequest, type LoopRequestView } from "./loop-request-model";
import {
  type LoopRunUsageRow,
  buildRunUsage,
  formatClockDuration,
  runElapsedSeconds,
  usageNote,
  usageSnapshotFacts,
} from "./loop-run-usage";

export interface LoopRunPageViewInput {
  run: LoopRunRecord;
  generations: readonly LoopRunGeneration[] | undefined;
  live: LoopRunLiveState;
  definition: LoopDefinition | undefined;
  nowMs: number;
  /** Durable per-node control truth from the run detail. */
  nodeControls?: readonly LoopNodeControl[];
  /** Durable wait cells from the run detail. */
  waits?: readonly LoopNodeWait[];

  requests?: readonly LoopRequest[];

  frames?: readonly LoopRunEventFrame[];
}

/**
 * What the run page still derives from the run detail and the stream.
 *
 * Everything the two-register redesign moved onto task_03's reads left with the
 * cockpit that read it: the client-side story, the now card, the next note, the
 * strategy panel and the node→session map are all projections of a page that no
 * longer exists.
 */
export interface LoopRunPageView {
  /** Run with max(polled, streamed) tokens_used so Usage never steps backward. */
  effectiveRun: LoopRunRecord;
  graph: LoopGraph | null;
  isLive: boolean;
  elapsedLabel: string;
  usageRows: LoopRunUsageRow[];
  usageNote: string | null;
  approvalRequest: LoopApprovalRequest | null;
  approvalFallbackFacts: LoopApprovalFact[];
  inputRows: LoopRunInputRow[];
  /** Unprefixed origin/actor from `humanizeStartOrigin` for the About rail. */
  startedBy: string;
  /** One row per node the daemon reports control, wait, or retry state for. */
  nodeLifecycles: LoopNodeLifecycle[];
  /** Nodes holding an open wait cell — the window chrome's waiting count. */
  waitingNodes: LoopNodeLifecycle[];

  requests: LoopRequestView[];
}

export function projectLoopRunPageView(input: LoopRunPageViewInput): LoopRunPageView {
  const { run, generations, live, definition, nowMs } = input;
  // Node lifecycle truth joins the three durable sources the run detail returns.
  // It gates the parked accounting in Progress, the Waiting/Needs-attention
  // panels, and every control the page is allowed to offer.
  const nodeLifecycles = projectNodeLifecycles({
    controls: input.nodeControls,
    waits: input.waits,
    generations,
    runGeneration: run.generation,
  });
  // A lifecycle refetch can return tokens newer than the latest streamed tick;
  // take the max so Usage never steps backward between a tick and a poll.
  const effectiveRun =
    live.tokensUsed !== null
      ? { ...run, tokens_used: Math.max(run.tokens_used, live.tokensUsed) }
      : run;

  const graph = definition ? readLoopGraph(definition) : null;
  const elapsedSeconds = runElapsedSeconds(run, nowMs);

  return {
    effectiveRun,
    graph,
    isLive: !isTerminalLoopStatus(run.status),
    elapsedLabel: formatClockDuration(elapsedSeconds),
    usageRows: buildRunUsage(effectiveRun, elapsedSeconds),
    usageNote: usageNote(run),
    approvalRequest: live.needsApproval,
    approvalFallbackFacts: usageSnapshotFacts(effectiveRun, elapsedSeconds),
    inputRows: buildInputRows(run, definition),
    startedBy: humanizeStartOrigin(run),
    nodeLifecycles,
    waitingNodes: waitingNodes(nodeLifecycles),

    requests: (input.requests ?? []).map(request =>
      projectLoopRequest(request, { nowMs, runStatus: run.status })
    ),
  };
}
