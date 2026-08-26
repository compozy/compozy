import type { LoopDefinitionGraph } from "../types";
import { implementTasksGraph } from "./fixture-implement-tasks";
import { qualityGateGraph } from "./fixture-quality-gate";
import { reviewAndFixGraph } from "./fixture-review-and-fix";

export const graphByName: Record<string, LoopDefinitionGraph> = {
  "implement-tasks": implementTasksGraph,
  "review-and-fix": reviewAndFixGraph,
  "quality-gate-demo": qualityGateGraph,
};
