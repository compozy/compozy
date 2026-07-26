import type {
  LoopAnnotation,
  LoopCatalogEntry,
  LoopConfig,
  LoopContract,
  LoopDefinition,
  LoopDefinitionGraph,
  LoopDetail,
  LoopEffectiveConfig,
  LoopRun,
  LoopRunAggregates,
  LoopRunDetail,
} from "../types";
import { buildLocalNetworkParticipationFixture } from "@/test/network-participation-fixtures";

export const MOCK_WORKSPACE_ID = "ws_default";

// The daemon serializes a full dsl.Graph, but OpenAPI types nodes/edges as opaque
// records (see lib/loop-graph.ts); the fixtures carry the real node shape and cast.
function graph(
  nodes: ReadonlyArray<Record<string, unknown>>,
  edges: ReadonlyArray<{ from: string; to: string }>
): LoopDefinitionGraph {
  return { nodes, edges } as unknown as LoopDefinitionGraph;
}

const deliveryGraph = graph(
  [
    { id: "slug", class: "source", kind: "input" },
    { id: "load_tasks", class: "source", kind: "file-import" },
    {
      id: "implement",
      class: "control",
      kind: "fan-out",
      batch_size: 1,
      max_parallel: 1,
      max_fan_out: 64,
    },
    { id: "execute_task", class: "action", kind: "run-agent" },
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

const watchGraph = graph(
  [
    { id: "watch_pr", class: "source", kind: "watch-source" },
    { id: "fetch_issues", class: "action", kind: "agh__network_send" },
    { id: "remediate", class: "action", kind: "run-agent" },
    { id: "resolve", class: "control", kind: "gate", verdict_policy: "revise_until_clean" },
  ],
  [
    { from: "watch_pr", to: "fetch_issues" },
    { from: "fetch_issues", to: "remediate" },
    { from: "remediate", to: "resolve" },
  ]
);

const graphByName: Record<string, LoopDefinitionGraph> = {
  "software-delivery": deliveryGraph,
  "reviews-watch": watchGraph,
};

// Vocabulary matches the canonical design spec (LOOPS-DESIGN-SPEC.md): the two
// default dev-cycle Loops are `software-delivery` / `reviews-watch` (§4.1); `reattempt_strategy`
// is `failed_only | full_body` (§5.5); verification criterion `type` is
// `command | agent-judge | human | extension` (§5.3). Screens (tasks 19-22) build
// on these MSW handlers, so the mock stays truthful to what the daemon emits.

const deliveryContract: LoopContract = {
  goal: "Ship the requested change end to end and prove it.",
  definition_of_done: "Configured project checks are green and the change is demonstrated.",
  iteration_cap: 50,
  budget: { tokens: 500_000, wall_clock_sec: 3_600, on_exceeded: "halt" },
  no_progress: { window: 3, hash_fields: ["delivery_artifact", "gate_verdict"] },
  boundaries: ["Do not touch unrelated packages."],
  constraints: ["No destructive git."],
  terminal_states: ["done", "no-op", "blocked", "failed", "exhausted", "stalled"],
  verification: [
    {
      id: "verify_build",
      type: "command",
      check: "npm test",
      expect: "exit 0",
    },
    {
      id: "acceptance_review",
      type: "agent-judge",
      agent: "reviewer",
      rubric: "The change satisfies the goal and definition of done.",
    },
  ],
};

const watchContract: LoopContract = {
  goal: "React to inbound review requests as they arrive.",
  definition_of_done: "Every queued review request has a recorded verdict.",
  iteration_cap: 0,
  budget: { tokens: 0, wall_clock_sec: 0, on_exceeded: "halt" },
  no_progress: { window: 5, hash_fields: ["gate_verdict"] },
};

function buildRun(
  overrides: Partial<LoopRun> & Pick<LoopRun, "id" | "loop_name" | "status">
): LoopRun {
  return {
    workspace_id: MOCK_WORKSPACE_ID,
    generation: 3,
    iteration_cap: 50,
    tokens_used: 128_400,
    pause_requested: false,
    budget_tokens: 500_000,
    budget_wall_sec: 3_600,
    budget_on_exceeded: "halt",
    reattempt_strategy: "failed_only",
    resolved_network_participation: buildLocalNetworkParticipationFixture(),
    created_at: "2026-07-05T12:00:00Z",
    started_at: "2026-07-05T12:00:00Z",
    last_progress_at: "2026-07-05T12:18:00Z",
    definition_version: 4,
    definition_digest: "sha256:mock-loop-definition",
    start_metadata: {},
    ...overrides,
  };
}

export const loopRunFixtures: LoopRun[] = [
  // Live runs (Active table + KPIs).
  buildRun({
    id: "looprun_running",
    loop_name: "software-delivery",
    status: "running",
    generation: 2,
    tokens_used: 412_000,
    budget_tokens: 2_000_000,
    started_origin_kind: "schedule",
    inputs: { slug: "loops-catalog-api" },
  }),
  buildRun({
    id: "looprun_watching",
    loop_name: "reviews-watch",
    status: "watching",
    iteration_cap: 0,
    budget_tokens: 0,
    budget_wall_sec: 0,
    tokens_used: 2_400_000,
    generation: 5,
    started_origin_kind: "webhook",
    inputs: { pr: "482" },
  }),
  buildRun({
    id: "looprun_needs_approval",
    loop_name: "software-delivery",
    status: "needs-approval",
    generation: 3,
    tokens_used: 1_100_000,
    budget_tokens: 2_000_000,
    started_origin_kind: "cli",
    inputs: { slug: "billing-webhooks" },
  }),
  buildRun({
    id: "looprun_paused",
    loop_name: "reviews-watch",
    status: "paused",
    iteration_cap: 0,
    budget_tokens: 0,
    tokens_used: 640_000,
    generation: 2,
    pause_requested: true,
    started_origin_kind: "manual",
    inputs: { pr: "493" },
  }),
  buildRun({
    id: "looprun_queued",
    loop_name: "software-delivery",
    status: "queued",
    generation: 0,
    tokens_used: 0,
    budget_tokens: 2_000_000,
    started_origin_kind: "cli",
    inputs: { slug: "search-reindex" },
  }),
  // Terminal runs (Past table). `done` rows land today for the "Done today" KPI.
  buildRun({
    id: "looprun_done_today",
    loop_name: "software-delivery",
    status: "done",
    generation: 2,
    tokens_used: 520_000,
    budget_tokens: 2_000_000,
    started_origin_kind: "cli",
    last_progress_at: "2026-07-05T15:41:00Z",
    inputs: { slug: "search-reindex" },
  }),
  buildRun({
    id: "looprun_done_watch",
    loop_name: "reviews-watch",
    status: "done",
    iteration_cap: 0,
    budget_tokens: 0,
    tokens_used: 380_000,
    generation: 3,
    started_origin_kind: "webhook",
    last_progress_at: "2026-07-05T11:02:00Z",
    inputs: { pr: "475" },
  }),
  buildRun({
    id: "looprun_no-op",
    loop_name: "reviews-watch",
    status: "no-op",
    iteration_cap: 0,
    budget_tokens: 0,
    tokens_used: 12_000,
    generation: 1,
    started_origin_kind: "webhook",
    inputs: { pr: "468" },
  }),
  buildRun({
    id: "looprun_blocked",
    loop_name: "software-delivery",
    status: "blocked",
    generation: 2,
    tokens_used: 140_000,
    budget_tokens: 2_000_000,
    started_origin_kind: "cli",
    inputs: { slug: "task-03" },
  }),
  buildRun({
    id: "looprun_failed",
    loop_name: "software-delivery",
    status: "failed",
    generation: 2,
    tokens_used: 340_000,
    budget_tokens: 2_000_000,
    started_origin_kind: "schedule",
    inputs: { slug: "approve-gate" },
  }),
  buildRun({
    id: "looprun_exhausted",
    loop_name: "software-delivery",
    status: "exhausted",
    generation: 50,
    iteration_cap: 50,
    tokens_used: 1_900_000,
    budget_tokens: 2_000_000,
    started_origin_kind: "cli",
    inputs: { slug: "hard-problem" },
  }),
  buildRun({
    id: "looprun_exhausted_budget",
    loop_name: "software-delivery",
    status: "exhausted",
    generation: 6,
    tokens_used: 2_000_000,
    budget_tokens: 2_000_000,
    started_origin_kind: "manual",
    inputs: { slug: "token-budget" },
  }),
  buildRun({
    id: "looprun_stalled",
    loop_name: "software-delivery",
    status: "stalled",
    generation: 4,
    tokens_used: 610_000,
    budget_tokens: 2_000_000,
    started_origin_kind: "cli",
    inputs: { slug: "no-progress" },
  }),
  buildRun({
    id: "looprun_stalled_watch",
    loop_name: "reviews-watch",
    status: "stalled",
    iteration_cap: 0,
    budget_tokens: 0,
    tokens_used: 1_200_000,
    generation: 3,
    started_origin_kind: "webhook",
    inputs: { pr: "461" },
  }),
];

export const loopRunAggregatesFixture: LoopRunAggregates = {
  total: loopRunFixtures.length,
  live: 5,
  terminal: 9,
  succeeded: 2,
  failed: 1,
};

export const loopCatalogFixtures: LoopCatalogEntry[] = [
  {
    name: "software-delivery",
    source: "workspace",
    version: 4,
    description: "The canonical ship-a-change delivery loop.",
    catalog: {
      category: "delivery",
      use_when: "You have a concrete change to ship with a verifiable definition of done.",
      keywords: ["ship", "verify", "delivery"],
    },
    contract: deliveryContract,
    inputs: {
      goal: { type: "string", required: true, description: "What to accomplish." },
      max_files: { type: "number", required: false, default: 20 },
    },
    // The 6 declared start kinds from the design (§4.2); a watch-source stays a body node.
    start: [
      { kind: "manual" },
      { kind: "cli" },
      { kind: "http" },
      { kind: "uds" },
      { kind: "native_tool" },
      { kind: "schedule" },
    ],
    last_run: buildRun({
      id: "looprun_running",
      loop_name: "software-delivery",
      status: "running",
    }),
    aggregate_30d: { runs: 42, succeeded: 38, failed: 4 },
    success_rate_30d: 0.9,
  },
  {
    name: "reviews-watch",
    source: "marketplace",
    version: 1,
    description: "A watch loop that drains inbound review requests.",
    catalog: {
      category: "watch",
      use_when: "You want continuous, event-driven handling of review requests.",
      keywords: ["watch", "review"],
    },
    contract: watchContract,
    inputs: {},
    // A watch-source is a body node, never a start binding (design §2/§5.6); the run
    // itself starts via an allowlisted kind (here manual) and then watches.
    start: [{ kind: "manual" }],
    last_run: buildRun({
      id: "looprun_watching",
      loop_name: "reviews-watch",
      status: "watching",
      iteration_cap: 0,
      budget_tokens: 0,
      budget_wall_sec: 0,
      tokens_used: 0,
      generation: 1,
    }),
    aggregate_30d: { runs: 9, succeeded: 9, failed: 0 },
    success_rate_30d: 1,
  },
];

function buildDefinition(entry: LoopCatalogEntry): LoopDefinition {
  return {
    apiVersion: "agh.loop/v1",
    kind: "Loop",
    meta: {
      name: entry.name,
      description: entry.description,
      version: entry.version,
      catalog: entry.catalog,
    },
    contract: entry.contract,
    graph: graphByName[entry.name] ?? graph([], []),
    inputs: entry.inputs,
    start: entry.start,
  };
}

export const loopDetailFixtures: LoopDetail[] = loopCatalogFixtures.map(entry => ({
  name: entry.name,
  source: entry.source,
  version: entry.version,
  description: entry.description,
  catalog: entry.catalog,
  definition: buildDefinition(entry),
}));

export const loopDetailByName = new Map(loopDetailFixtures.map(detail => [detail.name, detail]));

// Generations aligned to `deliveryGraph`'s node ids so the run-page timeline resolves
// each node's real class/kind. G1 fans `execute_task` over 3 tasks (one failed) and the
// `review` gate returns revise; G2 carries the two succeeded tasks forward read-only
// (`reused`), re-runs the failed one, and parks a `run-loop` child (`awaiting_child`).
function deliveryGenerations(run: LoopRun): LoopRunDetail["generations"] {
  const live = run.status === "running" || run.status === "needs-approval";
  const g1 = {
    generation: 1,
    outputs: [
      { node_id: "slug", status: "succeeded", generation: 1 },
      { node_id: "load_tasks", status: "succeeded", generation: 1 },
      {
        node_id: "execute_task",
        status: "succeeded",
        generation: 1,
        item_index: 0,
        task_run_id: "tr_1",
      },
      {
        node_id: "execute_task",
        status: "succeeded",
        generation: 1,
        item_index: 1,
        task_run_id: "tr_2",
      },
      {
        node_id: "execute_task",
        status: "failed",
        generation: 1,
        item_index: 2,
        task_run_id: "tr_3",
      },
      { node_id: "collect", status: "succeeded", generation: 1 },
      { node_id: "review", status: "failed", generation: 1 },
    ],
  };
  const g2 = {
    generation: 2,
    outputs: [
      { node_id: "execute_task", status: "reused", generation: 2, item_index: 0 },
      { node_id: "execute_task", status: "reused", generation: 2, item_index: 1 },
      {
        node_id: "execute_task",
        status: live ? "running" : "succeeded",
        generation: 2,
        item_index: 2,
      },
      {
        node_id: "child_delivery",
        status: "awaiting_child",
        generation: 2,
        child_loop_run_id: "looprun_child",
      },
      { node_id: "collect", status: live ? "pending" : "succeeded", generation: 2 },
      { node_id: "review", status: live ? "pending" : "succeeded", generation: 2 },
      { node_id: "verify", status: live ? "pending" : "succeeded", generation: 2 },
      { node_id: "approve", status: live ? "pending" : "succeeded", generation: 2 },
    ],
  };
  return run.generation >= 2 ? [g1, g2] : [g1];
}

function watchGenerations(run: LoopRun): LoopRunDetail["generations"] {
  return [
    {
      generation: Math.max(1, run.generation),
      outputs: [
        { node_id: "watch_pr", status: "succeeded", generation: run.generation },
        {
          node_id: "fetch_issues",
          status: run.status === "no-op" ? "no-op" : "succeeded",
          generation: run.generation,
        },
        {
          node_id: "remediate",
          status: run.status === "watching" ? "running" : "succeeded",
          generation: run.generation,
        },
        { node_id: "resolve", status: "succeeded", generation: run.generation },
      ],
    },
  ];
}

export const loopRunDetailFixtures: LoopRunDetail[] = loopRunFixtures.map(run => ({
  run: {
    ...run,
    started_by_kind: "user",
    started_by_ref: "operator",
    started_origin_kind: run.started_origin_kind ?? "cli",
  },
  executed_definition: loopDetailByName.get(run.loop_name)!.definition,
  generations: run.loop_name === "reviews-watch" ? watchGenerations(run) : deliveryGenerations(run),
}));

export const loopRunDetailByRunId = new Map(
  loopRunDetailFixtures.map(detail => [detail.run.id, detail])
);

export const loopConfigFixture: LoopConfig = {
  iteration_cap: 16,
  budget_tokens: 750_000,
  budget_wall_sec: null,
  budget_on_exceeded: "escalate",
  fan_out_width: 4,
  gate_max_revisions: 3,
  human_gate_enabled: true,
  no_progress_window: 3,
  reattempt_strategy: "failed_only",
  enabled_checks_json: null,
};

export const loopEffectiveConfigFixture: LoopEffectiveConfig = {
  budget_on_exceeded: "escalate",
  budget_tokens: 750_000,
  budget_wall_sec: 0,
  enabled_checks_json: {},
  fan_out_width: 4,
  gate_max_revisions: 3,
  human_gate_enabled: true,
  iteration_cap: 16,
  model_defaults: { judge: "", worker: "" },
  no_progress_window: 3,
  reattempt_strategy: "failed_only",
};

export const loopAnnotationsFixture: LoopAnnotation[] = [
  { node_id: "plan", x: 120, y: 80 },
  { node_id: "implement", x: 360, y: 80 },
];
