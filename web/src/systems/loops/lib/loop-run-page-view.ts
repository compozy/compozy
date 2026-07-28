import type { LoopDefinition, LoopRunEventFrame, LoopRunGeneration, LoopRunRecord } from "../types";
import type {
  LoopApprovalFact,
  LoopApprovalRequest,
  LoopCoordinatorFailure,
  LoopGateVerdict,
  LoopRunLiveState,
} from "./loop-events";
import { isTerminalLoopStatus } from "./loop-formatters";
import { type LoopGraph, findWatchNode, goalNodeIds, readLoopGraph } from "./loop-graph";
import {
  type LoopRunInputRow,
  buildInputRows,
  humanizeStartOrigin,
  watchedSubject,
} from "./loop-run-about";
import { buildNextNote } from "./loop-run-next-note";
import {
  type LoopRunProgressModel,
  buildRunProgress,
  latestGateVerdict,
} from "./loop-run-progress";
import { buildRunStory } from "./loop-run-story";
import type { LoopRunStory } from "./loop-run-story-types";
import {
  type LoopRunUsageRow,
  buildRunUsage,
  formatClockDuration,
  runElapsedSeconds,
  usageNote,
  usageSnapshotFacts,
} from "./loop-run-usage";

/**
 * The single run-page derivation path (redesign spec §2-§7). Both the live hook
 * (`useLoopRunPage`) and the Storybook fixtures read this projection, so the two
 * can never drift on story/progress/usage/about or the now-card gating. It is a
 * pure function of the run projection, the pinned definition, the reduced SSE
 * live state, and the current clock — no React, no query access.
 */

/** Statuses whose main column leads with the "Happening now" card (§7). */
export const NOW_CARD_STATUSES = new Set(["running", "watching", "paused"]);

/** The newest `status_changed` frame that landed the run on `toStatus`. */
function findTerminalTransitionFrame(
  frames: readonly LoopRunEventFrame[],
  toStatus: string
): LoopRunEventFrame | undefined {
  for (let index = frames.length - 1; index >= 0; index--) {
    const frame = frames[index];
    if (frame.kind !== "status_changed") continue;
    const payload =
      typeof frame.payload === "object" && frame.payload !== null
        ? (frame.payload as Record<string, unknown>)
        : null;
    if (!payload) continue;
    const to = typeof payload.to === "string" ? payload.to : payload.status;
    if (to === toStatus) return frame;
  }
  return undefined;
}

/** The `from` side of the newest transition that landed the run on `toStatus`. */
export function findTransitionFrom(
  frames: readonly LoopRunEventFrame[],
  toStatus: string
): string | undefined {
  const frame = findTerminalTransitionFrame(frames, toStatus);
  const from = frame?.payload as Record<string, unknown> | undefined;
  return typeof from?.from === "string" && from.from !== "" ? from.from : undefined;
}

/**
 * The `at` timestamp of the newest transition that landed the run on `toStatus`
 * — the true terminal instant the status CAS never writes back to the run row.
 */
export function findTransitionAt(
  frames: readonly LoopRunEventFrame[],
  toStatus: string
): string | undefined {
  const at = findTerminalTransitionFrame(frames, toStatus)?.at;
  return typeof at === "string" && at !== "" ? at : undefined;
}

export interface LoopRunPageViewInput {
  run: LoopRunRecord;
  generations: readonly LoopRunGeneration[] | undefined;
  live: LoopRunLiveState;
  definition: LoopDefinition | undefined;
  nowMs: number;
}

export interface LoopRunPageView {
  /** Run with max(polled, streamed) tokens_used so Usage never steps backward. */
  effectiveRun: LoopRunRecord;
  graph: LoopGraph | null;
  story: LoopRunStory;
  isLive: boolean;
  subject: string | null;
  hasWatchSource: boolean;
  elapsedLabel: string;
  stepElapsedLabel: string | null;
  progress: LoopRunProgressModel;
  goalIds: ReadonlySet<string>;
  usageRows: LoopRunUsageRow[];
  usageNote: string | null;
  approvalRequest: LoopApprovalRequest | null;
  approvalFallbackFacts: LoopApprovalFact[];
  failure: LoopCoordinatorFailure | null;
  latestVerdict: LoopGateVerdict | null;
  watchCadence: string | null;
  inputRows: LoopRunInputRow[];
  startedBy: string;
  nextNote: string | null;
  showNowCard: boolean;
  terminalFromStatus?: string;
  /** The terminal transition's `at`; the outcome card derives true elapsed from it. */
  terminalAt?: string;
}

export function projectLoopRunPageView(input: LoopRunPageViewInput): LoopRunPageView {
  const { run, generations, live, definition, nowMs } = input;
  // A lifecycle refetch can return tokens newer than the latest streamed tick;
  // take the max so Usage never steps backward between a tick and a poll.
  const effectiveRun =
    live.tokensUsed !== null
      ? { ...run, tokens_used: Math.max(run.tokens_used, live.tokensUsed) }
      : run;

  const graph = definition ? readLoopGraph(definition) : null;
  const story = buildRunStory(live.frames, {
    status: run.status,
    reattemptStrategy: run.reattempt_strategy,
    graph,
    generations,
  });

  const elapsedSeconds = runElapsedSeconds(run, nowMs);
  const stepStartedMs =
    run.status === "running" && story.now ? Date.parse(story.now.startedAt) : Number.NaN;
  const stepElapsedLabel = Number.isNaN(stepStartedMs)
    ? null
    : formatClockDuration((nowMs - stepStartedMs) / 1000);

  const latestVerdict = latestGateVerdict(live.gateVerdicts);
  const openPoints = latestVerdict ? latestVerdict.blockingIssues.length : null;

  const watchNode = graph ? findWatchNode(graph) : null;
  const watchPoll = typeof watchNode?.watch?.poll === "string" ? watchNode.watch.poll : "";
  const showNextNote =
    run.status === "running" || run.status === "watching" || run.status === "queued";

  return {
    effectiveRun,
    graph,
    story,
    isLive: !isTerminalLoopStatus(run.status),
    subject: watchedSubject(run, definition),
    hasWatchSource: watchNode !== null,
    elapsedLabel: formatClockDuration(elapsedSeconds),
    stepElapsedLabel,
    progress: buildRunProgress(effectiveRun, generations, openPoints),
    goalIds: graph ? goalNodeIds(graph) : new Set<string>(),
    usageRows: buildRunUsage(effectiveRun, elapsedSeconds),
    usageNote: usageNote(run),
    approvalRequest: live.needsApproval,
    approvalFallbackFacts: usageSnapshotFacts(effectiveRun, elapsedSeconds),
    failure: live.failure,
    latestVerdict,
    watchCadence: watchPoll ? `checks every ${watchPoll}` : null,
    inputRows: buildInputRows(run, definition),
    startedBy: humanizeStartOrigin(run),
    nextNote: showNextNote ? buildNextNote(graph, generations) : null,
    showNowCard:
      NOW_CARD_STATUSES.has(run.status) ||
      (run.status === "queued" && definition?.concurrency === "queue"),
    terminalFromStatus: findTransitionFrom(live.frames, run.status),
    terminalAt: findTransitionAt(live.frames, run.status),
  };
}
