import { describe, expect, it } from "vitest";

import {
  RUN_STATUS_TONE,
  TASK_LANE_TONE,
  TASK_STATUS_TONE,
  type TaskLane,
  type TaskRunStatus,
  type TaskStatus,
} from "../status-tone";

const EXPECTED_TASK_STATUS_KEYS: readonly TaskStatus[] = [
  "draft",
  "pending",
  "blocked",
  "needs_attention",
  "ready",
  "in_progress",
  "completed",
  "failed",
];

const EXPECTED_RUN_STATUS_KEYS: readonly TaskRunStatus[] = [
  "pending",
  "in_progress",
  "needs_attention",
  "completed",
  "failed",
  "canceled",
];

const EXPECTED_LANE_KEYS: readonly TaskLane[] = [
  "active",
  "blocked",
  "recent",
  "my_work",
  "mentions",
  "failed_runs",
  "updates",
  "approvals",
];

describe("TASK_STATUS_TONE", () => {
  it("Should expose exactly the eight techspec-scoped task statuses", () => {
    expect(Object.keys(TASK_STATUS_TONE).sort()).toEqual([...EXPECTED_TASK_STATUS_KEYS].sort());
  });

  it("Should not include the deferred 'stuck' UI tone", () => {
    expect(Object.keys(TASK_STATUS_TONE)).not.toContain("stuck");
  });

  it("Should not include 'queued' (not a backend Status value)", () => {
    expect(Object.keys(TASK_STATUS_TONE)).not.toContain("queued");
  });

  it("Should map blocked + failed to danger and completed to success", () => {
    expect(TASK_STATUS_TONE.blocked).toBe("danger");
    expect(TASK_STATUS_TONE.failed).toBe("danger");
    expect(TASK_STATUS_TONE.completed).toBe("success");
  });

  it("Should map in_progress to info and the resting states to neutral", () => {
    expect(TASK_STATUS_TONE.in_progress).toBe("info");
    expect(TASK_STATUS_TONE.draft).toBe("neutral");
    expect(TASK_STATUS_TONE.pending).toBe("neutral");
    expect(TASK_STATUS_TONE.ready).toBe("neutral");
  });

  it("Should map needs_attention to warning", () => {
    expect(TASK_STATUS_TONE.needs_attention).toBe("warning");
  });

  it("Should map needs_attention and blocked to distinct tokens (no coercion)", () => {
    // Truthful UI: the unblock-loop breaker's escalation must not read as the
    // same signal as a plain block. blocked → danger, needs_attention → warning.
    expect(TASK_STATUS_TONE.needs_attention).not.toBe(TASK_STATUS_TONE.blocked);
    expect(TASK_STATUS_TONE.blocked).toBe("danger");
    expect(TASK_STATUS_TONE.needs_attention).toBe("warning");
  });
});

describe("RUN_STATUS_TONE", () => {
  it("Should expose the six run lifecycle states with single-L Go convention", () => {
    expect(Object.keys(RUN_STATUS_TONE).sort()).toEqual([...EXPECTED_RUN_STATUS_KEYS].sort());
    expect(RUN_STATUS_TONE).toHaveProperty("canceled");
    expect(RUN_STATUS_TONE).not.toHaveProperty("cancelled");
  });

  it("Should map canceled to neutral (no destructive tone for operator-initiated stop)", () => {
    expect(RUN_STATUS_TONE.canceled).toBe("neutral");
  });

  it("Should map failed to danger and completed to success", () => {
    expect(RUN_STATUS_TONE.failed).toBe("danger");
    expect(RUN_STATUS_TONE.completed).toBe("success");
    expect(RUN_STATUS_TONE.in_progress).toBe("info");
    expect(RUN_STATUS_TONE.pending).toBe("neutral");
  });

  it("Should map needs_attention to the warning signal (operator action required)", () => {
    expect(RUN_STATUS_TONE.needs_attention).toBe("warning");
  });
});

describe("TASK_LANE_TONE", () => {
  it("Should expose the eight UI lane keys", () => {
    expect(Object.keys(TASK_LANE_TONE).sort()).toEqual([...EXPECTED_LANE_KEYS].sort());
  });

  it("Should map approvals to info", () => {
    expect(TASK_LANE_TONE.approvals).toBe("info");
  });

  it("Should map mentions to accent", () => {
    expect(TASK_LANE_TONE.mentions).toBe("accent");
  });

  it("Should map blocked + failed_runs to danger", () => {
    expect(TASK_LANE_TONE.blocked).toBe("danger");
    expect(TASK_LANE_TONE.failed_runs).toBe("danger");
  });

  it("Should leave active/recent/my_work/updates as neutral", () => {
    expect(TASK_LANE_TONE.active).toBe("neutral");
    expect(TASK_LANE_TONE.recent).toBe("neutral");
    expect(TASK_LANE_TONE.my_work).toBe("neutral");
    expect(TASK_LANE_TONE.updates).toBe("neutral");
  });
});
