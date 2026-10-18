import type { LoopCatalogEntry } from "../types";
import { hasHumanGate, loopCategory } from "./loop-catalog";

export interface LoopLastRunFact {
  id: string;
  iso: string;
}

/**
 * The shape-of-the-loop facts both catalog views state, in one order.
 *
 * Rows and cards render the same declared facts so a loop reads the same either
 * way; only the separator differs (meta dots vs a joined line). The catalog keeps
 * this short on purpose: inputs, the round cap, and the best score live on the
 * detail page.
 */
export function loopFactsSegments(entry: LoopCatalogEntry): string[] {
  const segments: string[] = [];
  const category = loopCategory(entry);
  if (category) segments.push(category);
  if (hasHumanGate(entry)) segments.push("Asks you before finishing");
  return segments;
}

/** Last-run identity for the shared facts line. Absent when the loop has never run. */
export function loopLastRunFact(entry: LoopCatalogEntry): LoopLastRunFact | null {
  const run = entry.last_run;
  if (!run) return null;
  return { id: run.id, iso: run.created_at };
}
