import { buildLocalNetworkParticipationFixture } from "@/test/network-participation-fixtures";
import {
  SPEC_CYCLE_FINALIZE_REVIEW_ROUND_KIND,
  SPEC_CYCLE_WRITE_REVIEW_ARTIFACTS_KIND,
} from "../../mocks/fixture-action-kinds";

import type {
  LoopDefinition,
  LoopDefinitionGraph,
  LoopRunEventFrame,
  LoopRunGeneration,
  LoopRunGenerationOutput,
  LoopRunRecord,
} from "../../types";

/** Shared story data for the bundled agent-authored review Loop. */

export const STORY_NOW = Date.now();

export function minutesAgo(minutes: number): string {
  return new Date(STORY_NOW - minutes * 60_000).toISOString();
}

export const reviewAndFixDefinition: LoopDefinition = {
  apiVersion: "compozy.loop/v1",
  kind: "Loop",
  meta: {
    name: "review-and-fix",
    version: 1,
    description:
      "Review a named task with an agent, write inspectable findings, and remediate them until a round comes back clean.",
    catalog: {
      category: "Engineering",
      use_when:
        "You want the work on a task reviewed by an agent and every finding remediated until a round comes back clean.",
      keywords: ["reviews", "agents", "artifacts", "remediate"],
    },
  },
  concurrency: "forbid",
  inputs: {
    task_name: { type: "string", required: true },
    reviewer: { type: "agent", default: "reviewer" },
    fixer: { type: "agent", default: "review_fixer" },
    auto_commit: { type: "boolean", default: false },
  },
  contract: {
    goal: "Review the work for task billing-webhooks and remediate every valid finding.",
    definition_of_done:
      "A new agent review round for the task returns no issues after all earlier findings were triaged, remediated, verified, and finalized.",
    stop_when: "nodes.review.status == 'succeeded' && size(nodes.review.output.issues) == 0",
    iteration_cap: 3,
    budget: { tokens: 0, wall_clock_sec: 0, on_exceeded: "escalate" },
    no_progress: { window: 2 },
    terminal_states: ["done", "no-op", "blocked", "failed", "exhausted", "stalled"],
  },
  graph: {
    nodes: [
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
        collection: "{{ .nodes.write_artifacts.output.batches }}",
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
    edges: [
      { from: "review", to: "has_issues" },
      { from: "has_issues", to: "write_artifacts" },
      { from: "write_artifacts", to: "fix_batches" },
      { from: "fix_batches", to: "fix_batch" },
      { from: "fix_batch", to: "collect_fixes" },
      { from: "collect_fixes", to: "finalize_round" },
    ],
  } as unknown as LoopDefinitionGraph,
  start: [
    { kind: "manual" },
    { kind: "cli" },
    { kind: "http" },
    { kind: "uds" },
    { kind: "native_tool" },
    { kind: "trigger" },
    { kind: "webhook" },
  ],
};

export const genericWatchDefinition: LoopDefinition = {
  apiVersion: "compozy.loop/v1",
  kind: "Loop",
  meta: {
    name: "inbox-watch",
    version: 1,
    description: "Wait for inbox events and process each ready batch.",
    catalog: { category: "Operations" },
  },
  inputs: {
    inbox: { type: "string", required: true },
  },
  contract: {
    goal: "Process every ready inbox event.",
    definition_of_done: "Each ready event is processed before the next poll.",
    iteration_cap: 0,
    budget: { tokens: 0, wall_clock_sec: 0, on_exceeded: "halt" },
    no_progress: { window: 3 },
    terminal_states: ["done", "no-op", "blocked", "failed", "exhausted", "stalled"],
  },
  graph: {
    nodes: [
      { id: "watch_inbox", class: "source", kind: "watch-source" },
      { id: "handle_event", class: "action", kind: "run-agent" },
    ],
    edges: [{ from: "watch_inbox", to: "handle_event" }],
  } as unknown as LoopDefinitionGraph,
  start: [{ kind: "manual" }],
};

/** Metric-bearing Loop used only by the score/best/provenance visual states. */
export const metricRatchetDefinition: LoopDefinition = {
  apiVersion: "compozy.loop/v1",
  kind: "Loop",
  meta: {
    name: "quality-ratchet",
    version: 1,
    description: "Improve one candidate while preserving the strongest generation.",
    catalog: { category: "Testing" },
  },
  concurrency: "allow",
  inputs: { candidate: { type: "string", required: true } },
  contract: {
    goal: "Improve the candidate until its quality score reaches 0.95.",
    definition_of_done: "The quality metric reaches 0.95.",
    stop_when: "best.score >= 0.95",
    iteration_cap: 3,
    budget: { tokens: 0, wall_clock_sec: 0, on_exceeded: "halt" },
    no_progress: { window: 5 },
    terminal_states: ["done", "blocked", "failed", "exhausted", "stalled"],
  },
  graph: {
    nodes: [
      { id: "draft", class: "action", kind: "run-agent" },
      {
        id: "quality",
        class: "control",
        kind: "gate",
        criteria: [
          {
            id: "quality_score",
            type: "agent-judge",
            agent: "quality-judge",
            rubric: "Score the completed candidate.",
            metric: { direction: "maximize" },
          },
        ],
        verdict_policy: "fixed_passes",
        on_result: { fail: "revise" },
        max_revisions: 10,
      },
    ],
    edges: [{ from: "draft", to: "quality" }],
  } as unknown as LoopDefinitionGraph,
  start: [{ kind: "manual" }, { kind: "cli" }, { kind: "http" }, { kind: "uds" }],
};

export function reviewAndFixRun(overrides: Partial<LoopRunRecord> = {}): LoopRunRecord {
  return {
    id: "r-7c4e19",
    workspace_id: "ws_default",
    loop_name: "review-and-fix",
    status: "running",
    completion_state: "complete",
    generation: 2,
    reattempt_strategy: "failed_only",
    created_at: minutesAgo(22),
    started_at: minutesAgo(22),
    last_progress_at: minutesAgo(4),
    started_by_kind: "user",
    started_by_ref: "operator",
    started_origin_kind: "cli",
    started_origin_ref: "loop run",
    definition_version: 1,
    definition_digest: "sha256:agent-authored-review",
    iteration_cap: 3,
    budget_tokens: 0,
    budget_wall_sec: 0,
    budget_on_exceeded: "escalate",
    tokens_used: 68_000,
    pause_requested: false,
    inputs: {
      task_name: "billing-webhooks",
      reviewer: "reviewer",
      fixer: "review_fixer",
      auto_commit: false,
    },
    resolved_network_participation: buildLocalNetworkParticipationFixture(),
    ...overrides,
    forks: overrides.forks ?? [],
  };
}

export function genericWatchRun(overrides: Partial<LoopRunRecord> = {}): LoopRunRecord {
  return {
    ...reviewAndFixRun(),
    id: "r-watch-01",
    loop_name: "inbox-watch",
    status: "watching",
    generation: 4,
    definition_digest: "sha256:generic-watch",
    iteration_cap: 0,
    budget_on_exceeded: "halt",
    tokens_used: 21_000,
    inputs: { inbox: "support" },
    ...overrides,
  };
}

export function metricRatchetRun(overrides: Partial<LoopRunRecord> = {}): LoopRunRecord {
  return {
    ...reviewAndFixRun(),
    loop_name: "quality-ratchet",
    definition_digest: "sha256:metric-ratchet",
    iteration_cap: 3,
    reattempt_strategy: "full_body",
    inputs: { candidate: "release-candidate" },
    ...overrides,
  };
}

type FrameBuilder = (
  kind: LoopRunEventFrame["kind"],
  minutes: number,
  payload: Record<string, unknown>
) => LoopRunEventFrame;

export function createFrameFactory(runID = "r-7c4e19"): FrameBuilder {
  let seq = 0;
  return (kind, minutes, payload) => {
    seq += 1;
    const frame = {
      id: `loopevt_${seq}`,
      seq,
      kind,
      loop_run_id: runID,
      workspace_id: "ws_default",
      at: minutesAgo(minutes),
      payload,
    };
    // Story fixtures deliberately assemble every event kind through one builder;
    // each call site below owns the matching kind-specific payload.
    return frame as LoopRunEventFrame;
  };
}

export function nodePayload(
  nodeId: string,
  generation: number,
  extra: Record<string, unknown> = {}
): Record<string, unknown> {
  return { node_id: nodeId, generation, ...extra };
}

/** Shared first-round history; offset preserves chronological event ordering. */
export function roundOneFrames(frame: FrameBuilder, offset = 0): LoopRunEventFrame[] {
  return [
    frame("generation_started", offset + 22, {
      generation: 1,
      parent_generation: 0,
      origin: "initial",
      reattempt_strategy: "failed_only",
    }),
    frame(
      "node_succeeded",
      offset + 21,
      nodePayload("review", 1, { task_id: "task_review", task_run_id: "tr_100" })
    ),
    frame("node_succeeded", offset + 20, nodePayload("has_issues", 1)),
    frame(
      "node_succeeded",
      offset + 19,
      nodePayload("write_artifacts", 1, { task_id: "task_write", task_run_id: "tr_101" })
    ),
    frame(
      "node_succeeded",
      offset + 10,
      nodePayload("fix_batch", 1, { item_index: 1, task_id: "task_fix", task_run_id: "tr_102" })
    ),
    frame(
      "node_succeeded",
      offset + 10,
      nodePayload("fix_batch", 1, { item_index: 2, task_id: "task_fix", task_run_id: "tr_103" })
    ),
    frame(
      "node_failed",
      offset + 6,
      nodePayload("fix_batch", 1, {
        item_index: 3,
        task_id: "task_fix",
        task_run_id: "tr_104",
        output_ref: JSON.stringify({
          kind: "action_failure",
          code: "incomplete_batch",
          cause: "Two issue files did not receive a structured result.",
          recovery: "Re-run the failed artifact batch.",
        }),
      })
    ),
  ];
}

export function generationsFor(
  branchThree: LoopRunGenerationOutput["status"],
  remaining: LoopRunGenerationOutput["status"] = "pending"
): LoopRunGeneration[] {
  return [
    {
      generation: 2,
      parent_generation: 1,
      origin: "gate_revise",
      route_causes: [],
      verdicts: [],
      outputs: [
        { node_id: "review", status: "reused", generation: 2 },
        { node_id: "has_issues", status: "reused", generation: 2 },
        { node_id: "write_artifacts", status: "reused", generation: 2 },
        { node_id: "fix_batch", status: "reused", generation: 2, item_index: 1 },
        { node_id: "fix_batch", status: "reused", generation: 2, item_index: 2 },
        {
          node_id: "fix_batch",
          status: branchThree,
          generation: 2,
          item_index: 3,
          task_run_id: "tr_204",
          resolved_runtime: {
            provider: "openai",
            model: "gpt-5.4",
            reasoning: "high",
            source: { provider: "run", model: "frontmatter", reasoning: "config" },
          },
        },
        { node_id: "collect_fixes", status: remaining, generation: 2 },
        { node_id: "finalize_round", status: remaining, generation: 2 },
      ],
    },
  ];
}
