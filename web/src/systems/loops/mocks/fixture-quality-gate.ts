import { fixtureGraph } from "./fixture-graph";

export const qualityGateGraph = fixtureGraph(
  [
    { id: "slug", class: "source", kind: "input" },
    { id: "load_tasks", class: "source", kind: "file-import" },
    {
      id: "implement",
      class: "control",
      kind: "fan-out",
      collection: "{{ .nodes.load_tasks.output.tasks }}",
      batch_size: 1,
      max_parallel: 1,
      max_fan_out: 64,
    },
    {
      id: "execute_task",
      class: "action",
      kind: "run-agent",
      timeout: "45m0s",
      deadline: "2h0m0s",
    },
    { id: "collect", class: "control", kind: "collect" },
    { id: "review", class: "control", kind: "gate", verdict_policy: "revise_until_clean" },
    { id: "verify", class: "control", kind: "gate", verdict_policy: "fixed_passes" },
    { id: "approve", class: "control", kind: "gate", verdict_policy: "fixed_passes" },
  ],
  [
    { from: "slug", to: "load_tasks" },
    { from: "load_tasks", to: "implement" },
    { from: "implement", to: "execute_task" },
    { from: "execute_task", to: "collect" },
    { from: "collect", to: "review" },
    { from: "review", to: "verify" },
    { from: "verify", to: "approve" },
  ]
);
