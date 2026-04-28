import type { Run, RunSnapshot, RunTranscript } from "../types";
import { workspaceFixture } from "@/systems/app-shell/mocks";
import { workflowAlphaFixture } from "@/systems/workflows/mocks";

export const runsFixture: Run[] = [
  {
    run_id: "run-task-02",
    mode: "task",
    presentation_mode: "text",
    workspace_id: workspaceFixture.id,
    started_at: "2026-04-20T03:10:00Z",
    status: "running",
    workflow_slug: workflowAlphaFixture.slug,
  },
  {
    run_id: "run-review-1",
    mode: "review",
    presentation_mode: "text",
    workspace_id: workspaceFixture.id,
    started_at: "2026-04-20T02:10:00Z",
    ended_at: "2026-04-20T02:12:00Z",
    status: "failed",
    workflow_slug: "beta",
    error_text: "exit 1",
  },
];

export function buildRunSnapshotFixture(overrides: Partial<RunSnapshot> = {}): RunSnapshot {
  return {
    run: {
      run_id: "run-task-02",
      mode: "task",
      presentation_mode: "text",
      workspace_id: workspaceFixture.id,
      workflow_slug: workflowAlphaFixture.slug,
      status: "running",
      started_at: "2026-04-20T03:10:00Z",
    },
    jobs: [
      {
        index: 0,
        job_id: "job-1",
        status: "running",
        updated_at: "2026-04-20T03:11:00Z",
      },
    ],
    transcript: [
      {
        content: "Storybook route harness booted.",
        role: "assistant",
        sequence: 1,
        stream: "stdout",
        timestamp: "2026-04-20T03:11:30Z",
      },
    ],
    usage: {
      input_tokens: 120,
      output_tokens: 64,
      total_tokens: 184,
    },
    next_cursor: "2026-04-20T03:11:30Z|00000000000000000001",
    ...overrides,
  } as RunSnapshot;
}

export const runSnapshotFixture = buildRunSnapshotFixture();

export function buildRunTranscriptFixture(overrides: Partial<RunTranscript> = {}): RunTranscript {
  return {
    run_id: "run-task-02",
    messages: [
      {
        id: "msg-1",
        role: "assistant",
        parts: [
          {
            type: "text",
            text: "Storybook route harness booted.",
            state: "done",
          },
          {
            type: "dynamic-tool",
            toolCallId: "tool-1",
            toolName: "Bash",
            state: "output-available",
            input: { command: "bun test" },
            output: {
              blocks: [
                {
                  type: "terminal_output",
                  command: "bun test",
                  output: "1 test passed",
                  exitCode: 0,
                },
              ],
            },
          },
        ],
      },
    ],
    ...overrides,
  } as RunTranscript;
}

export const runTranscriptFixture = buildRunTranscriptFixture();

export const completedRunSnapshotFixture = buildRunSnapshotFixture({
  run: {
    ...runSnapshotFixture.run,
    status: "completed",
    ended_at: "2026-04-20T03:15:00Z",
  },
  jobs: [
    {
      index: 0,
      job_id: "job-1",
      status: "completed",
      updated_at: "2026-04-20T03:15:00Z",
    },
  ],
});

export const dispatchedRunFixture: Run = {
  run_id: "run-task-started",
  mode: "task",
  presentation_mode: "text",
  workspace_id: workspaceFixture.id,
  started_at: "2026-04-20T05:00:00Z",
  status: "queued",
  workflow_slug: workflowAlphaFixture.slug,
};
