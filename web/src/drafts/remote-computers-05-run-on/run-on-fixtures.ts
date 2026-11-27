/**
 * Local fixtures for Screen 05 (`remote-computers-05-run-on`):
 * realistic coding agents (`AgentPayload[]`), per-computer worktrees, and
 * effective automation timezones per computer.
 *
 * Imports shared computers and projects from `remote-computers-fixtures*`
 * without redefining any computer, device, or account preset.
 */
import type { AgentPayload } from "@/systems/agent/types";
import { buildWorktreeFixture } from "@/systems/workspace/mocks/worktree-fixtures";
import type { WorktreePayload } from "@/systems/workspace/types";

const FIXTURE_DIGEST = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa";

/** Realistic on-demand coding agents matching `UNIFIED_SESSIONS` (`general` default). */
export const RUN_ON_AGENTS: AgentPayload[] = [
  {
    name: "general",
    label: "General",
    kind: "on_demand",
    provider: "claude",
    model: "claude-sonnet-4-6",
    prompt: "General-purpose software engineering agent for everyday coding tasks.",
    origin: "global",
    definition_digest: FIXTURE_DIGEST,
    resolved_appearance: {
      version: 1,
      avatar: "generated",
      seed: "general",
      generation: 1,
      color: "#7c5cff",
    },
  },
  {
    name: "reviewer",
    label: "Code Reviewer",
    kind: "on_demand",
    provider: "claude",
    model: "claude-opus-4-6",
    category_path: ["engineering"],
    prompt: "Review diffs for correctness, regressions, and test coverage.",
    origin: "global",
    definition_digest: FIXTURE_DIGEST,
    resolved_appearance: {
      version: 1,
      avatar: "generated",
      seed: "reviewer",
      generation: 1,
      color: "#22a06b",
    },
  },
  {
    name: "frontend",
    label: "Frontend",
    kind: "on_demand",
    provider: "codex",
    model: "gpt-5.4",
    category_path: ["engineering"],
    prompt: "Build and polish React UI components, design tokens, and browser tests.",
    origin: "global",
    definition_digest: FIXTURE_DIGEST,
    resolved_appearance: {
      version: 1,
      avatar: "generated",
      seed: "frontend",
      generation: 1,
      color: "#0b84ff",
    },
  },
  {
    name: "release",
    label: "Release",
    kind: "on_demand",
    provider: "codex",
    model: "gpt-5.4",
    category_path: ["ops"],
    prompt: "Verify release builds, changelogs, and deployment readiness.",
    origin: "global",
    definition_digest: FIXTURE_DIGEST,
    resolved_appearance: {
      version: 1,
      avatar: "generated",
      seed: "release",
      generation: 1,
      color: "#d97706",
    },
  },
  {
    name: "docs",
    label: "Docs",
    kind: "on_demand",
    provider: "gemini",
    category_path: ["writing"],
    prompt: "Keep reference documentation and guides aligned with code changes.",
    origin: "global",
    definition_digest: FIXTURE_DIGEST,
    resolved_appearance: {
      version: 1,
      avatar: "generated",
      seed: "docs",
      generation: 1,
      color: "#0ea5e9",
    },
  },
];

/** Worktrees per project ID so `SessionEnvironmentField` re-scopes with computer + project. */
const WORKTREES_BY_PROJECT: Readonly<Record<string, readonly WorktreePayload[]>> = {
  ws_devbox_api: [
    buildWorktreeFixture({
      id: "wt_devbox_pg_pool",
      workspace_id: "ws_devbox_api",
      name: "fix-pg-pool-flake",
      branch: "fix/pg-pool-flake",
      path: "/home/pedro/.compozy/worktrees/api/fix-pg-pool-flake",
    }),
    buildWorktreeFixture({
      id: "wt_devbox_rate_limit",
      workspace_id: "ws_devbox_api",
      name: "rate-limit-buckets",
      branch: "feat/rate-limit-buckets",
      path: "/home/pedro/.compozy/worktrees/api/rate-limit-buckets",
    }),
  ],
  ws_devbox_billing: [
    buildWorktreeFixture({
      id: "wt_devbox_stripe",
      workspace_id: "ws_devbox_billing",
      name: "stripe-v2",
      branch: "feat/stripe-v2",
      path: "/home/pedro/.compozy/worktrees/billing-worker/stripe-v2",
    }),
  ],
  ws_studio_api: [
    buildWorktreeFixture({
      id: "wt_studio_mlx",
      workspace_id: "ws_studio_api",
      name: "mlx-embeddings",
      branch: "perf/mlx-embeddings",
      path: "/Users/pedro/.compozy/worktrees/api/mlx-embeddings",
    }),
  ],
  ws_studio_compozy: [
    buildWorktreeFixture({
      id: "wt_studio_release",
      workspace_id: "ws_studio_compozy",
      name: "release-0-8",
      branch: "release/v0.8.0",
      path: "/Users/pedro/.compozy/worktrees/compozy/release-0-8",
    }),
  ],
  ws_local_compozy: [
    buildWorktreeFixture({
      id: "wt_local_settings",
      workspace_id: "ws_local_compozy",
      name: "settings-nav",
      branch: "feat/settings-nav",
      path: "/Users/pedro/.compozy/worktrees/compozy/settings-nav",
    }),
  ],
  ws_local_api: [
    buildWorktreeFixture({
      id: "wt_local_webhook",
      workspace_id: "ws_local_api",
      name: "webhook-ingress",
      branch: "feat/webhook-ingress",
      path: "/Users/pedro/.compozy/worktrees/api/webhook-ingress",
    }),
  ],
  ws_ws_trainer: [
    buildWorktreeFixture({
      id: "wt_ws_batch",
      workspace_id: "ws_ws_trainer",
      name: "batch-scheduler",
      branch: "refactor/batch-scheduler",
      path: "/home/pedro/.compozy/worktrees/trainer/batch-scheduler",
    }),
  ],
};

export function worktreesForProject(projectId: string | undefined): readonly WorktreePayload[] {
  if (!projectId) return [];
  return WORKTREES_BY_PROJECT[projectId] ?? [];
}

/**
 * Effective automation timezone per computer (`automation.timezone` override else host zone,
 * `_uiux.md` U4). Displayed beside the schedule clock (`09:00 · America/Sao_Paulo`).
 */
export const COMPUTER_AUTOMATION_TIMEZONES: Readonly<Record<string, string>> = {
  local: "America/Sao_Paulo",
  cmp_7f3k2q9x: "America/Sao_Paulo",
  cmp_1c9d0a2b: "America/Los_Angeles",
  cmp_9r5k3n7q: "America/Sao_Paulo",
  cmp_3v8q1z4m: "Europe/Berlin",
  cmp_4b2m9x1p: "UTC",
  cmp_8e7f6d5c: "UTC",
};

export function automationTimeZoneForTarget(target: string): string {
  return COMPUTER_AUTOMATION_TIMEZONES[target] ?? "UTC";
}
