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
} from "../types";
import { buildLocalNetworkParticipationFixture } from "@/test/network-participation-fixtures";
import { buildLoopRunDetailFixtures } from "./fixture-run-details";

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

const reviewGraph = graph(
  [
    { id: "review", class: "action", kind: "run-agent" },
    {
      id: "has_issues",
      class: "control",
      kind: "branch",
      condition: "size(nodes.review.output.issues) > 0",
    },
    { id: "write_artifacts", class: "action", kind: "ext__dev_cycle__write_review_artifacts" },
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
      kind: "ext__dev_cycle__finalize_review_round",
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

const graphByName: Record<string, LoopDefinitionGraph> = {
  "software-delivery": deliveryGraph,
  "review-and-fix": reviewGraph,
};

// Vocabulary matches the canonical design spec (LOOPS-DESIGN-SPEC.md): the two
// default dev-cycle Loops are `software-delivery` / `review-and-fix` (§4.1); `reattempt_strategy`
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

const reviewContract: LoopContract = {
  goal: "Review the work for the named task and remediate every valid finding.",
  definition_of_done:
    "A new agent review round returns no issues after earlier findings were triaged, remediated, verified, and finalized.",
  stop_when: "nodes.review.status == 'succeeded' && size(nodes.review.output.issues) == 0",
  iteration_cap: 3,
  budget: { tokens: 0, wall_clock_sec: 0, on_exceeded: "escalate" },
  no_progress: { window: 2, hash_fields: ["nodes.review.output.issues"] },
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
    best_generation: 2,
    best_score: 0.82,
    tokens_used: 412_000,
    budget_tokens: 2_000_000,
    started_origin_kind: "schedule",
    inputs: { slug: "loops-catalog-api" },
  }),
  buildRun({
    id: "looprun_review_running",
    loop_name: "review-and-fix",
    status: "running",
    iteration_cap: 3,
    budget_tokens: 0,
    budget_wall_sec: 0,
    budget_on_exceeded: "escalate",
    tokens_used: 68_000,
    generation: 2,
    started_origin_kind: "cli",
    inputs: {
      task_name: "billing-webhooks",
      reviewer: "reviewer",
      fixer: "review_fixer",
      auto_commit: false,
    },
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
    loop_name: "review-and-fix",
    status: "paused",
    iteration_cap: 3,
    budget_tokens: 0,
    budget_on_exceeded: "escalate",
    tokens_used: 74_000,
    generation: 2,
    pause_requested: true,
    started_origin_kind: "manual",
    inputs: { task_name: "session-auth", reviewer: "reviewer", fixer: "review_fixer" },
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
    id: "looprun_done_review",
    loop_name: "review-and-fix",
    status: "done",
    iteration_cap: 3,
    budget_tokens: 0,
    budget_on_exceeded: "escalate",
    tokens_used: 96_000,
    generation: 3,
    started_origin_kind: "native_tool",
    last_progress_at: "2026-07-05T11:02:00Z",
    inputs: { task_name: "loop-catalog", reviewer: "reviewer", fixer: "review_fixer" },
  }),
  buildRun({
    id: "looprun_no-op",
    loop_name: "software-delivery",
    status: "no-op",
    iteration_cap: 50,
    budget_tokens: 2_000_000,
    tokens_used: 12_000,
    generation: 1,
    started_origin_kind: "cli",
    inputs: { slug: "empty-task-set" },
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
    best_generation: 12,
    best_score: 0.88,
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
    best_generation: 2,
    best_score: 0.76,
    tokens_used: 610_000,
    budget_tokens: 2_000_000,
    started_origin_kind: "cli",
    inputs: { slug: "no-progress" },
  }),
  buildRun({
    id: "looprun_stalled_review",
    loop_name: "review-and-fix",
    status: "stalled",
    iteration_cap: 3,
    budget_tokens: 0,
    budget_on_exceeded: "escalate",
    tokens_used: 152_000,
    generation: 3,
    started_origin_kind: "http",
    inputs: { task_name: "network-bridge", reviewer: "reviewer", fixer: "review_fixer" },
  }),
];

export const loopRunAggregatesFixture: LoopRunAggregates = {
  total: loopRunFixtures.length,
  live: 5,
  terminal: 9,
  succeeded: 2,
  failed: 1,
};

function loopRunFixture(id: string): LoopRun {
  const run = loopRunFixtures.find(candidate => candidate.id === id);
  if (!run) throw new Error(`Missing Loop run fixture ${id}`);
  return run;
}

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
    last_run: loopRunFixture("looprun_running"),
    aggregate_30d: { runs: 42, succeeded: 38, failed: 4 },
    success_rate_30d: 0.9,
  },
  {
    name: "review-and-fix",
    source: "marketplace",
    version: 1,
    description:
      "Review a named task with an agent, write inspectable findings, and remediate them until a round comes back clean.",
    catalog: {
      category: "Engineering",
      use_when:
        "You want the work on a task reviewed by an agent and every finding remediated until a round comes back clean.",
      keywords: ["reviews", "agents", "artifacts", "remediate"],
    },
    contract: reviewContract,
    inputs: {
      task_name: { type: "string", required: true },
      reviewer: { type: "agent", required: false, default: "reviewer" },
      fixer: { type: "agent", required: false, default: "review_fixer" },
      auto_commit: { type: "boolean", required: false, default: false },
    },
    start: [
      { kind: "manual" },
      { kind: "cli" },
      { kind: "http" },
      { kind: "uds" },
      { kind: "native_tool" },
      { kind: "trigger" },
      { kind: "webhook" },
    ],
    last_run: loopRunFixture("looprun_review_running"),
    aggregate_30d: { runs: 9, succeeded: 9, failed: 0 },
    success_rate_30d: 1,
  },
];

function buildDefinition(entry: LoopCatalogEntry): LoopDefinition {
  return {
    apiVersion: "compozy.loop/v1",
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

export const loopRunDetailFixtures = buildLoopRunDetailFixtures(loopRunFixtures, loopDetailByName);

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
  runtime_defaults: {
    worker: { provider: "openai", model: "gpt-5.4" },
    judge: { provider: "anthropic", model: "claude-sonnet-4" },
  },
  runtime_rules: [
    {
      match: { type: "implementation" },
      runtime: { reasoning: "high" },
    },
  ],
  no_progress_window: 3,
  reattempt_strategy: "failed_only",
};

export const loopAnnotationsFixture: LoopAnnotation[] = [
  { node_id: "plan", x: 120, y: 80 },
  { node_id: "implement", x: 360, y: 80 },
];
