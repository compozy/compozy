import type { Meta, StoryObj } from "@storybook/react-vite";

import { StorySurface } from "@/storybook/story-layout";

import type { GoalTurnTimelineItem } from "../../../hooks/use-goal-turns";
import { GoalTurnTimeline } from "../goal-turn-timeline";

const completedTurn: GoalTurnTimelineItem = {
  key: "goal:0:1:7:1:prompt_7",
  seq: 7,
  generation: 1,
  nodeId: "implement_goal_surface",
  itemIndex: 0,
  turn: 7,
  promptAttempt: 1,
  promptId: "prompt_7",
  sessionId: "session_release_042",
  resultStatus: "completed",
  stopReason: "end_turn",
  reasonCode: null,
  verdictOutcome: "rejected",
  blockingIssues: [{ id: "issue_042", note: "The Run page does not expose turn evidence yet." }],
  evidenceRef: "blob_goal_review_0042",
  tokensUsed: 8_412,
  startedAt: "2026-07-10T19:34:00Z",
  endedAt: "2026-07-10T19:40:00Z",
};

const openTurn: GoalTurnTimelineItem = {
  ...completedTurn,
  key: "goal:0:2:8:1:prompt_8",
  seq: 8,
  generation: 2,
  turn: 8,
  promptId: "prompt_8",
  sessionId: "session_release_042_reseed_2",
  resultStatus: null,
  stopReason: null,
  verdictOutcome: null,
  blockingIssues: [],
  evidenceRef: null,
  tokensUsed: null,
  startedAt: "2026-07-10T20:18:00Z",
  endedAt: null,
};

const ambiguousTurn: GoalTurnTimelineItem = {
  ...openTurn,
  key: "goal:0:2:9:1:prompt_9",
  seq: 9,
  turn: 9,
  promptId: "prompt_9",
  resultStatus: "ambiguous",
  reasonCode: "goal_recovery_ambiguous",
  blockingIssues: [{ id: "recovery", note: "Terminal provider evidence was incomplete." }],
  evidenceRef: "blob_goal_recovery_0042",
  startedAt: "2026-07-10T20:21:00Z",
  endedAt: "2026-07-10T20:32:00Z",
};

function Surface({ children }: { children: React.ReactNode }) {
  return (
    <StorySurface className="mx-auto max-w-4xl">
      <div className="rounded-md border border-line bg-canvas-soft px-4 py-4">{children}</div>
    </StorySurface>
  );
}

const meta: Meta<typeof GoalTurnTimeline> = {
  title: "systems/loops/components/GoalTurnTimeline",
  component: GoalTurnTimeline,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Total-order Goal turn audit with stop reason, nullable verdict, blocking issues, and evidence.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Rejected turn with evidence and a structured blocking issue. */
export const Default: Story = {
  args: { turns: [completedTurn], live: false },
  render: args => (
    <Surface>
      <GoalTurnTimeline {...args} />
    </Surface>
  ),
};

/** Live audit with an open turn whose stop reason and verdict remain nullable. */
export const InProgress: Story = {
  args: { turns: [completedTurn, openTurn], live: true },
  render: args => (
    <Surface>
      <GoalTurnTimeline {...args} />
    </Surface>
  ),
};

/** Recovery ambiguity remains explicit with its durable cause and evidence. */
export const RecoveryAmbiguous: Story = {
  args: { turns: [ambiguousTurn], live: false },
  render: args => (
    <Surface>
      <GoalTurnTimeline {...args} />
    </Surface>
  ),
};

/** Empty audit state before the first work prompt claim. */
export const Empty: Story = {
  args: { turns: [], live: true },
  render: args => (
    <Surface>
      <GoalTurnTimeline {...args} />
    </Surface>
  ),
};
