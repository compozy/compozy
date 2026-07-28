import type { Meta, StoryObj } from "@storybook/react-vite";

import { CenteredSurface } from "@/storybook/story-layout";
import { agentFixtures, primaryAgentFixture } from "@/systems/agent/mocks";

import { AgentIcon } from "../agent-icon";

const meta: Meta<typeof AgentIcon> = {
  title: "systems/agent/components/AgentIcon",
  component: AgentIcon,
  parameters: {
    layout: "centered",
  },
  args: {
    provider: primaryAgentFixture.provider,
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {},
  render: args => (
    <CenteredSurface className="gap-3">
      <AgentIcon {...args} className="size-5" />
      <span className="text-sm text-muted">{args.provider}</span>
    </CenteredSurface>
  ),
};

export const Providers: Story = {
  args: {},
  render: () => (
    <CenteredSurface className="gap-4">
      {agentFixtures.map(agent => (
        <div
          key={agent.name}
          className="flex min-w-28 flex-col items-center gap-2 rounded-xl border border-line bg-canvas-soft px-4 py-3"
        >
          <AgentIcon provider={agent.provider} className="size-5" />
          <span className="text-xs font-medium text-fg">{agent.name}</span>
        </div>
      ))}
    </CenteredSurface>
  ),
};
