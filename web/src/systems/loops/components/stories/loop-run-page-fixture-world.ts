import { buildLocalNetworkParticipationFixture } from "@/test/network-participation-fixtures";

import type {
  LoopDefinition,
  LoopDefinitionGraph,
  LoopRunEventFrame,
  LoopRunGeneration,
  LoopRunGenerationOutput,
  LoopRunRecord,
} from "../../types";

/**
 * Shared `reviews-watch` story world mirroring the canonical prototypes. The
 * scenario module layers state-specific event sequences on this stable base.
 */

export const STORY_NOW = Date.now();

export function minutesAgo(minutes: number): string {
  return new Date(STORY_NOW - minutes * 60_000).toISOString();
}

export const reviewsWatchDefinition: LoopDefinition = {
  apiVersion: "agh.loop/v1",
  kind: "Loop",
  meta: {
    name: "reviews-watch",
    version: 3,
    description: "Watches a PR and resolves every review comment.",
    catalog: { category: "watch" },
  },
  concurrency: "forbid",
  inputs: {
    pr: { type: "string", required: true, description: "The pull request to watch." },
    fixer: { type: "ref", ref: { kind: "agent" }, description: "The fix agent." },
  },
  contract: {
    goal: "Resolve every review comment on PR #128",
    definition_of_done:
      "Done when a fresh CodeRabbit review of the current changes reports zero unresolved comments.",
    stop_when: "review.unresolved == 0",
    iteration_cap: 0,
    budget: { tokens: 1_500_000, wall_clock_sec: 2_700, on_exceeded: "escalate" },
    no_progress: { window: 2, hash_fields: ["gate_verdict"] },
    terminal_states: ["done", "no-op", "blocked", "failed", "exhausted", "stalled"],
    verification: [
      {
        id: "all_issues_handled",
        type: "agent-judge",
        agent: "agent-judge",
        rubric: "Every comment has a decision; valid ones a fix with tests.",
      },
    ],
  },
  graph: {
    nodes: [
      {
        id: "watch_pr",
        class: "source",
        kind: "watch-events",
        watch: { poll: "30s", settle: "20s" },
        events: [{ kind: "review.completed" }],
      },
      { id: "fetch_issues", class: "action", kind: "run-agent" },
      {
        id: "split_batches",
        class: "control",
        kind: "fan-out",
        batch_size: 10,
        max_parallel: 1,
        max_fan_out: 64,
      },
      { id: "fix_batches", class: "action", kind: "run-agent" },
      {
        id: "check_all",
        class: "control",
        kind: "gate",
        verdict_policy: "revise_until_clean",
        criteria: [{ id: "all_issues_handled", type: "agent-judge" }],
      },
      { id: "resolve_threads", class: "action", kind: "run-agent" },
      { id: "push_changes", class: "action", kind: "run-agent" },
    ],
    edges: [
      { from: "watch_pr", to: "fetch_issues" },
      { from: "fetch_issues", to: "split_batches" },
      { from: "split_batches", to: "fix_batches" },
      { from: "fix_batches", to: "check_all" },
      { from: "check_all", to: "resolve_threads" },
      { from: "resolve_threads", to: "push_changes" },
    ],
  } as unknown as LoopDefinitionGraph,
};

export function reviewsWatchRun(overrides: Partial<LoopRunRecord> = {}): LoopRunRecord {
  return {
    id: "r-7c4e19",
    workspace_id: "ws_default",
    loop_name: "reviews-watch",
    status: "running",
    generation: 2,
    reattempt_strategy: "failed_only",
    created_at: minutesAgo(22),
    started_at: minutesAgo(22),
    last_progress_at: minutesAgo(4),
    started_by_kind: "user",
    started_by_ref: "operator",
    started_origin_kind: "webhook",
    started_origin_ref: "pr-opened",
    definition_version: 3,
    definition_digest: "sha256:4f9c2a1e8b",
    iteration_cap: 0,
    budget_tokens: 1_500_000,
    budget_wall_sec: 2_700,
    budget_on_exceeded: "escalate",
    tokens_used: 268_000,
    pause_requested: false,
    inputs: { pr: "128", fixer: "review-fixer" },
    resolved_network_participation: buildLocalNetworkParticipationFixture(),
    ...overrides,
  };
}

type FrameBuilder = (
  kind: LoopRunEventFrame["kind"],
  minutes: number,
  payload: Record<string, unknown>
) => LoopRunEventFrame;

export function createFrameFactory(): FrameBuilder {
  let seq = 0;
  return (kind, minutes, payload) => {
    seq += 1;
    return {
      id: `loopevt_${seq}`,
      seq,
      kind,
      loop_run_id: "r-7c4e19",
      workspace_id: "ws_default",
      at: minutesAgo(minutes),
      payload,
    };
  };
}

export function nodePayload(
  nodeId: string,
  generation: number,
  extra: Record<string, unknown> = {}
): Record<string, unknown> {
  return { node_id: nodeId, generation, ...extra };
}

export const REVISE_ISSUES = [
  { id: "issue_022", note: "no decision recorded in the group harvest" },
  { id: "issue_024", note: "triaged valid but no fix landed" },
];

/** Shared round-one history; offset preserves chronological event ordering. */
export function roundOneFrames(frame: FrameBuilder, offset = 0): LoopRunEventFrame[] {
  return [
    frame("status_changed", offset + 22, {
      from: "watching",
      to: "running",
      status: "running",
      cause: "watch_events",
    }),
    frame(
      "node_succeeded",
      offset + 22,
      nodePayload("fetch_issues", 1, { task_id: "task_fetch", task_run_id: "tr_101" })
    ),
    frame(
      "node_succeeded",
      offset + 10,
      nodePayload("fix_batches", 1, { item_index: 1, task_id: "task_fix", task_run_id: "tr_102" })
    ),
    frame(
      "node_succeeded",
      offset + 10,
      nodePayload("fix_batches", 1, { item_index: 2, task_id: "task_fix", task_run_id: "tr_103" })
    ),
    frame(
      "node_failed",
      offset + 6,
      nodePayload("fix_batches", 1, {
        item_index: 3,
        task_id: "task_fix",
        task_run_id: "tr_104",
        output_ref: JSON.stringify({
          kind: "action_failure",
          code: "incomplete_batch",
          cause: "2 of its 4 comments weren't fully handled.",
          recovery: "Re-run the failed group.",
        }),
      })
    ),
    frame("gate_verdict", offset + 5, {
      node_id: "check_all",
      generation: 1,
      verdict: "revise",
      confidence: 0.91,
      criteria: [
        {
          id: "all_issues_handled",
          type: "agent-judge",
          status: "revise",
          note: "two open points",
        },
      ],
      blocking_issues: REVISE_ISSUES,
    }),
  ];
}

export function generationsFor(
  branchThree: LoopRunGenerationOutput["status"],
  remaining: LoopRunGenerationOutput["status"] = "pending"
): LoopRunGeneration[] {
  return [
    {
      generation: 2,
      outputs: [
        { node_id: "fetch_issues", status: "reused", generation: 2 },
        { node_id: "fix_batches", status: "reused", generation: 2, item_index: 1 },
        { node_id: "fix_batches", status: "reused", generation: 2, item_index: 2 },
        {
          node_id: "fix_batches",
          status: branchThree,
          generation: 2,
          item_index: 3,
          task_run_id: "tr_204",
        },
        { node_id: "check_all", status: remaining, generation: 2 },
        { node_id: "resolve_threads", status: remaining, generation: 2 },
        { node_id: "push_changes", status: remaining, generation: 2 },
      ],
    },
  ];
}
