import type {
  LoopAnnotation,
  LoopCatalogEntry,
  LoopConfig,
  LoopContract,
  LoopDefinition,
  LoopDetail,
  LoopEffectiveConfig,
  LoopRun,
  LoopRunAggregates,
} from "../types";
import { buildLocalNetworkParticipationFixture } from "@/test/network-participation-fixtures";
import { isTerminalLoopStatus } from "../lib/loop-formatters";
import { heroEffectiveLifecycle, heroRunFixtures } from "./fixture-hero-path";
import { fixtureGraph } from "./fixture-graph";
import { graphByName } from "./fixture-graphs";
import { qualityGateGraph } from "./fixture-quality-gate";
import { buildLoopRunDetailFixtures } from "./fixture-run-details";

export const MOCK_WORKSPACE_ID = "ws_default";

const implementTasksContract: LoopContract = {
  goal: "Implement every authored task under .compozy/tasks/{{ .inputs.slug }} in dependency order.",
  definition_of_done:
    "Every loaded task completed implementation, task-level validation, and tracking updates.",
  iteration_cap: 50,
  budget: { tokens: 0, wall_clock_sec: 0, on_exceeded: "halt" },
  no_progress: { window: 3 },
  terminal_states: ["done", "no-op", "blocked", "failed", "exhausted", "stalled"],
};

const orchestrateTasksContract: LoopContract = {
  goal: "Drive every authored task under .compozy/tasks/{{ .inputs.slug }} to completed by delegating each task to its own worker session.",
  definition_of_done:
    "Every task frontmatter carries status completed and no orchestrator-created worker session is nonterminal.",
  iteration_cap: 3,
  budget: { tokens: 0, wall_clock_sec: 0, on_exceeded: "halt" },
  no_progress: { window: 2 },
  terminal_states: ["done", "no-op", "blocked", "failed", "exhausted", "stalled"],
};

const qualityGateContract: LoopContract = {
  goal: "Exercise generic review, verification, and approval controls.",
  definition_of_done: "Every configured gate passes.",
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
  const definitionVersion = overrides.loop_name === "quality-gate-demo" ? 4 : 0;
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
    definition_version: definitionVersion,
    definition_digest: "sha256:mock-loop-definition",
    start_metadata: {},
    ...overrides,
  };
}

export const loopRunFixtures: LoopRun[] = [
  // Live runs (Active table + KPIs).
  buildRun({
    id: "looprun_running",
    loop_name: "implement-tasks",
    status: "running",
    generation: 1,
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
    loop_name: "quality-gate-demo",
    status: "needs-approval",
    generation: 3,
    tokens_used: 1_100_000,
    budget_tokens: 2_000_000,
    started_origin_kind: "cli",
    inputs: { goal: "Review billing webhook delivery" },
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
    loop_name: "implement-tasks",
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
    loop_name: "implement-tasks",
    status: "done",
    generation: 1,
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
    loop_name: "quality-gate-demo",
    status: "no-op",
    iteration_cap: 50,
    budget_tokens: 2_000_000,
    tokens_used: 12_000,
    generation: 1,
    started_origin_kind: "cli",
    inputs: { goal: "Exercise a no-op gate path" },
  }),
  buildRun({
    id: "looprun_blocked",
    loop_name: "quality-gate-demo",
    status: "blocked",
    generation: 2,
    tokens_used: 140_000,
    budget_tokens: 2_000_000,
    started_origin_kind: "cli",
    inputs: { goal: "Exercise a blocked gate path" },
  }),
  buildRun({
    id: "looprun_failed",
    loop_name: "implement-tasks",
    status: "failed",
    generation: 1,
    tokens_used: 340_000,
    budget_tokens: 2_000_000,
    started_origin_kind: "schedule",
    inputs: { slug: "task-action-failure" },
  }),
  buildRun({
    id: "looprun_exhausted",
    loop_name: "quality-gate-demo",
    status: "exhausted",
    generation: 50,
    best_generation: 12,
    best_score: 0.88,
    iteration_cap: 50,
    tokens_used: 1_900_000,
    budget_tokens: 2_000_000,
    started_origin_kind: "cli",
    inputs: { goal: "Exercise iteration exhaustion" },
  }),
  buildRun({
    id: "looprun_exhausted_budget",
    loop_name: "implement-tasks",
    status: "exhausted",
    generation: 1,
    tokens_used: 2_000_000,
    budget_tokens: 2_000_000,
    started_origin_kind: "manual",
    inputs: { slug: "token-budget" },
  }),
  buildRun({
    id: "looprun_stalled",
    loop_name: "quality-gate-demo",
    status: "stalled",
    generation: 4,
    best_generation: 2,
    best_score: 0.76,
    tokens_used: 610_000,
    budget_tokens: 2_000_000,
    started_origin_kind: "cli",
    inputs: { goal: "Exercise no-progress detection" },
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
  ...heroRunFixtures,
];

const terminalRunCount = loopRunFixtures.filter(run => isTerminalLoopStatus(run.status)).length;

export const loopRunAggregatesFixture: LoopRunAggregates = {
  total: loopRunFixtures.length,
  live: loopRunFixtures.length - terminalRunCount,
  terminal: terminalRunCount,
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
    name: "implement-tasks",
    source: "marketplace",
    version: 0,
    description: "Implement pending CompozyOS task files for one slug.",
    catalog: {
      category: "Engineering",
      use_when:
        "You have authored tasks under .compozy/tasks/<slug> and want them implemented in dependency order.",
      keywords: ["tasks", "implement", "engineering"],
    },
    contract: implementTasksContract,
    inputs: {
      slug: { type: "string", required: true },
      implementer: { type: "agent", required: false, default: "code_implementer" },
      auto_commit: { type: "boolean", required: false, default: false },
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
    name: "orchestrate-tasks",
    source: "marketplace",
    version: 0,
    description:
      "Delegate every authored task for one slug to a dedicated worker session conducted by an orchestrator agent.",
    catalog: {
      category: "Engineering",
      use_when:
        "You have authored tasks under .compozy/tasks/<slug> and want one orchestrator agent to conduct them in dependency order.",
      keywords: ["tasks", "orchestrate", "sessions", "engineering"],
    },
    contract: orchestrateTasksContract,
    inputs: {
      slug: { type: "string", required: true },
      orchestrator: { type: "agent", required: false, default: "general" },
    },
    start: [
      { kind: "manual" },
      { kind: "cli" },
      { kind: "http" },
      { kind: "uds" },
      { kind: "native_tool" },
    ],
    aggregate_30d: { runs: 0, succeeded: 0, failed: 0 },
    success_rate_30d: 0,
  },
  {
    name: "review-and-fix",
    source: "marketplace",
    version: 0,
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
    concurrency: entry.name === "quality-gate-demo" ? undefined : "forbid",
    graph: graphByName[entry.name] ?? fixtureGraph([], []),
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
  effective_lifecycle: heroEffectiveLifecycle,
}));

export const qualityGateDetail: LoopDetail = {
  name: "quality-gate-demo",
  source: "workspace",
  version: 4,
  description: "Exercise generic gate states in stories and tests.",
  catalog: {
    category: "Testing",
    use_when: "You need a custom Loop fixture that exercises gate controls.",
    keywords: ["gates", "review", "approval"],
  },
  definition: {
    apiVersion: "compozy.loop/v1",
    kind: "Loop",
    meta: {
      name: "quality-gate-demo",
      description: "Exercise generic gate states in stories and tests.",
      version: 4,
      catalog: {
        category: "Testing",
        use_when: "You need a custom Loop fixture that exercises gate controls.",
        keywords: ["gates", "review", "approval"],
      },
    },
    contract: qualityGateContract,
    graph: qualityGateGraph,
    inputs: {
      goal: { type: "string", required: true, description: "What to accomplish." },
      max_files: { type: "number", required: false, default: 20 },
    },
    start: [
      { kind: "manual" },
      { kind: "cli" },
      { kind: "http" },
      { kind: "uds" },
      { kind: "native_tool" },
      { kind: "schedule" },
    ],
  },
  effective_lifecycle: heroEffectiveLifecycle,
};

export const loopDetailByName = new Map(
  [...loopDetailFixtures, qualityGateDetail].map(detail => [detail.name, detail])
);

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
  // Resolved Loop environment: no node or Loop override, so runs execute at the
  // workspace root.
  environment: { mode: "root" },
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
  { node_id: "load_tasks", x: 120, y: 80 },
  { node_id: "implement", x: 360, y: 80 },
];
