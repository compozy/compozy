import type { LoopCatalogEntry } from "../types";
import { hasHumanGate, iterationCapLabel, loopCategory, loopInputCount } from "./loop-catalog";
import { loopRunBestLabel } from "./loop-generation-presentation";

export interface LoopLastRunFact {
  id: string;
  iso: string;
}

/**
 * The shape-of-the-loop facts both catalog views state, in one order.
 *
 * Rows and cards render the same declared facts so a loop reads the same either
 * way; only the separator differs (meta dots vs a joined line). Category leads;
 * best is a plain metric, never a state tone.
 */
export function loopFactsSegments(entry: LoopCatalogEntry): string[] {
  const segments: string[] = [];
  const category = loopCategory(entry);
  if (category) segments.push(category);
  segments.push(
    `${loopInputCount(entry)} inputs`,
    `iteration cap ${iterationCapLabel(entry.contract.iteration_cap)}`
  );
  if (hasHumanGate(entry)) segments.push("human gate");
  const best = entry.last_run ? loopRunBestLabel(entry.last_run) : null;
  if (best) segments.push(`best ${best}`);
  return segments;
}

/** Last-run identity for the shared facts line. Absent when the loop has never run. */
export function loopLastRunFact(entry: LoopCatalogEntry): LoopLastRunFact | null {
  const run = entry.last_run;
  if (!run) return null;
  return { id: run.id, iso: run.created_at };
}
