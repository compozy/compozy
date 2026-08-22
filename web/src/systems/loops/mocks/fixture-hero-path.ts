import { buildLocalNetworkParticipationFixture } from "@/test/network-participation-fixtures";

import type { LoopDetail, LoopRun } from "../types";

/**
 * The workspace-effective Spec 1 lifecycle envelope the daemon reports on every Loop
 * detail read. The hero-path reliability tiles are derived from exactly these fields,
 * so the mock carries the same shape and provenance map the runtime does.
 */
export const heroEffectiveLifecycle: NonNullable<LoopDetail["effective_lifecycle"]> = {
  admission_horizon: "24h0m0s",
  liveness_silence_window: "10m0s",
  predicate_cost_limit: 200,
  resume_death_streak_limit: 3,
  retry_backoff_base: "30s",
  retry_backoff_max: "10m0s",
  retry_max_attempts: 3,
  retry_non_retryable: ["invalid_input", "permission_denied"],
  sources: {
    admission_horizon: "config",
    liveness_silence_window: "config",
    retry_backoff_base: "config",
    retry_backoff_max: "config",
    retry_max_attempts: "config",
  },
  wait_admission_attempts: 5,
  wait_admission_interval: "1m0s",
};

export const heroRunFixtures: LoopRun[] = [
  {
    profile_id: "00000000000000000000000000",
    profile_name: "default",
    workspace_id: "ws_default",
    id: "looprun_canceled",
    loop_name: "implement-tasks",
    status: "canceled",
    completion_state: "complete",
    generation: 1,
    iteration_cap: 50,
    tokens_used: 96_300,
    pause_requested: false,
    budget_tokens: 500_000,
    budget_wall_sec: 3_600,
    budget_on_exceeded: "halt",
    reattempt_strategy: "failed_only",
    resolved_network_participation: buildLocalNetworkParticipationFixture(),
    created_at: "2026-07-04T09:10:00Z",
    started_at: "2026-07-04T09:10:00Z",
    last_progress_at: "2026-07-04T09:28:00Z",
    definition_version: 0,
    definition_digest: "sha256:mock-loop-definition",
    start_metadata: {},
    started_origin_kind: "manual",
    started_by_ref: "pedro",
    inputs: { slug: "search-reindex" },
    forks: [],
  },
];
