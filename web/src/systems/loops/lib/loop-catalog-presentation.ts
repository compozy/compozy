import type { LoopCatalogEntry } from "../types";
import { hasHumanGate, iterationCapLabel, loopInputCount } from "./loop-catalog";

/**
 * The shape-of-the-loop facts both catalog views state, in one order.
 *
 * Rows and cards render the same declared facts so a loop reads the same either
 * way; only the separator differs (meta dots vs a joined line).
 */
export function loopFactsSegments(entry: LoopCatalogEntry): string[] {
  const segments = [
    `${loopInputCount(entry)} inputs`,
    `iteration cap ${iterationCapLabel(entry.contract.iteration_cap)}`,
  ];
  if (hasHumanGate(entry)) segments.push("human gate");
  return segments;
}
