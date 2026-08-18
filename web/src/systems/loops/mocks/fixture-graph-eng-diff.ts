import {
  GRAPH_ENG_FORK_RUN_ID,
  GRAPH_ENG_RUN_ID,
  GRAPH_ENG_TERMINAL_RUN_ID,
} from "./fixture-graph-eng-requests";
import type { LoopDiff } from "../types";

export const generationDiffFixture: LoopDiff = {
  kind: "generation",
  base: { run_id: GRAPH_ENG_RUN_ID, generation: 2, status: "running" },
  against: { run_id: GRAPH_ENG_RUN_ID, generation: 3, status: "running" },
  inputs: [],
  nodes: [
    {
      node_id: "triage",
      change: "changed",
      base: { inline: "standard" },
      against: { inline: "hotfix" },
      cause: "condition_matched",
    },
    {
      node_id: "apply-migration",
      change: "rerun",
      item_index: 1,
      base: { inline: "failed" },
      against: { inline: "succeeded" },
    },
    {
      node_id: "backlog",
      change: "skipped",
      cause: "route_not_taken",
    },
    {
      node_id: "services",
      change: "carried",
      base: { hash: "sha256:9f21ab", size: 48_112 },
      against: { hash: "sha256:9f21ab", size: 48_112 },
    },
    {
      node_id: "render-notes",
      change: "verdict",
      base: { inline: "rejected" },
      against: { inline: "approved" },
    },
  ],
  terminal: null,
};

export const runDiffFixture: LoopDiff = {
  kind: "run",
  base: { run_id: GRAPH_ENG_RUN_ID, generation: 3, status: "running" },
  against: { run_id: GRAPH_ENG_FORK_RUN_ID, generation: 2, status: "running", as_of: true },
  definition_divergence: true,
  inputs: [
    { key: "severity", base: { inline: "p1" }, against: { inline: "p0" } },
    {
      key: "services",
      base: { inline: ["api", "web", "worker"] },
      against: { inline: ["api", "web", "worker"] },
    },
  ],
  nodes: [
    {
      node_id: "triage",
      change: "changed",
      base: { inline: "standard" },
      against: { inline: "hotfix" },
      cause: "condition_matched",
    },
    { node_id: "standard", change: "skipped", cause: "route_not_taken" },
  ],
  terminal: { base: "done", against: "running" },
};

export const emptyDiffFixture: LoopDiff = {
  kind: "generation",
  base: { run_id: GRAPH_ENG_TERMINAL_RUN_ID, generation: 1, status: "done" },
  against: { run_id: GRAPH_ENG_TERMINAL_RUN_ID, generation: 2, status: "done" },
  inputs: [],
  nodes: [],
  terminal: { base: "done", against: "done" },
};

export function resolveDiffFixture(runId: string, query: URLSearchParams): LoopDiff | null {
  const againstRun = query.get("against_run");
  if (againstRun) return runDiffFixture;
  const againstGeneration = query.get("against_generation");
  if (runId === GRAPH_ENG_TERMINAL_RUN_ID && againstGeneration === "2") return emptyDiffFixture;
  if (againstGeneration) return generationDiffFixture;
  return null;
}
