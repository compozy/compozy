import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { StorySurface } from "@/storybook/story-layout";

import {
  GoalStatusChip,
  type GoalStatusChipProps,
  type SessionGoalSnapshot,
} from "../goal-status-chip";

const baseSnapshot: SessionGoalSnapshot = {
  bound_session_id: "session_release_042",
  cause: null,
  context: {
    nudge_ratio: 0.8,
    ratio: 0.5,
    reported_at: "2026-07-10T19:42:00Z",
    size: 200_000,
    state: "known",
    used: 100_000,
  },
  contract_summary: "Ship only after the runtime checks and implementation review pass.",
  last_verdict: {
    blocking_issues: [{ id: "issue_042", note: "Run detail still lacks turn evidence." }],
    evaluated_at: "2026-07-10T19:40:00Z",
    evidence_ref: "blob_goal_review_0042",
    outcome: "rejected",
  },
  live: true,
  node_id: "implement_goal_surface",
  objective: "Finish the Goal operator surfaces and preserve the existing watch-event controls.",
  origin_session_id: "session_release_042",
  run_id: "looprun_goal_042",
  run_status: "running",
  status: "active",
  turn_limit: 20,
  turns_used: 7,
};

const callbacks: Pick<
  GoalStatusChipProps,
  "onPause" | "onResume" | "onApprove" | "onClear" | "onPrefillComposer"
> = {
  onPause: fn(),
  onResume: fn(),
  onApprove: fn(),
  onClear: fn(),
  onPrefillComposer: fn(),
};

function GoalStorySurface({ children }: { children: React.ReactNode }) {
  return <StorySurface className="mx-auto max-w-5xl">{children}</StorySurface>;
}

const meta: Meta<typeof GoalStatusChip> = {
  title: "systems/session/components/GoalStatusChip",
  component: GoalStatusChip,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "The session-header pointer to the canonical Goal snapshot, including truthful context freshness, controls, and Run navigation.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Active Goal with known context usage and the ordinary pause and clear controls. */
export const Default: Story = {
  args: { snapshot: baseSnapshot, ...callbacks },
  render: args => (
    <GoalStorySurface>
      <GoalStatusChip {...args} />
    </GoalStorySurface>
  ),
};

/** All six public Goal statuses, kept lowercase and mapped to semantic signal tones. */
export const StatusMatrix: Story = {
  args: { snapshot: baseSnapshot },
  render: () => {
    const statuses = [
      ["active", "running", true],
      ["paused", "paused", true],
      ["blocked", "blocked", false],
      ["usage-limited", "needs-approval", true],
      ["budget-limited", "needs-approval", true],
      ["complete", "done", false],
    ] as const;
    return (
      <StorySurface className="grid gap-4 xl:grid-cols-2">
        {statuses.map(([status, runStatus, live]) => (
          <GoalStatusChip
            key={status}
            snapshot={{
              ...baseSnapshot,
              live,
              run_status: runStatus,
              status,
              last_verdict: status === "active" ? baseSnapshot.last_verdict : null,
            }}
          />
        ))}
      </StorySurface>
    );
  },
};

/** Known-normal, known-warning, unknown, and pending context states without invented percentages. */
export const ContextMatrix: Story = {
  args: { snapshot: baseSnapshot },
  render: () => {
    const contexts: Array<[string, SessionGoalSnapshot["context"]]> = [
      ["known-normal", { ...baseSnapshot.context, ratio: 0.5 }],
      ["known-warning", { ...baseSnapshot.context, ratio: 0.85, used: 170_000 }],
      [
        "unknown",
        {
          nudge_ratio: 0.8,
          ratio: null,
          reported_at: null,
          size: null,
          state: "unknown",
          used: null,
        },
      ],
      [
        "pending",
        {
          nudge_ratio: 0.8,
          ratio: null,
          reported_at: "2026-07-10T19:42:00Z",
          size: null,
          state: "pending",
          used: null,
        },
      ],
    ];
    return (
      <StorySurface className="grid gap-4 xl:grid-cols-2">
        {contexts.map(([key, context]) => (
          <GoalStatusChip key={key} snapshot={{ ...baseSnapshot, context }} />
        ))}
      </StorySurface>
    );
  },
};

/** A completed Goal whose continuous binding moved away from the origin session after reseed. */
export const TerminalMovedSession: Story = {
  args: {
    snapshot: {
      ...baseSnapshot,
      bound_session_id: "session_release_042_reseed_2",
      cause: "goal_reseed_confirmation_required",
      live: false,
      run_status: "done",
      status: "complete",
      turns_used: 12,
      last_verdict: {
        blocking_issues: [],
        evaluated_at: "2026-07-10T20:06:00Z",
        evidence_ref: "blob_goal_ship_0042",
        outcome: "approved",
      },
    },
    onClear: callbacks.onClear,
  },
  render: args => (
    <GoalStorySurface>
      <GoalStatusChip {...args} />
    </GoalStorySurface>
  ),
};

/** Replace-required and draft results expose composer prefill, never implicit execution. */
export const ComposerPrefill: Story = {
  args: {
    snapshot: { ...baseSnapshot, cause: "goal_replace_required" },
    composerAffordance: {
      kind: "replace",
      expectedRunId: baseSnapshot.run_id,
      objective: "Finish the Goal surfaces and rerun visual verification.",
    },
    onPrefillComposer: callbacks.onPrefillComposer,
    onPause: callbacks.onPause,
    onClear: callbacks.onClear,
  },
  render: args => (
    <GoalStorySurface>
      <GoalStatusChip {...args} />
    </GoalStorySurface>
  ),
};
