import { describe, expect, it } from "vitest";

import { buildLiveNetworkParticipationFixture } from "@/test/network-participation-fixtures";

import type { TaskBlockedReason, TaskListItem, TaskRecord, TaskRun } from "../../types";
import { buildTaskRunRecordFixture } from "../../mocks/fixtures";
import {
  countTasksByStatus,
  formatAttemptLabel,
  formatDurationMs,
  formatPercent,
  formatRelativeTime,
  matchesTaskQuery,
  projectBlockedReasonChips,
  runCoordinationChannelLabel,
  runIsCoordinated,
  taskApprovalStateLabel,
  taskCanRecover,
  taskHandoffActionCopy,
  taskHasApprovalPending,
  taskInboxLaneLabel,
  taskIsBlocked,
  taskIsDraft,
  taskLaneTone,
  taskLifecyclePhase,
  taskLifecyclePhaseDescription,
  taskLifecyclePhaseLabel,
  taskLifecyclePhaseTone,
  taskOwnerKindLabel,
  taskOwnerLabel,
  taskPriorityLabel,
  taskPriorityTone,
  taskRunStatusLabel,
  taskRunStatusTone,
  taskStatusLabel,
  taskStatusSignal,
  taskStatusTone,
  taskWakeIndicatorApplies,
} from "../task-formatters";
import { taskRunCanRecover } from "../task-run-recovery";

function makeTask(overrides: Partial<TaskListItem> = {}): TaskListItem {
  return {
    id: "task_001",
    title: "Review",
    status: "ready",
    scope: "workspace",
    origin: { kind: "web", ref: "op" },
    created_at: "2026-04-11T09:00:00Z",
    updated_at: "2026-04-11T09:00:00Z",
    created_by: { kind: "human", ref: "op" },
    ...overrides,
  } as TaskListItem;
}

describe("task status and priority labels", () => {
  it("labels every documented task status", () => {
    expect(taskStatusLabel("draft")).toBe("Draft");
    expect(taskStatusLabel("pending")).toBe("Not started");
    expect(taskStatusLabel("blocked")).toBe("Blocked");
    expect(taskStatusLabel("ready")).toBe("Ready");
    expect(taskStatusLabel("in_progress")).toBe("In progress");
    expect(taskStatusLabel("completed")).toBe("Completed");
    expect(taskStatusLabel("failed")).toBe("Failed");
    expect(taskStatusLabel("canceled")).toBe("Canceled");
    expect(taskStatusLabel("needs_attention")).toBe("Needs attention");
    expect(taskStatusLabel(null)).toBe("Unknown");
  });

  it("Should label every changed task run status", () => {
    expect(taskRunStatusLabel("claimed")).toBe("Assigned");
    expect(taskRunStatusLabel("needs_attention")).toBe("Needs attention");
  });

  it("labels priorities", () => {
    expect(taskPriorityLabel("low")).toBe("Low");
    expect(taskPriorityLabel("urgent")).toBe("Urgent");
    expect(taskPriorityLabel(null)).toBe("Unset");
  });

  it("labels inbox lanes", () => {
    expect(taskInboxLaneLabel("my_work")).toBe("My work");
    expect(taskInboxLaneLabel("approvals")).toBe("Approvals");
    expect(taskInboxLaneLabel("failed_runs")).toBe("Failed runs");
    expect(taskInboxLaneLabel("blocked")).toBe("Blocked");
    expect(taskInboxLaneLabel("archived")).toBe("Archived");
  });

  it("labels approval states", () => {
    expect(taskApprovalStateLabel("pending")).toBe("Approval pending");
    expect(taskApprovalStateLabel("approved")).toBe("Approved");
    expect(taskApprovalStateLabel(undefined)).toBe("Not required");
  });
});

describe("task run recovery eligibility", () => {
  it.each([
    { attempt: 1, expected: true, maxAttempts: 2 },
    { attempt: 1, expected: false, maxAttempts: 1 },
  ])(
    "returns $expected for needs_attention attempt $attempt of $maxAttempts",
    ({ attempt, expected, maxAttempts }) => {
      expect(
        taskRunCanRecover(
          {
            attempt,
            status: "needs_attention",
          },
          maxAttempts
        )
      ).toBe(expected);
    }
  );
});

describe("task semantic tones", () => {
  it("Should resolve task statuses through TASK_STATUS_TONE PillTone dictionary", () => {
    expect(taskStatusTone("completed")).toBe("success");
    expect(taskStatusTone("failed")).toBe("danger");
    expect(taskStatusTone("canceled")).toBe("neutral");
    expect(taskStatusTone("in_progress")).toBe("info");
    expect(taskStatusTone("blocked")).toBe("danger");
    expect(taskStatusTone("ready")).toBe("neutral");
    expect(taskStatusTone("draft")).toBe("neutral");
    expect(taskStatusTone("pending")).toBe("neutral");
    expect(taskStatusTone(undefined)).toBe("neutral");
    expect(taskStatusTone(null)).toBe("neutral");
  });

  it("Should always resolve priority to neutral — priority never colorizes", () => {
    expect(taskPriorityTone("urgent")).toBe("neutral");
    expect(taskPriorityTone("high")).toBe("neutral");
    expect(taskPriorityTone("medium")).toBe("neutral");
    expect(taskPriorityTone("low")).toBe("neutral");
    expect(taskPriorityTone(undefined)).toBe("neutral");
  });

  it("Should resolve run statuses through RUN_STATUS_TONE PillTone dictionary", () => {
    expect(taskRunStatusTone("running")).toBe("info");
    expect(taskRunStatusTone("completed")).toBe("success");
    expect(taskRunStatusTone("failed")).toBe("danger");
    expect(taskRunStatusTone("canceled")).toBe("neutral");
    expect(taskRunStatusTone("queued")).toBe("neutral");
    expect(taskRunStatusTone("starting")).toBe("info");
    expect(taskRunStatusTone("claimed")).toBe("info");
    expect(taskRunStatusTone("needs_attention")).toBe("warning");
    expect(taskRunStatusTone(null)).toBe("neutral");
  });

  it("Should resolve inbox lanes through TASK_LANE_TONE — approvals collapses to info", () => {
    expect(taskLaneTone("approvals")).toBe("info");
    expect(taskLaneTone("failed_runs")).toBe("danger");
    expect(taskLaneTone("blocked")).toBe("danger");
    expect(taskLaneTone("archived")).toBe("neutral");
    expect(taskLaneTone("my_work")).toBe("neutral");
  });
});

describe("task predicates and counts", () => {
  it("detects draft, blocked, and approval-pending tasks", () => {
    expect(taskIsDraft(makeTask({ draft: true }))).toBe(true);
    expect(taskIsDraft(makeTask({ status: "draft" }))).toBe(true);
    expect(taskIsDraft(makeTask())).toBe(false);
    expect(taskIsBlocked(makeTask({ status: "blocked" }))).toBe(true);
    expect(taskIsBlocked(makeTask())).toBe(false);
    expect(taskHasApprovalPending(makeTask({ approval_state: "pending" }))).toBe(true);
    expect(taskHasApprovalPending(makeTask({ approval_state: "approved" }))).toBe(false);
  });

  it("matches queries by title and identifier", () => {
    const task = makeTask({ title: "Review PR", identifier: "TASK-42" });

    expect(matchesTaskQuery(task, "")).toBe(true);
    expect(matchesTaskQuery(task, "review")).toBe(true);
    expect(matchesTaskQuery(task, "TASK-42")).toBe(true);
    expect(matchesTaskQuery(task, "missing")).toBe(false);
  });

  it("formats owner labels with kind fallbacks", () => {
    expect(taskOwnerKindLabel("agent_session")).toBe("Agent");
    expect(taskOwnerKindLabel("network_peer")).toBe("Peer");
    expect(taskOwnerKindLabel(null)).toBe("Unassigned");
    expect(taskOwnerLabel(null)).toBe("Unassigned");
    expect(taskOwnerLabel({ kind: "agent_session", ref: "Coder" })).toBe("Coder");
    expect(taskOwnerLabel({ kind: "agent_session", ref: "" })).toBe("Agent");
  });

  it("formats relative time and attempt labels", () => {
    const now = new Date("2026-04-11T10:00:00Z");
    expect(formatRelativeTime("2026-04-11T09:59:30Z", now)).toBe("now");
    expect(formatRelativeTime("2026-04-11T09:30:00Z", now)).toBe("30m");
    expect(formatRelativeTime("2026-04-11T08:00:00Z", now)).toBe("2h");
    expect(formatRelativeTime("2026-04-09T10:00:00Z", now)).toBe("2d");
    expect(formatRelativeTime(null)).toBe("—");

    expect(formatAttemptLabel(2, 3)).toBe("attempt 2 of 3");
    expect(formatAttemptLabel(1)).toBe("attempt 1");
    expect(formatAttemptLabel(null)).toBeNull();
  });

  it("formats durations and percentages for dashboard metrics", () => {
    expect(formatDurationMs(0)).toBe("0ms");
    expect(formatDurationMs(450)).toBe("450ms");
    expect(formatDurationMs(12_000)).toBe("12s");
    expect(formatDurationMs(167_000)).toBe("2m 47s");
    expect(formatDurationMs(3_600_000)).toBe("1h");
    expect(formatDurationMs(3_900_000)).toBe("1h 5m");
    expect(formatDurationMs(null)).toBe("—");
    expect(formatDurationMs(-10)).toBe("—");

    expect(formatPercent(43)).toBe("43%");
    expect(formatPercent(100)).toBe("100%");
    expect(formatPercent(120)).toBe("100%");
    expect(formatPercent(-5)).toBe("0%");
    expect(formatPercent(null)).toBe("—");
  });

  it("counts tasks by status", () => {
    const counts = countTasksByStatus([
      makeTask({ status: "ready" }),
      makeTask({ status: "ready" }),
      makeTask({ status: "failed" }),
    ]);

    expect(counts.ready).toBe(2);
    expect(counts.failed).toBe(1);
    expect(counts.draft).toBe(0);
  });
});

describe("task lifecycle phases — manual-first signaling", () => {
  it("treats draft tasks without runs as saved intent, not executable", () => {
    const phase = taskLifecyclePhase(makeTask({ status: "draft", draft: true, active_run: null }));
    expect(phase).toBe("saved_intent");
    expect(taskLifecyclePhaseLabel(phase)).toBe("Saved intent");
    expect(taskLifecyclePhaseDescription(phase)).toMatch(/saved intent/i);
    expect(taskLifecyclePhaseDescription(phase)).toMatch(/coordinator/i);
  });

  it("treats ready tasks without runs as ready_to_start, not running", () => {
    const phase = taskLifecyclePhase(makeTask({ status: "ready", active_run: null }));
    expect(phase).toBe("ready_to_start");
    expect(taskLifecyclePhaseDescription(phase)).toMatch(/start enqueues/i);
  });

  it("uses the active run to tell queued from running", () => {
    const queued = taskLifecyclePhase(
      makeTask({
        status: "in_progress",
        active_run: makeRun("queued"),
      } as Partial<TaskListItem>)
    );
    const running = taskLifecyclePhase(
      makeTask({
        status: "in_progress",
        active_run: makeRun("running"),
      } as Partial<TaskListItem>)
    );

    expect(queued).toBe("queued");
    expect(running).toBe("running");
    expect(taskLifecyclePhaseLabel(queued)).toBe("Coordinator handoff");
    expect(taskLifecyclePhaseLabel(running)).toBe("Running");
  });

  it("treats agent-created approval-pending tasks as awaiting approval", () => {
    const phase = taskLifecyclePhase(
      makeTask({
        status: "blocked",
        approval_policy: "manual",
        approval_state: "pending",
        active_run: null,
      })
    );

    expect(phase).toBe("awaiting_approval");
    expect(taskLifecyclePhaseDescription(phase)).toMatch(/approving enqueues/i);
  });

  it("falls back to terminal phases without inferring activity from status", () => {
    expect(taskLifecyclePhase(makeTask({ status: "completed", active_run: null }))).toBe(
      "completed"
    );
    expect(taskLifecyclePhase(makeTask({ status: "failed", active_run: null }))).toBe("failed");
    expect(taskLifecyclePhase(makeTask({ status: "canceled", active_run: null }))).toBe("canceled");
    expect(taskLifecyclePhase(makeTask({ status: "blocked", active_run: null }))).toBe("blocked");
  });

  it("Should treat needs_attention as recovery-only, not ready to start", () => {
    const phase = taskLifecyclePhase(makeTask({ status: "needs_attention", active_run: null }));

    expect(phase).toBe("needs_attention");
    expect(taskLifecyclePhaseLabel(phase)).toBe("Needs attention");
    expect(taskLifecyclePhaseDescription(phase)).toMatch(/recover it before/i);
    expect(taskLifecyclePhaseDescription(phase)).toMatch(/enqueued or claimed/i);
  });

  it("Should project an active needs_attention run as recovery-only when the task is ready", () => {
    const phase = taskLifecyclePhase(
      makeTask({
        status: "ready",
        active_run: makeRun("needs_attention"),
      } as Partial<TaskListItem>)
    );

    expect(phase).toBe("needs_attention");
  });

  it("Should never mark saved intent or ready as activity in lifecycle tones", () => {
    expect(taskLifecyclePhaseTone("saved_intent")).toBe("neutral");
    expect(taskLifecyclePhaseTone("ready_to_start")).toBe("neutral");
    expect(taskLifecyclePhaseTone("queued")).toBe("neutral");
    expect(taskLifecyclePhaseTone("running")).toBe("accent");
    expect(taskLifecyclePhaseTone("awaiting_approval")).toBe("info");
    expect(taskLifecyclePhaseTone("needs_attention")).toBe("warning");
    expect(taskLifecyclePhaseTone("blocked")).toBe("danger");
    expect(taskLifecyclePhaseTone("failed")).toBe("danger");
    expect(taskLifecyclePhaseTone("canceled")).toBe("danger");
    expect(taskLifecyclePhaseTone("completed")).toBe("neutral");
  });
});

describe("task handoff actions — boundary semantics", () => {
  it("Should expose copy only for the explicit actions rendered by callers", () => {
    expect(taskHandoffActionCopy("publish").label).toBe("Publish");
    expect(taskHandoffActionCopy("publish").tooltip).toMatch(/coordinator handoff/i);
    expect(taskHandoffActionCopy("start").label).toBe("Start run");
    expect(taskHandoffActionCopy("start").tooltip).toMatch(/coordinator handoff/i);
  });
});

describe("coordination channel signal", () => {
  it("recognises runs with live resolved participation as coordinated", () => {
    const run = buildTaskRunRecordFixture({
      coordination_channel: null,
      resolved_network_participation: buildLiveNetworkParticipationFixture({
        workspaceId: "ws_storybook",
        channelId: "coord-task-001",
      }),
    });

    expect(runIsCoordinated(run)).toBe(true);
    expect(runCoordinationChannelLabel(run)).toBe("coord-task-001");
  });

  it("prefers the embedded display name when available", () => {
    const run = buildTaskRunRecordFixture({
      resolved_network_participation: buildLiveNetworkParticipationFixture({
        workspaceId: "ws_storybook",
        channelId: "coord-task-001",
      }),
      coordination_channel: {
        id: "coord-task-001",
        display_name: "TASK-1 coordination",
        allowed_message_kinds: ["status"],
      },
    });

    expect(runIsCoordinated(run)).toBe(true);
    expect(runCoordinationChannelLabel(run)).toBe("TASK-1 coordination");
  });

  it("ignores runs without channel binding", () => {
    expect(runIsCoordinated(null)).toBe(false);
    expect(runIsCoordinated({} as TaskRun)).toBe(false);
    expect(runCoordinationChannelLabel(null)).toBe("");
  });
});

function makeRun(status: TaskRun["status"]): TaskListItem["active_run"] {
  return {
    id: "run_test",
    task_id: "task_test",
    attempt: 1,
    status,
    queued_at: "2026-04-17T09:58:00Z",
  } as TaskListItem["active_run"];
}

describe("blocked-reasons chip projection", () => {
  it("Should return an empty array when the task carries no blocking causes", () => {
    expect(projectBlockedReasonChips(undefined)).toEqual([]);
    expect(projectBlockedReasonChips(null)).toEqual([]);
    expect(projectBlockedReasonChips([])).toEqual([]);
  });

  it("Should project exactly one chip per blocked_reasons entry, preserving order", () => {
    const reasons: TaskBlockedReason[] = [
      { source: "dependency", depends_on_task_ids: ["task_dep_a", "task_dep_b"] },
      { source: "approval", reason: "Pending review" },
      { source: "block", kind: "transient", reason: "External API down", block_id: "block_1" },
    ];

    const chips = projectBlockedReasonChips(reasons);

    expect(chips).toHaveLength(reasons.length);
    expect(chips.map(chip => chip.source)).toEqual(["dependency", "approval", "block"]);
  });

  it("Should carry the source label, block kind, and reason on the block chip", () => {
    const chips = projectBlockedReasonChips([
      { source: "block", kind: "capability", reason: "Missing browser skill", block_id: "block_9" },
    ]);

    expect(chips[0]).toMatchObject({
      key: "block_9",
      source: "block",
      sourceLabel: "Block",
      kind: "capability",
      kindLabel: "Capability",
      reason: "Missing browser skill",
      tone: "warning",
    });
  });

  it("Should surface dependency ids and omit blank reasons", () => {
    const chips = projectBlockedReasonChips([
      { source: "dependency", reason: "   ", depends_on_task_ids: ["task_x", "  "] },
    ]);

    expect(chips[0].reason).toBeUndefined();
    expect(chips[0].dependsOnTaskIds).toEqual(["task_x"]);
    expect(chips[0].tone).toBe("neutral");
  });

  it("Should fall back to a source+index key when block_id is absent", () => {
    const chips = projectBlockedReasonChips([
      { source: "paused", reason: "Operator hold" },
      { source: "paused", reason: "Ancestor hold" },
    ]);

    expect(chips.map(chip => chip.key)).toEqual(["paused-0", "paused-1"]);
  });
});

describe("recover + wake affordance gating", () => {
  it("Should enable recover only when the derived status is needs_attention", () => {
    expect(taskCanRecover({ status: "needs_attention" } as Pick<TaskRecord, "status">)).toBe(true);
    expect(taskCanRecover({ status: "ready" } as Pick<TaskRecord, "status">)).toBe(false);
    expect(taskCanRecover({ status: "blocked" } as Pick<TaskRecord, "status">)).toBe(false);
  });

  it("Should not offer recover on a terminal task even if needs_attention_at lingered", () => {
    // Derived precedence is terminal > needs_attention (ADR-003): a canceled task
    // must not present a Recover control the runtime would reject.
    expect(taskCanRecover({ status: "canceled" } as Pick<TaskRecord, "status">)).toBe(false);
    expect(taskCanRecover({ status: "completed" } as Pick<TaskRecord, "status">)).toBe(false);
  });

  it("Should render blocked and needs_attention as distinct StatusDot signals", () => {
    // The header title dot must agree with the status pill and keep the two
    // states distinct (no coercion): blocked → danger, needs_attention → warning.
    expect(taskStatusSignal("blocked").tone).toBe("danger");
    expect(taskStatusSignal("needs_attention").tone).toBe("warning");
    expect(taskStatusSignal("blocked").tone).not.toBe(taskStatusSignal("needs_attention").tone);
  });

  it("Should surface the wake indicator only for agent-session-created tasks", () => {
    expect(
      taskWakeIndicatorApplies({
        created_by: { kind: "agent_session", ref: "session_a" },
      } as Pick<TaskRecord, "created_by">)
    ).toBe(true);
    expect(
      taskWakeIndicatorApplies({
        created_by: { kind: "human", ref: "operator" },
      } as Pick<TaskRecord, "created_by">)
    ).toBe(false);
  });
});
