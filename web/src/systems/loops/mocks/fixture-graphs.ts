import type { LoopDefinitionGraph } from "../types";
import { implementTasksGraph } from "./fixture-implement-tasks";
import { orchestrateTasksGraph } from "./fixture-orchestrate-tasks";
import { qualityGateGraph } from "./fixture-quality-gate";
import { reviewAndFixGraph } from "./fixture-review-and-fix";

export const graphByName: Record<string, LoopDefinitionGraph> = {
  "implement-tasks": implementTasksGraph,
  "orchestrate-tasks": orchestrateTasksGraph,
  "review-and-fix": reviewAndFixGraph,
  "quality-gate-demo": qualityGateGraph,
};
