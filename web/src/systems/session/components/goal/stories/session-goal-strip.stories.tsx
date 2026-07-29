import type { Meta, StoryObj } from "@storybook/react-vite";

import { CenteredSurface } from "@/storybook/story-layout";
import type { SessionGoalSnapshot } from "@/systems/session/types";

import { SessionGoalStrip } from "../session-goal-strip";

function snapshot(overrides: Partial<SessionGoalSnapshot> = {}): SessionGoalSnapshot {
  return {
    bound_session_id: "session_1",
    cause: null,
    context: {
      nudge_ratio: 0.7,
      ratio: 0.41,
      reported_at: "2026-07-10T12:00:00Z",
      size: 200_000,
      state: "known",
      used: 84_120,
    },
    contract_summary: "All 16 transcript components on session.css tokens; make verify green.",
    last_verdict: {
      blocking_issues: [],
      evaluated_at: "2026-07-10T12:00:00Z",
      evidence_ref: "verdict_012",
      outcome: "approved",
    },
    live: true,
    node_id: "plan-4",
    objective: "Migrate session transcript to the calm-surface vocabulary",
    origin_session_id: "session_1",
    run_id: "run_1",
    run_status: "running",
    status: "active",
    turn_limit: 20,
    turns_used: 3,
    ...overrides,
  };
}

const meta: Meta<typeof SessionGoalStrip> = {
  title: "systems/session/components/SessionGoalStrip",
  component: SessionGoalStrip,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "The goal as one quiet line above the transcript: state dot + GOAL kicker + objective + mono facts + chevron, expanding to key/value rows. Lifecycle actions live on the window head; the body Actions row only stages /goal commands into the composer.",
      },
    },
  },
  decorators: [
    Story => (
      <CenteredSurface>
        <div className="w-full max-w-2xl">
          <Story />
        </div>
      </CenteredSurface>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Active: Story = {
  args: { snapshot: snapshot() },
};

export const Paused: Story = {
  args: { snapshot: snapshot({ status: "paused", run_status: "paused" }) },
};

export const Blocked: Story = {
  args: {
    snapshot: snapshot({ status: "blocked", run_status: "blocked", cause: "goal_agent_refused" }),
  },
};

export const Done: Story = {
  args: { snapshot: snapshot({ live: false, status: "complete", run_status: "done" }) },
};

export const Moved: Story = {
  args: {
    snapshot: snapshot({ bound_session_id: "session_2", origin_session_id: "session_1" }),
  },
};

export const WithDraftAction: Story = {
  args: {
    snapshot: snapshot(),
    composerAffordance: { kind: "draft", expandedObjective: "Ship the calm transcript" },
    onPrefillComposer: () => {},
  },
};
