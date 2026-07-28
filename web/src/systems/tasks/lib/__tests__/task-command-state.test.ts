import { describe, expect, it } from "vitest";

import {
  buildDetailFixture,
  buildTaskRunRecordFixture,
  buildTaskRunFixture,
} from "../../mocks/fixtures";
import {
  resolveTaskCommandState,
  taskHighestAttemptOrdinal,
  type TaskPrimaryCommand,
} from "../task-command-state";
import type { TaskDetailView, TaskRun } from "../../types";

interface CommandCase {
  detail: TaskDetailView;
  expected: TaskPrimaryCommand | null;
  name: string;
  runs?: TaskRun[];
}

const COMMAND_CASES: CommandCase[] = [
  {
    detail: buildDetailFixture({
      task: { draft: true, status: "draft" },
      summary: { active_run: null, status: "draft" },
    } as never),
    expected: { kind: "publish" },
    name: "publish a draft",
  },
  {
    detail: buildDetailFixture({
      task: { approval_state: "pending", status: "pending" },
      summary: { active_run: null, status: "pending" },
    } as never),
    expected: { kind: "approve" },
    name: "approve a gated task",
  },
  {
    detail: buildDetailFixture({
      task: { paused: true, status: "ready" },
      summary: { active_run: null, status: "ready" },
    } as never),
    expected: { kind: "resume" },
    name: "resume a paused task",
  },
  {
    detail: buildDetailFixture({
      task: { status: "in_progress" },
      summary: {
        active_run: buildTaskRunFixture({ id: "run_open", status: "running" }),
        status: "in_progress",
      },
    } as never),
    expected: { kind: "open_run", runId: "run_open" },
    name: "open an active run",
  },
  {
    detail: buildDetailFixture({
      task: { max_attempts: 2, status: "needs_attention" },
      summary: {
        active_run: buildTaskRunFixture({
          attempt: 1,
          id: "run_attention",
          status: "needs_attention",
        }),
        status: "needs_attention",
      },
    } as never),
    expected: { kind: "recover" },
    name: "recover a needs-attention run",
  },
  {
    detail: buildDetailFixture({
      task: { max_attempts: 2, status: "needs_attention" },
      summary: {
        active_run: buildTaskRunFixture({
          attempt: 2,
          id: "run_exhausted",
          status: "needs_attention",
        }),
        status: "needs_attention",
      },
    } as never),
    expected: null,
    name: "withhold recovery after attempt exhaustion",
  },
  {
    detail: buildDetailFixture({
      task: { max_attempts: 2, status: "failed" },
      summary: { active_run: null, status: "failed" },
    } as never),
    expected: { kind: "retry", runId: "run_failed" },
    name: "retry a failed run with attempts remaining",
    runs: [buildTaskRunRecordFixture({ id: "run_failed", attempt: 1, status: "failed" })],
  },
  {
    detail: buildDetailFixture({
      task: { status: "ready" },
      summary: { active_run: null, status: "ready" },
    } as never),
    expected: { kind: "start" },
    name: "start ready work",
  },
  {
    detail: buildDetailFixture({
      task: { status: "blocked" },
      summary: { active_run: null, status: "blocked" },
    } as never),
    expected: null,
    name: "withhold a primary command while blocked",
  },
  {
    detail: buildDetailFixture({
      task: { status: "completed" },
      summary: { active_run: null, status: "completed" },
    } as never),
    expected: null,
    name: "withhold a primary command after completion",
  },
];

describe("resolveTaskCommandState", () => {
  it.each(COMMAND_CASES)("Should $name", ({ detail, runs = [], expected }) => {
    expect(resolveTaskCommandState(detail, runs).primary).toEqual(expected);
  });

  it("Should expose matching secondary and fan-out policies", () => {
    const approval = resolveTaskCommandState(COMMAND_CASES[1]!.detail, []);
    const active = resolveTaskCommandState(COMMAND_CASES[3]!.detail, []);

    expect(approval.secondary).toMatchObject({ reject: true });
    expect(approval.canFanOut).toBe(false);
    expect(active.secondary).toMatchObject({ pause: true });
    expect(active.canFanOut).toBe(false);
  });

  it("Should return the maximum attempt ordinal across active and historical runs", () => {
    expect(
      taskHighestAttemptOrdinal(
        [buildTaskRunRecordFixture({ attempt: 1 }), buildTaskRunRecordFixture({ attempt: 4 })],
        { attempt: 3 }
      )
    ).toBe(4);
  });
});
