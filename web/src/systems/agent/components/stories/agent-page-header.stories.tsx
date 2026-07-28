import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { CenteredSurface } from "@/storybook/story-layout";

import { AgentPageActions, AgentPageOverflow, AgentPageStatusPill } from "../agent-page-header";

const meta: Meta<typeof AgentPageActions> = {
  title: "systems/agent/components/AgentPageHeader",
  component: AgentPageActions,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Topbar-slot building blocks for the agent detail route: status, one primary New session action, and overflow Edit/Duplicate/Delete.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Actions: Story = {
  args: {
    onNewSession: fn(),
    isCreatingSession: false,
    newSessionDisabled: false,
  },
  render: args => (
    <CenteredSurface>
      <div className="flex w-full max-w-3xl items-center justify-end gap-2">
        <AgentPageActions {...args} />
        <AgentPageOverflow onDelete={fn()} onDuplicate={fn()} onEditSettings={fn()} />
      </div>
    </CenteredSurface>
  ),
};

export const CreatingSession: Story = {
  args: {
    onNewSession: fn(),
    isCreatingSession: true,
    newSessionDisabled: true,
  },
  render: args => (
    <CenteredSurface>
      <div className="flex w-full max-w-3xl items-center justify-end gap-2">
        <AgentPageActions {...args} />
        <AgentPageOverflow onDelete={fn()} onDuplicate={fn()} onEditSettings={fn()} />
      </div>
    </CenteredSurface>
  ),
};

export const StatusIdle: StoryObj<typeof AgentPageStatusPill> = {
  args: {},
  render: () => (
    <CenteredSurface>
      <AgentPageStatusPill activeCount={0} />
    </CenteredSurface>
  ),
};

export const StatusActive: StoryObj<typeof AgentPageStatusPill> = {
  args: {},
  render: () => (
    <CenteredSurface>
      <AgentPageStatusPill activeCount={3} />
    </CenteredSurface>
  ),
};
