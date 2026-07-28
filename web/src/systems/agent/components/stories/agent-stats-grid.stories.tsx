import type { ReactNode } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";

import { CenteredSurface } from "@/storybook/story-layout";

import { AgentStatsGrid } from "../agent-stats-grid";

const meta: Meta<typeof AgentStatsGrid> = {
  title: "systems/agent/components/AgentStatsGrid",
  component: AgentStatsGrid,
  parameters: { layout: "fullscreen" },
};

export default meta;
type Story = StoryObj<typeof meta>;

function Frame({ children }: { children: ReactNode }) {
  return (
    <CenteredSurface>
      <div className="w-full max-w-4xl">{children}</div>
    </CenteredSurface>
  );
}

export const Default: Story = {
  args: {
    active: 7,
    runtimeLabel: "4h 12m",
    failed: 1,
    lastActivityAt: new Date().toISOString(),
    sessionsTotal: 205,
  },
  render: args => (
    <Frame>
      <AgentStatsGrid {...args} />
    </Frame>
  ),
};

export const Idle: Story = {
  args: {
    active: 0,
    runtimeLabel: "12m",
    failed: 0,
    lastActivityAt: "2026-04-17T18:10:00Z",
    sessionsTotal: 42,
  },
  render: args => (
    <Frame>
      <AgentStatsGrid {...args} />
    </Frame>
  ),
};

export const Empty: Story = {
  args: {
    active: 0,
    runtimeLabel: "0s",
    failed: 0,
    lastActivityAt: null,
    sessionsTotal: 0,
  },
  render: args => (
    <Frame>
      <AgentStatsGrid {...args} />
    </Frame>
  ),
};

export const MetricsUnavailable: Story = {
  args: {
    active: 2,
    runtimeLabel: null,
    failed: null,
    lastActivityAt: null,
    sessionsTotal: 40,
    metricsAvailable: false,
  },
  render: args => (
    <Frame>
      <AgentStatsGrid {...args} />
    </Frame>
  ),
};

export const SessionsVariant: Story = {
  args: {
    variant: "sessions",
    active: 2,
    runtimeLabel: "4h 12m",
    failed: 1,
    sessionsTotal: 6,
  },
  render: args => (
    <Frame>
      <AgentStatsGrid {...args} />
    </Frame>
  ),
};
