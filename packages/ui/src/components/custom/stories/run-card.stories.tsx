import type { Meta, StoryObj } from "@storybook/react-vite";

import { RunCard } from "../run-card";

const meta: Meta<typeof RunCard> = {
  title: "components/custom/RunCard",
  component: RunCard,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Active-run summary — pill row (status pill + run-id mono + session info + attempt counter + optional warning) followed by a 4-col stat grid (CHANNEL / QUEUED / STARTED / ELAPSED). No `border-l-2 border-l-accent` rail.",
      },
    },
  },
  decorators: [
    Story => (
      <div className="w-[720px] bg-background p-6">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

const BASE = {
  runId: "run_2026_05_11_planner_42",
  sessionInfo: "session 42 · agent planner-prime",
  attempt: 1,
  channel: "cli",
  queuedAt: "2026-05-11T11:58:30Z",
  startedAt: "2026-05-11T11:59:05Z",
  elapsed: "3m 42s",
} as const;

export const Running: Story = {
  args: { status: "in_progress", ...BASE },
};

export const Completed: Story = {
  args: { status: "completed", ...BASE, elapsed: "12m 04s" },
};

export const Failed: Story = {
  args: {
    status: "failed",
    ...BASE,
    attempt: 3,
    elapsed: "0m 22s",
  },
};

export const NeedsAttention: Story = {
  args: {
    status: "needs_attention",
    ...BASE,
    attempt: 2,
    elapsed: "11m 18s",
    warning: {
      tone: "warning",
      message: "Queued past escalation budget — no agent claimed this run",
    },
  },
};

export const WithWarning: Story = {
  args: {
    status: "in_progress",
    ...BASE,
    attempt: 2,
    warning: {
      tone: "warning",
      message: "Awaiting filesystem write permission from operator",
    },
  },
};
