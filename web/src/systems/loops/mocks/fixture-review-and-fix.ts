import {
  SPEC_CYCLE_FINALIZE_REVIEW_ROUND_KIND,
  SPEC_CYCLE_WRITE_REVIEW_ARTIFACTS_KIND,
} from "./fixture-action-kinds";
import { fixtureGraph } from "./fixture-graph";

export const reviewAndFixGraph = fixtureGraph(
  [
    { id: "review", class: "action", kind: "run-agent" },
    {
      id: "has_issues",
      class: "control",
      kind: "branch",
      condition: "size(nodes.review.output.issues) > 0",
    },
    {
      id: "write_artifacts",
      class: "action",
      kind: SPEC_CYCLE_WRITE_REVIEW_ARTIFACTS_KIND,
    },
    {
      id: "fix_batches",
      class: "control",
      kind: "fan-out",
      batch_size: 1,
      max_parallel: 1,
      max_fan_out: 64,
    },
    { id: "fix_batch", class: "action", kind: "run-agent" },
    { id: "collect_fixes", class: "control", kind: "collect" },
    {
      id: "finalize_round",
      class: "action",
      kind: SPEC_CYCLE_FINALIZE_REVIEW_ROUND_KIND,
    },
  ],
  [
    { from: "review", to: "has_issues" },
    { from: "has_issues", to: "write_artifacts" },
    { from: "write_artifacts", to: "fix_batches" },
    { from: "fix_batches", to: "fix_batch" },
    { from: "fix_batch", to: "collect_fixes" },
    { from: "collect_fixes", to: "finalize_round" },
  ]
);
