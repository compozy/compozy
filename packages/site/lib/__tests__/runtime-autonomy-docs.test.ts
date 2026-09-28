import { existsSync, readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const siteRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..");
const runtimeRoot = resolve(siteRoot, "content/docs");

function readRuntimeDoc(...parts: string[]): string {
  return readFileSync(resolve(runtimeRoot, ...parts), "utf8");
}

function readJSON<T>(...parts: string[]): T {
  return JSON.parse(readRuntimeDoc(...parts)) as T;
}

function expectIncludesAll(content: string, values: string[]): void {
  for (const value of values) {
    expect(content).toContain(value);
  }
}

function expectExcludesAll(content: string, values: string[]): void {
  for (const value of values) {
    expect(content).not.toContain(value);
  }
}

describe("runtime autonomy docs", () => {
  it("documents the MVP execution boundary and manual operator control", () => {
    const overview = readRuntimeDoc("autonomy/index.mdx");
    const coordinator = readRuntimeDoc("autonomy/coordinator.mdx");
    const config = readRuntimeDoc("configuration/config-toml.mdx");

    expectIncludesAll(overview, [
      "Creating a task records intent only",
      "does not enqueue claimable work",
      "publish",
      "start",
      "approves",
      "execution boundary",
    ]);
    expectIncludesAll(coordinator, [
      "Task creation alone does not start a coordinator",
      "Manual sessions, task APIs, and run claims",
      "Global-scope runs do not auto-start a coordinator",
    ]);
    expectIncludesAll(config, [
      "[roles.coordinator]",
      "Task creation alone does not start a coordinator",
      "Role changes are classified `live`",
      "Global `$COMPOZY_HOME/config.toml` values",
      "workspace's `.compozy/config.toml` overlays them",
      "builtin `coordinator`",
    ]);
    expectExcludesAll(config, ["[autonomy.coordinator]", "default_ttl ="]);
  });

  it("documents task lease authority without exposing raw tokens in read paths", () => {
    const leases = readRuntimeDoc("autonomy/task-runs-and-leases.mdx");

    expectIncludesAll(leases, [
      "`claim_token_hash`",
      "The raw bearer lease token is internal to CompozyOS",
      "the calling session plus `run_id`",
      "One active lease per session",
      "Stale holders fail",
      "Never send raw lease credentials through prompts",
    ]);
  });

  it("exposes autonomy docs in runtime navigation without a marketing redesign", () => {
    const coreMeta = readJSON<{ pages: string[] }>("meta.json");
    const autonomyMeta = readJSON<Record<string, unknown>>("autonomy/meta.json");

    expect(coreMeta.pages).toContain("autonomy");
    expect(autonomyMeta).toMatchObject({
      title: "Autonomy",
      pages: [
        "index",
        "coordinator",
        "task-runs-and-leases",
        "execution-profiles",
        "review-gate",
        "notification-cursors",
        "safe-spawn",
      ],
    });
  });

  it("documents task execution profiles with truthful management surfaces and config lifecycle", () => {
    const profiles = readRuntimeDoc("autonomy/execution-profiles.mdx");
    const overview = readRuntimeDoc("autonomy/index.mdx");
    const config = readRuntimeDoc("configuration/config-toml.mdx");

    expectIncludesAll(profiles, [
      "[task.orchestration.profile]",
      "[task.orchestration.review]",
      "/api/tasks/{id}/execution-profile",
      "PUT",
      "DELETE",
      "409 Conflict",
      "compozy__task_execution_profile_get",
      "compozy__task_execution_profile_set",
      "compozy__task_execution_profile_delete",
      "Task setup",
      "View JSON",
      "configured session permission policy",
      "active run",
      "provider authorization",
      "worktree",
      "review verdicts",
    ]);
    expect(overview).toContain("/docs/autonomy/execution-profiles");
    expectIncludesAll(config, [
      "[task.orchestration]",
      "[task.orchestration.profile]",
      "[task.orchestration.review]",
      "default_coordinator_mode",
      "default_worker_mode",
      "allow_task_provider_override",
      "max_active_runs_per_workspace",
      'lifecycle="restart-required"',
      "Profile validation runs in `task.Service` when a profile is created or updated",
      "Workspace overlays may tighten or relax",
      "compozy config set",
      "compozy__config_*",
    ]);
  });
});

describe("runtime review-gate docs", () => {
  it("documents the post-terminal review gate, reviewer routing, and continuation runs", () => {
    const reviewGate = readRuntimeDoc("autonomy/review-gate.mdx");
    const overview = readRuntimeDoc("autonomy/index.mdx");
    const config = readRuntimeDoc("configuration/config-toml.mdx");

    expectIncludesAll(reviewGate, [
      "Authority boundary",
      "task.Service.RecordRunReview",
      "task.Service.BindRunReviewSession",
      "Review is opt-in",
      "post-terminal",
      "Lifecycle",
      "ReviewRouter",
      "review_required",
      "review_request_id",
      "idempotent on",
      "(run_id, review_round, attempt = 1)",
      "Review policy and outcomes",
      "on_success",
      "on_failure",
      "always",
      "approved",
      "rejected",
      "blocked",
      "error",
      "timeout",
      "invalid_output",
      "missing_work",
      "next_round_guidance",
      "delivery_id",
      "failure_policy",
      "rapid_terminal_limit",
      "Reviewer routing and binding",
      "allow_original_worker",
      "LookupReviewForSession",
      "ErrToolUnavailable",
      "Continuation runs",
      "parent_run_id",
      "review_id",
      "review_round",
      "continuation_reason",
      "review_rejected",
      "TaskContextBundle.ReviewContinuation",
      "Manage reviews from the CLI",
      "compozy task review request",
      "compozy task review list",
      "compozy task review show",
      "compozy task review submit",
      "Manage reviews through HTTP and UDS",
      "/api/task-runs/{id}/reviews",
      "/api/tasks/{id}/reviews",
      "/api/task-reviews/{id}",
      "/api/task-reviews/{id}/verdict",
      "submitTaskRunReviewVerdict",
      "requestTaskRunReview",
      "listTaskRunReviews",
      "listTaskReviews",
      "getTaskRunReview",
      "Reviewer-bound native tool",
      "submit_run_review",
      "compozy__task_run_review_submit",
      "references/tasks-and-orchestration.md",
      "task-service state, not skill metadata",
      "Inspect from the operator web UI",
      "Runs",
      "Activity",
      "Inspect → Stream",
      "Review events",
      "task.run_review_requested",
      "task.run_review_bound",
      "task.run_review_recorded",
      "task.run_review_approved",
      "task.run_review_rejected",
      "task.run_review_blocked",
      "task.run_review_error",
      "task.run_review_timeout",
      "task.run_review_invalid_output",
      "task.run_review_retry_enqueued",
      "CompozyOS skill expectations",
      "Config lifecycle",
      "[task.orchestration.review]",
    ]);
    expectExcludesAll(reviewGate, [
      "/docs/agent/context",
      "route_run_review_request",
      "task.run_review_routed",
      "task.run_review_circuit_opened",
      "task.run_review_canceled",
      "submitting a typed\n   `error` outcome",
      "Circuit reset is explicit through the API/UDS/CLI/task-service path",
      "deadline, actor identity, idempotency, bounds, and round limits",
      "allow_coordinator",
      "ParticipantPolicy",
    ]);
    expect(overview).toContain("/docs/autonomy/review-gate");
    expectIncludesAll(config, [
      "default_policy",
      "max_review_attempts",
      "rapid_terminal_window",
      "missing_work_max_items",
      "next_round_guidance_max_bytes",
    ]);
  });
});

describe("runtime notification cursor docs", () => {
  it("documents task projection progress and SSE resume", () => {
    const notifications = readRuntimeDoc("autonomy/notification-cursors.mdx");
    const overview = readRuntimeDoc("autonomy/index.mdx");

    expectIncludesAll(notifications, [
      "notification_cursors",
      "task-status projection",
      "sequence",
      "latest_event_seq",
      "Last-Event-ID",
      "after_sequence",
      "Inspect → Stream",
      "/api/tasks/{id}/stream",
      "does not grant task or run authority",
    ]);
    expectExcludesAll(notifications, [
      "matching run-detail context render the same cursor diagnostics",
      'shows "No delivery yet"',
      "route it through the existing operator surface",
      "bound delivery diagnostics and event payload sizes",
    ]);
    expect(overview).toContain("/docs/autonomy/notification-cursors");
  });
});

describe("bundled CompozyOS skill docs", () => {
  it("describes the CompozyOS skill as instructional only and lists contextual references", () => {
    const bundled = readRuntimeDoc("skills/bundled.mdx");

    expectIncludesAll(bundled, [
      "`compozy`",
      "references/tools-and-skills.md",
      "references/native-tools.md",
      "references/tasks-and-orchestration.md",
      "instructional",
      "binding is the authority",
      "task.Service.RecordRunReview",
      "submit_run_review",
      "contextual prompt help",
    ]);
  });
});

describe("generated task review CLI references", () => {
  const requiredReviewPages = [
    "cli/task/review/index.mdx",
    "cli/task/review/request.mdx",
    "cli/task/review/list.mdx",
    "cli/task/review/show.mdx",
    "cli/task/review/submit.mdx",
  ];

  it("keeps regenerated CLI reference pages present for the review command group", () => {
    for (const page of requiredReviewPages) {
      expect(existsSync(resolve(runtimeRoot, page))).toBe(true);
    }
  });

  it("documents review CLI flags exactly once on each generated page", () => {
    const request = readRuntimeDoc("cli/task/review/request.mdx");
    const list = readRuntimeDoc("cli/task/review/list.mdx");
    const show = readRuntimeDoc("cli/task/review/show.mdx");
    const submit = readRuntimeDoc("cli/task/review/submit.mdx");

    expectIncludesAll(request, ["--policy", "--reason", "--round", "--attempt"]);
    expectIncludesAll(list, ["--task", "--run", "--status", "--reviewer-session", "--last"]);
    expectIncludesAll(show, ["help for show"]);
    expectIncludesAll(submit, [
      "--outcome",
      "--confidence",
      "--reason",
      "--missing-work",
      "--missing-work-json",
      "--next-round-guidance",
      "--review-text",
      "--delivery-id",
      "--run",
    ]);
  });
});

describe("generated task execution profile CLI references", () => {
  const requiredProfilePages = [
    "cli/task/profile/index.mdx",
    "cli/task/profile/inspect.mdx",
    "cli/task/profile/update.mdx",
    "cli/task/profile/delete.mdx",
  ];

  it("keeps regenerated CLI reference pages present for the profile command group", () => {
    for (const page of requiredProfilePages) {
      expect(existsSync(resolve(runtimeRoot, page))).toBe(true);
    }
  });

  it("documents the profile update --profile JSON flag on the generated CLI page", () => {
    const update = readRuntimeDoc("cli/task/profile/update.mdx");
    const inspect = readRuntimeDoc("cli/task/profile/inspect.mdx");
    const del = readRuntimeDoc("cli/task/profile/delete.mdx");

    expectIncludesAll(update, ["--profile", "Replace one task execution profile"]);
    expectIncludesAll(inspect, ["Show one task execution profile", "-o, --output"]);
    expectIncludesAll(del, ["Delete one task execution profile", "-o, --output"]);
    for (const content of [update, inspect, del]) {
      expect(content).not.toContain("--patch");
    }
  });
});

describe("generated autonomy CLI references", () => {
  const requiredPages = [
    "cli/me/index.mdx",
    "cli/me/context.mdx",
    "cli/spawn.mdx",
    "cli/task/next.mdx",
    "cli/task/heartbeat.mdx",
    "cli/task/complete.mdx",
    "cli/task/fail.mdx",
    "cli/task/release.mdx",
    "cli/task/retry.mdx",
  ];

  it("keeps regenerated command pages present for agent-facing autonomy commands", () => {
    for (const page of requiredPages) {
      expect(existsSync(resolve(runtimeRoot, page))).toBe(true);
    }
  });

  it("lists exact implemented flags for task and spawn examples", () => {
    const taskNext = readRuntimeDoc("cli/task/next.mdx");
    const heartbeat = readRuntimeDoc("cli/task/heartbeat.mdx");
    const complete = readRuntimeDoc("cli/task/complete.mdx");
    const fail = readRuntimeDoc("cli/task/fail.mdx");
    const release = readRuntimeDoc("cli/task/release.mdx");
    const retry = readRuntimeDoc("cli/task/retry.mdx");
    const spawn = readRuntimeDoc("cli/spawn.mdx");

    expectIncludesAll(taskNext, ["--wait", "--lease-seconds", "--capability", "--priority-min"]);
    expectIncludesAll(heartbeat, ["--lease-seconds"]);
    expectIncludesAll(complete, ["--result"]);
    expectIncludesAll(fail, ["--reason", "--metadata"]);
    expectIncludesAll(release, ["--reason"]);
    for (const content of [heartbeat, complete, fail, release, retry]) {
      expect(content).not.toContain("--claim-token");
    }
    expectIncludesAll(spawn, [
      "--agent",
      "--ttl-seconds",
      "--provider",
      "--model",
      "--role",
      "--tool",
      "--skill",
      "--mcp-server",
      "--workspace-path",
    ]);
  });
});
