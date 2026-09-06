import type { Meta, StoryObj } from "@storybook/react-vite";

import { primarySessionFixture } from "@/systems/session/mocks";

import { SessionThinkingRow } from "../session-thinking-row";

const STARTED_AT = new Date(Date.now() - 134_000).toISOString();

function session(currentTool?: string, agents = 0, pending = 0) {
  return {
    activity: {
      current_tool: currentTool,
      elapsed_ms: 134_000,
      elapsed_seconds: 134,
      idle_seconds: 0,
      iteration_current: 1,
      iteration_max: 1,
      turn_id: "story-turn",
      turn_started_at: STARTED_AT,
    },
    pending_interactions: primarySessionFixture.pending_interactions.slice(0, pending),
    supervision: {
      quiet_warning: null,
      sources: [],
      work_signals: Array.from({ length: agents }, (_, index) => ({
        kind: "active_child" as const,
        since: STARTED_AT,
        ref: `sess_child_${index + 1}`,
      })),
    },
  };
}

const stoppedTurn = {
  startedAtMs: Date.parse("2026-07-07T12:00:00Z"),
  endedAtMs: Date.parse("2026-07-07T12:01:40Z"),
  cause: "stopped" as const,
  failureCause: null,
};

/**
 * The one line that says what the agent is doing right now (S3): "Thinking…"
 * until the first content, "Working for {elapsed} · {activity}" on the daemon's
 * durable turn start, still working while spawned agents run, and a frozen
 * sentence after a stop or failure. A completed turn reads nothing here.
 */
const meta: Meta<typeof SessionThinkingRow> = {
  title: "systems/session/components/SessionThinkingRow",
  component: SessionThinkingRow,
  parameters: { layout: "centered" },
  decorators: [
    Story => (
      <div className="flex w-96 flex-col gap-4 border border-line bg-background p-6">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/** VC-10 — before any content: dots and a shimmering "Thinking…", no assistant shell anywhere. */
export const Thinking: Story = {
  args: { session: session(), running: true, thinking: true, lastTurn: null },
};

/** VC-11 — "Working for 2m 14s · Running shell": elapsed from `turn_started_at`, activity from `current_tool`. */
export const WorkingWithTool: Story = {
  args: { session: session("Bash"), running: true, thinking: false, lastTurn: null },
};

/** Between tools the row reads the elapsed alone. */
export const WorkingBetweenTools: Story = {
  args: { session: session(), running: true, thinking: false, lastTurn: null },
};

/** VC-11 — the lead reply landed, two helpers have not: "· 2 agents running". */
export const WorkingWithChildren: Story = {
  args: { session: session(undefined, 2), running: true, thinking: false, lastTurn: null },
};

/** A pending decision is activity too: "· Waiting for your decision". */
export const WaitingForDecision: Story = {
  args: { session: session("Bash", 0, 1), running: true, thinking: false, lastTurn: null },
};

/** VC-12 — "Stopped by you after 1m 40s" in warning ink, the only tone the row wears besides danger. */
export const StoppedByYou: Story = {
  args: { session: session(), running: false, thinking: false, lastTurn: stoppedTurn },
};

/** VC-12 — the daemon escalated a stop the agent never answered and verified the close: never "by you". */
export const StoppedEscalated: Story = {
  args: {
    session: session(),
    running: false,
    thinking: false,
    lastTurn: {
      startedAtMs: Date.parse("2026-07-07T12:00:00Z"),
      endedAtMs: Date.parse("2026-07-07T12:00:14Z"),
      cause: "stopped",
      failureCause: null,
      stop: { kind: "escalated" },
    },
  },
};

/** VC-12 — supervision stopped the session; the span is the episode's actual quiet time. */
export const StoppedInactivity: Story = {
  args: {
    session: session(),
    running: false,
    thinking: false,
    lastTurn: {
      startedAtMs: Date.parse("2026-07-07T11:30:00Z"),
      endedAtMs: Date.parse("2026-07-07T12:01:00Z"),
      cause: "stopped",
      failureCause: null,
      stop: { kind: "inactivity", noWorkMs: 40 * 60_000 },
    },
  },
};

/** US-009.EC-2 — the stop arrived after the turn finished: a faint, transient note; the turn folds normally. */
export const StopCompletionNote: Story = {
  args: {
    session: session(),
    running: false,
    thinking: false,
    lastTurn: null,
    stopCompletionNote: true,
  },
};

/** A turn that failed before or during its reply: "Failed after 4s · rate limited". */
export const Failed: Story = {
  args: {
    session: session(),
    running: false,
    thinking: false,
    lastTurn: {
      startedAtMs: Date.parse("2026-07-07T12:00:00Z"),
      endedAtMs: Date.parse("2026-07-07T12:00:04Z"),
      cause: "failed",
      failureCause: "rate limited",
    },
  },
};

/** A background window: the clock freezes and the row says "as of HH:MM". */
export const PausedAsOf: Story = {
  args: {
    session: session("Bash"),
    running: true,
    thinking: false,
    lastTurn: null,
    liveDataEnabled: false,
    pausedAtMs: Date.now() - 4 * 60_000,
  },
};

/** Reduced motion: no dots, no shimmer — the words carry every state. */
export const ReducedMotion: Story = {
  args: {
    session: session("Bash"),
    running: true,
    thinking: false,
    lastTurn: null,
    reducedMotion: true,
  },
};
