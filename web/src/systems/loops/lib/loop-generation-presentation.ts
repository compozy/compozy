import type { LoopRosterNode, LoopRun, LoopRunGeneration } from "../types";
import { isTerminalLoopStatus } from "./loop-formatters";
import { isFailedRosterState, isUnsettledRosterState } from "./loop-run-state-copy";
import { deriveCostEstimate } from "./loop-run-usage";

const ORIGIN_LABELS: Record<LoopRunGeneration["origin"], string> = {
  initial: "Initial generation",
  stop_when: "Stop condition",
  reattempt: "Re-attempt",
  gate_revise: "Gate revision",
  gate_next_generation: "Gate continuation",
  dod_retry: "Definition-of-done retry",
  ratchet_restore: "Ratchet restore",
  requeue: "Manual requeue",
  operator_rerun: "Operator rerun",
  fork_seed: "Fork seed",
};

export function isLoopGenerationOrigin(value: string): value is LoopRunGeneration["origin"] {
  return Object.hasOwn(ORIGIN_LABELS, value);
}

/** Formats persisted metric values consistently across run summaries and detail. */
export function formatLoopScore(score: number | null | undefined): string | null {
  return typeof score === "number" && Number.isFinite(score) ? score.toFixed(2) : null;
}

/** The daemon-owned best-generation projection; the client never recomputes it. */
export function loopRunBestLabel(
  run: Pick<LoopRun, "best_generation" | "best_score">
): string | null {
  if (run.best_generation === null || run.best_generation === undefined) return null;
  const score = formatLoopScore(run.best_score);
  return score ? `Gen ${run.best_generation} · ${score}` : `Gen ${run.best_generation}`;
}

/** Human-readable provenance for one persisted generation. */
export function loopGenerationOriginLabel(
  origin: LoopRunGeneration["origin"],
  parentGeneration: number
): string {
  if (origin === "ratchet_restore") return `Restored from gen ${parentGeneration}`;
  return ORIGIN_LABELS[origin];
}

/** A Loop allows one metric criterion, so at most one persisted verdict carries a score. */
export function loopGenerationScore(generation: LoopRunGeneration | undefined): number | undefined {
  return (
    generation?.verdicts.find(verdict => verdict.score !== null && verdict.score !== undefined)
      ?.score ?? undefined
  );
}

type LoopVerdictOutcome = LoopRunGeneration["verdicts"][number]["outcome"];

/**
 * The gate's verdict in words. The enum is closed, so this map is exhaustive by
 * type — a new outcome fails the build here rather than leaking `invalid_output`
 * into the page as a sentence nobody wrote (UT-044).
 */
const VERDICT_OUTCOME_LABELS = {
  approved: "accepted",
  rejected: "rejected",
  awaiting_approval: "waiting for a decision",
  blocked: "blocked",
  error: "the check errored",
  timeout: "the check timed out",
  invalid_output: "the output did not match its schema",
} as const satisfies Record<LoopVerdictOutcome, string>;

const VERDICT_OUTCOME_TONES = {
  approved: "success",
  rejected: "danger",
  awaiting_approval: "warning",
  blocked: "warning",
  error: "danger",
  timeout: "danger",
  invalid_output: "danger",
} as const satisfies Record<LoopVerdictOutcome, LoopGenerationTone>;

export type LoopGenerationTone = "success" | "danger" | "warning" | "neutral";

/**
 * What happened to the round, beyond its verdict.
 *
 * `interrupted` is the one that has to be earned: a terminal run holding an
 * unsettled step means the round never finished, and dressing that as a settled
 * outcome is the most misleading thing this view could do (US-013.EC-1).
 */
export type LoopGenerationProgressState = "settled" | "in-progress" | "partial" | "interrupted";

/** Rounds that never reached an end, so no timestamp of theirs is an ending. */
const ROUND_NEVER_ENDED = new Set<LoopGenerationProgressState>(["in-progress", "interrupted"]);

export interface LoopGenerationRow {
  generation: number;
  outcomeLabel: string;
  tone: LoopGenerationTone;
  /** Only present when the loop defines scoring; no column of dashes otherwise. */
  scoreLabel: string | null;
  originLabel: string;
  stepResults: number;
  progressState: LoopGenerationProgressState;
  /** Summed from this round's roster rows — the only per-step usage there is. */
  tokens: number | null;
  /** `~$0.05`; an estimate, and the leading `~` says so. */
  costLabel: string | null;
  /** Latest settled step in the round; absent while anything is still running. */
  endedAt: string | null;
}

function outcomeOf(generation: LoopRunGeneration): { label: string; tone: LoopGenerationTone } {
  const verdicts = generation.verdicts;
  if (verdicts.length === 0) return { label: "no verdict recorded", tone: "neutral" };
  const approved = verdicts.filter(verdict => verdict.outcome === "approved").length;
  if (approved === verdicts.length) return { label: "accepted", tone: "success" };
  if (approved === 0) {
    const first = verdicts[0].outcome;
    return { label: VERDICT_OUTCOME_LABELS[first], tone: VERDICT_OUTCOME_TONES[first] };
  }
  return { label: `partly accepted — ${approved} of ${verdicts.length}`, tone: "warning" };
}

/**
 * How far a round got, from the fate of its rows.
 *
 * A round with no rows at all is the case worth being careful about. On a live
 * run that is almost never "this round finished with nothing in it" — it is a
 * round whose rows have not been read yet, and calling it settled hands the
 * operator a conclusion the page has no evidence for. Only once the run itself
 * has stopped can an empty round be reported as over.
 */
function progressOf(
  nodes: readonly LoopRosterNode[],
  runStatus: string | undefined
): LoopGenerationProgressState {
  if (nodes.length === 0) return isTerminalLoopStatus(runStatus) ? "settled" : "in-progress";
  const unsettled = nodes.some(node => isUnsettledRosterState(node.state));
  if (unsettled) return isTerminalLoopStatus(runStatus) ? "interrupted" : "in-progress";
  const failed = nodes.some(node => isFailedRosterState(node.state));
  const succeeded = nodes.some(node => node.state === "succeeded");
  return failed && succeeded ? "partial" : "settled";
}

function latestEnd(nodes: readonly LoopRosterNode[]): string | null {
  let latest: string | null = null;
  let highest = Number.NEGATIVE_INFINITY;
  for (const node of nodes) {
    if (!node.ended_at) continue;
    const parsed = Date.parse(node.ended_at);
    if (Number.isNaN(parsed) || parsed <= highest) continue;
    highest = parsed;
    latest = node.ended_at;
  }
  return latest;
}

export interface BuildGenerationHistoryInput {
  generations: readonly LoopRunGeneration[];
  /** This run's roster rows; the per-round usage and settlement come from here. */
  nodes: readonly LoopRosterNode[];
  runStatus?: string;
}

/**
 * How the run converged, round by round.
 *
 * Usage is summed from the roster rather than restated from the run total,
 * because "this round cost that much" is a different fact from "the run has
 * spent this much", and only the roster records the first one.
 */
export function buildGenerationHistory({
  generations,
  nodes,
  runStatus,
}: BuildGenerationHistoryInput): LoopGenerationRow[] {
  // Indexed once: a run with many rounds and a large roster would otherwise
  // rescan every loaded row for every round it draws.
  const byRound = new Map<number, LoopRosterNode[]>();
  for (const node of nodes) {
    const bucket = byRound.get(node.generation);
    if (bucket) bucket.push(node);
    else byRound.set(node.generation, [node]);
  }
  return [...generations]
    .sort((left, right) => right.generation - left.generation)
    .map(generation => {
      const roundNodes = byRound.get(generation.generation) ?? [];
      const outcome = outcomeOf(generation);
      const progressState = progressOf(roundNodes, runStatus);
      const tokens = roundNodes.reduce((total, node) => total + (node.usage?.tokens ?? 0), 0);
      const hasUsage = tokens > 0;
      return {
        generation: generation.generation,
        outcomeLabel: outcome.label,
        tone: outcome.tone,
        scoreLabel: formatLoopScore(loopGenerationScore(generation)),
        originLabel: loopGenerationOriginLabel(generation.origin, generation.parent_generation),
        stepResults: generation.outputs.length,
        progressState,
        tokens: hasUsage ? tokens : null,
        costLabel: hasUsage ? deriveCostEstimate(tokens) : null,
        // A round that never finished has no end time. An interrupted round is
        // the trap: its settled steps do have timestamps, and printing the last
        // of them reads as "the round ended here" when what actually happened is
        // that the run died holding it open.
        endedAt: ROUND_NEVER_ENDED.has(progressState) ? null : latestEnd(roundNodes),
      };
    });
}
