import type { LoopRun, LoopRunGeneration } from "../types";

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
