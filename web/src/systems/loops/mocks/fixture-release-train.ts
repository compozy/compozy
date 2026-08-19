import { fixtureGraph } from "./fixture-graph";
import { heroEffectiveLifecycle } from "./fixture-hero-path";
import type { LoopContract, LoopDetail } from "../types";

export const RELEASE_TRAIN_LOOP_NAME = "release-train";

const releaseTrainContract: LoopContract = {
  goal: "Ship a release across every service with a human in the loop.",
  definition_of_done: "Every service is rolled out or explicitly accepted as partial.",
  iteration_cap: 20,
  budget: { tokens: 900_000, wall_clock_sec: 7_200, on_exceeded: "escalate" },
  no_progress: { window: 3 },
  boundaries: ["Never roll out without an approved migration."],
  constraints: ["Canary before full rollout."],
  terminal_states: ["done", "no-op", "blocked", "failed", "exhausted", "stalled"],
  verification: [{ id: "smoke", type: "command", check: "make smoke", expect: "exit 0" }],
};

export const releaseTrainGraph = fixtureGraph(
  [
    { id: "services", class: "source", kind: "input" },
    {
      id: "confirm-rollout",
      class: "control",
      kind: "ask",
      params: {
        prompt: "Which regions ship first?",
        context: { train: "release-train", services: ["api", "web", "worker"] },
        expect: {
          type: "object",
          required: ["regions", "canary"],
          properties: {
            regions: { type: "array", items: { type: "string" } },
            canary: { type: "boolean" },
          },
        },
        responders: { agents: "allow" },
        expires: { after: "24h" },
      },
    },
    {
      id: "triage",
      class: "control",
      kind: "route",
      routes: [
        { when: 'inputs.severity == "p0"', to: "hotfix" },
        { when: 'inputs.severity == "p1"', to: "standard" },
      ],
      default: "backlog",
      on_eval_error: "fail",
    },
    { id: "hotfix", class: "action", kind: "transform" },
    { id: "standard", class: "action", kind: "transform" },
    { id: "backlog", class: "action", kind: "transform" },
    {
      id: "rollout",
      class: "control",
      kind: "fan-out",
      collection: "{{ .inputs.services }}",
      batch_size: 1,
      max_parallel: 2,
      max_fan_out: 500,
      bind_as: "service",
      index_as: "service_index",
      strategy: { kind: "best_effort", threshold: "66%", missing: "acceptable" },
    },
    {
      id: "apply-migration",
      class: "action",
      kind: "run-agent",
      timeout: "30m0s",
      review: {
        decisions: ["approve", "edit", "reject", "respond"],
        prompt: "apply-migration proposes a migrate",
        responders: { agents: "deny" },
        on_reject: { route: "backlog" },
        expires: { after: "24h" },
      },
    },
    { id: "collect-rollout", class: "control", kind: "collect" },
    { id: "render-notes", class: "action", kind: "transform" },
  ],
  [
    { from: "services", to: "confirm-rollout" },
    { from: "confirm-rollout", to: "triage" },
    { from: "triage", to: "hotfix" },
    { from: "triage", to: "standard" },
    { from: "triage", to: "backlog" },
    { from: "standard", to: "rollout" },
    { from: "rollout", to: "apply-migration" },
    { from: "apply-migration", to: "collect-rollout" },
    { from: "collect-rollout", to: "render-notes" },
  ]
);

export const releaseTrainDetail: LoopDetail = {
  name: RELEASE_TRAIN_LOOP_NAME,
  source: "workspace",
  version: 3,
  description: "Ship a release across every service with a human in the loop.",
  catalog: {
    category: "Delivery",
    use_when: "You ship a coordinated release and want a human decision before the migration.",
    keywords: ["release", "rollout", "review"],
  },
  definition: {
    apiVersion: "compozy.loop/v1",
    kind: "Loop",
    meta: {
      name: RELEASE_TRAIN_LOOP_NAME,
      description: "Ship a release across every service with a human in the loop.",
      version: 3,
      catalog: {
        category: "Delivery",
        use_when: "You ship a coordinated release and want a human decision before the migration.",
        keywords: ["release", "rollout", "review"],
      },
    },
    contract: releaseTrainContract,
    graph: releaseTrainGraph,
    inputs: {
      services: { type: "string", required: true, description: "Services in this train." },
      severity: { type: "string", required: false, default: "p1" },
    },
    start: [{ kind: "manual" }, { kind: "cli" }, { kind: "http" }],
  },
  effective_lifecycle: heroEffectiveLifecycle,
};
