import { use } from "react";

import { emptySteerProvenance, type SteerProvenanceIndex } from "../lib/steer-provenance";
import { SteerProvenanceContext } from "../lib/steer-provenance-context-value";

/** The thread's steer provenance; empty outside a thread so markers stay legacy rows. */
export function useSteerProvenance(): SteerProvenanceIndex {
  return use(SteerProvenanceContext) ?? emptySteerProvenance();
}
