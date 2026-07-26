import { existsSync, readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const siteRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..");
const runtimeRoot = resolve(siteRoot, "content/runtime");

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
    const overview = readRuntimeDoc("core/autonomy/index.mdx");
    const coordinator = readRuntimeDoc("core/autonomy/coordinator.mdx");
    const config = readRuntimeDoc("core/configuration/config-toml.mdx");

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
      "changing workspace coordination affects future coordinated runs only",
    ]);
    expectIncludesAll(config, [
      "[roles.coordinator]",
      "Task creation alone does not start a coordinator",
      "Role changes are classified `live`",
      "Global `$AGH_HOME/config.toml` values",
      "workspace's `.agh/config.toml` overlays them",
      "builtin `coordinator`",
    ]);
    expectExcludesAll(config, ["[autonomy.coordinator]", "default_ttl ="]);
  });

  it("documents task leases and channel authority without exposing raw tokens in read paths", () => {
    const leases = readRuntimeDoc("core/autonomy/task-runs-and-leases.mdx");
    const channels = readRuntimeDoc("core/autonomy/coordination-channels.mdx");

    expectIncludesAll(leases, [
      "`claim_token_hash`",
      "The raw bearer lease token is internal to AGH",
      "the calling session plus `run_id`",
      "One active lease per session",
      "Stale holders fail",
      "Never send raw lease credentials through `agh ch send`",
    ]);
    expectIncludesAll(channels, [
      "There is no channel bound to every run",
      "`resolved_network_participation`",
      "Conversation is not ownership",
      "only authority even when every participant",
      "directly addressed or explicitly mentioned",
      "`usage_unavailable`",
    ]);
  });

  it("exposes autonomy docs in runtime navigation without a marketing redesign", () => {
    const coreMeta = readJSON<{ pages: string[] }>("core/meta.json");
    const autonomyMeta = readJSON<Record<string, unknown>>("core/autonomy/meta.json");

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
        "notification-presets",
        "coordination-channels",
        "safe-spawn",
      ],
    });
  });

  it("documents task execution profiles with truthful management surfaces and config lifecycle", () => {
    const profiles = readRuntimeDoc("core/autonomy/execution-profiles.mdx");
    const overview = readRuntimeDoc("core/autonomy/index.mdx");
    const config = readRuntimeDoc("core/configuration/config-toml.mdx");

    expectIncludesAll(profiles, [
      "[task.orchestration.profile]",
      "[task.orchestration.review]",
      "Selector precedence and runtime selection",
      "ClaimNextRun",
      "Session start (load only)",
      "Sandbox mode behavior",
      "Worker runtime selection",
      "config.ResolveSessionAgentWithRuntime",
      "Manage a profile from the CLI",
      "agh task profile inspect",
      "agh task profile update",
      "agh task profile delete",
      "Manage a profile through HTTP and UDS",
      "/api/tasks/{id}/execution-profile",
      "Update is a full replace, not a patch",
      "Native tools for in-session agents",
      "task_execution_profile_get",
      "task_execution_profile_set",
      "task_execution_profile_delete",
      "agh__task_execution_profile_get",
      "agh__task_execution_profile_set",
      "agh__task_execution_profile_delete",
      "Inspect and edit from the operator web UI",
      "Task setup",
      "View JSON",
      "form-first",
      "Config lifecycle",
      "agh config set",
      "agh__config_*",
      "Authority boundary",
    ]);
    expect(overview).toContain("/runtime/core/autonomy/execution-profiles");
    expectIncludesAll(config, [
      "[task.orchestration]",
      "[task.orchestration.profile]",
      "[task.orchestration.review]",
      "default_coordinator_mode",
      "default_worker_mode",
      "default_sandbox_mode",
      "allow_task_provider_override",
      "allow_task_sandbox_none",
      "max_active_runs_per_workspace",
      'lifecycle="restart-required"',
      "Profile validation runs in `task.Service` when a profile is created or updated",
      "Workspace overlays may tighten or relax",
      "agh config set",
      "agh__config_*",
    ]);
  });
});

describe("runtime review-gate docs", () => {
  it("documents the post-terminal review gate, reviewer routing, and continuation runs", () => {
    const reviewGate = readRuntimeDoc("core/autonomy/review-gate.mdx");
    const overview = readRuntimeDoc("core/autonomy/index.mdx");
    const config = readRuntimeDoc("core/configuration/config-toml.mdx");

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
      "agh task review request",
      "agh task review list",
      "agh task review show",
      "agh task review submit",
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
      "agh__task_run_review_submit",
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
      "AGH skill expectations",
      "Config lifecycle",
      "[task.orchestration.review]",
    ]);
    expectExcludesAll(reviewGate, [
      "/runtime/core/agent/context",
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
    expect(overview).toContain("/runtime/core/autonomy/review-gate");
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
  it("documents cursor identity, lifecycle, terminal notifier states, and SSE resume", () => {
    const notifications = readRuntimeDoc("core/autonomy/notification-cursors.mdx");
    const overview = readRuntimeDoc("core/autonomy/index.mdx");

    expectIncludesAll(notifications, [
      "internal/notifications",
      "delivery progress",
      "never owns task ownership",
      "Cursor identity and store",
      "notification_cursors",
      "consumer_id",
      "stream_name",
      "subject_id",
      "last_sequence",
      "last_delivery_id",
      "last_delivered_at",
      "last_error",
      "`last_delivered_at` |",
      "ErrNonMonotonicCursor",
      "only service/store path that lowers a cursor",
      "No task notification CLI/API reset verb is exposed today",
      "at-least-once",
      "Bridge task subscription lifecycle",
      "bridge_task_subscriptions",
      "zero-sequence",
      "Resubscribe",
      "Terminal notifier states",
      "deliver",
      "defer",
      "mismatch",
      "task.run_completed",
      "task.run_failed",
      "task.run_canceled",
      "task.run_review_approved",
      "task.canceled",
      "fail-closed",
      "notification.terminal_state_mismatch",
      "Bridge notification envelope",
      "delivery_id",
      "Manage subscriptions from the CLI",
      "agh task notification subscribe",
      "agh task notification list",
      "agh task notification show",
      "agh task notification delete",
      "Manage subscriptions with native tools",
      "agh__task_notification_subscribe",
      "agh__task_notification_list",
      "agh__task_notification_show",
      "agh__task_notification_delete",
      "Manage subscriptions through HTTP and UDS",
      "/api/tasks/{id}/notifications/bridges",
      "/api/tasks/{id}/notifications/bridges/{subscription_id}",
      "createTaskBridgeNotificationSubscription",
      "listTaskBridgeNotificationSubscriptions",
      "getTaskBridgeNotificationSubscription",
      "deleteTaskBridgeNotificationSubscription",
      "Inspect from the operator web UI",
      "Bridges",
      "requires confirmation",
      "Inspect → Stream",
      "SSE resume seeding",
      "latest_event_seq",
      "Last-Event-ID",
      "Authority boundary",
      "accepted-final",
    ]);
    expectExcludesAll(notifications, [
      "matching run-detail context render the same cursor diagnostics",
      'shows "No delivery yet"',
      "route it through the existing operator surface",
      "bound delivery diagnostics and event payload sizes",
    ]);
    expect(overview).toContain("/runtime/core/autonomy/notification-cursors");
  });
});

describe("bundled AGH skill docs", () => {
  it("describes the AGH skill as instructional only and lists contextual references", () => {
    const bundled = readRuntimeDoc("core/skills/bundled.mdx");

    expectIncludesAll(bundled, [
      "`agh`",
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
    "cli-reference/task/review/index.mdx",
    "cli-reference/task/review/request.mdx",
    "cli-reference/task/review/list.mdx",
    "cli-reference/task/review/show.mdx",
    "cli-reference/task/review/submit.mdx",
  ];

  it("keeps regenerated CLI reference pages present for the review command group", () => {
    for (const page of requiredReviewPages) {
      expect(existsSync(resolve(runtimeRoot, page))).toBe(true);
    }
  });

  it("documents review CLI flags exactly once on each generated page", () => {
    const request = readRuntimeDoc("cli-reference/task/review/request.mdx");
    const list = readRuntimeDoc("cli-reference/task/review/list.mdx");
    const show = readRuntimeDoc("cli-reference/task/review/show.mdx");
    const submit = readRuntimeDoc("cli-reference/task/review/submit.mdx");

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

describe("generated task notification CLI references", () => {
  const requiredNotificationPages = [
    "cli-reference/task/notification/index.mdx",
    "cli-reference/task/notification/subscribe.mdx",
    "cli-reference/task/notification/list.mdx",
    "cli-reference/task/notification/show.mdx",
    "cli-reference/task/notification/delete.mdx",
  ];

  it("keeps regenerated CLI reference pages present for the notification command group", () => {
    for (const page of requiredNotificationPages) {
      expect(existsSync(resolve(runtimeRoot, page))).toBe(true);
    }
  });

  it("documents notification CLI flags on the generated pages", () => {
    const subscribe = readRuntimeDoc("cli-reference/task/notification/subscribe.mdx");
    const list = readRuntimeDoc("cli-reference/task/notification/list.mdx");

    expectIncludesAll(subscribe, [
      "--bridge",
      "--peer",
      "--thread",
      "--group",
      "--scope",
      "--workspace",
      "--mode",
      "--subscription-id",
    ]);
    expectIncludesAll(list, ["--bridge", "--scope", "--workspace", "--last"]);
  });
});

describe("generated task execution profile CLI references", () => {
  const requiredProfilePages = [
    "cli-reference/task/profile/index.mdx",
    "cli-reference/task/profile/inspect.mdx",
    "cli-reference/task/profile/update.mdx",
    "cli-reference/task/profile/delete.mdx",
  ];

  it("keeps regenerated CLI reference pages present for the profile command group", () => {
    for (const page of requiredProfilePages) {
      expect(existsSync(resolve(runtimeRoot, page))).toBe(true);
    }
  });

  it("documents the profile update --profile JSON flag on the generated CLI page", () => {
    const update = readRuntimeDoc("cli-reference/task/profile/update.mdx");
    const inspect = readRuntimeDoc("cli-reference/task/profile/inspect.mdx");
    const del = readRuntimeDoc("cli-reference/task/profile/delete.mdx");

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
    "cli-reference/me/index.mdx",
    "cli-reference/me/context.mdx",
    "cli-reference/ch/index.mdx",
    "cli-reference/ch/list.mdx",
    "cli-reference/ch/recv.mdx",
    "cli-reference/ch/send.mdx",
    "cli-reference/ch/reply.mdx",
    "cli-reference/spawn.mdx",
    "cli-reference/task/next.mdx",
    "cli-reference/task/heartbeat.mdx",
    "cli-reference/task/complete.mdx",
    "cli-reference/task/fail.mdx",
    "cli-reference/task/release.mdx",
    "cli-reference/task/retry.mdx",
  ];

  it("keeps regenerated command pages present for agent-facing autonomy commands", () => {
    for (const page of requiredPages) {
      expect(existsSync(resolve(runtimeRoot, page))).toBe(true);
    }
  });

  it("lists exact implemented flags for task, channel, and spawn examples", () => {
    const taskNext = readRuntimeDoc("cli-reference/task/next.mdx");
    const heartbeat = readRuntimeDoc("cli-reference/task/heartbeat.mdx");
    const complete = readRuntimeDoc("cli-reference/task/complete.mdx");
    const fail = readRuntimeDoc("cli-reference/task/fail.mdx");
    const release = readRuntimeDoc("cli-reference/task/release.mdx");
    const retry = readRuntimeDoc("cli-reference/task/retry.mdx");
    const send = readRuntimeDoc("cli-reference/ch/send.mdx");
    const reply = readRuntimeDoc("cli-reference/ch/reply.mdx");
    const spawn = readRuntimeDoc("cli-reference/spawn.mdx");

    expectIncludesAll(taskNext, ["--wait", "--lease-seconds", "--capability", "--priority-min"]);
    expectIncludesAll(heartbeat, ["--lease-seconds"]);
    expectIncludesAll(complete, ["--result"]);
    expectIncludesAll(fail, ["--reason", "--metadata"]);
    expectIncludesAll(release, ["--reason"]);
    for (const content of [heartbeat, complete, fail, release, retry]) {
      expect(content).not.toContain("--claim-token");
    }
    expectIncludesAll(send, [
      "--body",
      "--task-id",
      "--run-id",
      "--kind",
      "--correlation-id",
      "--channel-id",
    ]);
    expectIncludesAll(reply, ["--to-message", "--body", "--task-id", "--run-id", "--channel-id"]);
    expectExcludesAll(send, ["--coordination-channel-id"]);
    expectExcludesAll(reply, ["--coordination-channel-id"]);
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
      "--channel",
      "--sandbox-profile",
    ]);
  });
});
