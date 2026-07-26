import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { Command, CommandInput } from "@agh/ui";

import { CenteredSurface } from "@/storybook/story-layout";
import { agentFixtures } from "@/systems/agent/mocks";

import { AgentCommandList } from "../agent-command-list";
import { withStoryAgentCategories } from "./agent-command-select.stories";

const categorizedAgents = withStoryAgentCategories(agentFixtures);

const meta: Meta<typeof AgentCommandList> = {
  title: "systems/agent/components/AgentCommandList",
  component: AgentCommandList,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Grouped command list of agents partitioned by category. Designed to be embedded inside a `Command` shell or the project's `CommandSelect` popover. Provider and category labels render as mono uppercase pills using the `text-badge tracking-mono` kit convention.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Default — categorized agents grouped by their category path. Items are
 * unselected; the consumer wires `isSelected` to its own selection state.
 */
export const Default: Story = {
  args: {
    agents: categorizedAgents,
    isSelected: () => false,
    onSelect: fn(),
  },
  render: args => (
    <CenteredSurface>
      <div className="w-full max-w-md">
        <Command>
          <CommandInput placeholder="Search agents..." data-testid="agent-command-input" />
          <AgentCommandList {...args} />
        </Command>
      </div>
    </CenteredSurface>
  ),
};

/**
 * Selected — first agent in the list is marked selected; rendered with
 * `data-checked="true"` so consumers can style the active row.
 */
export const Selected: Story = {
  args: {
    agents: categorizedAgents,
    isSelected: agent => agent.name === categorizedAgents[0]?.name,
    onSelect: fn(),
  },
  render: args => (
    <CenteredSurface>
      <div className="w-full max-w-md">
        <Command>
          <CommandInput placeholder="Search agents..." data-testid="agent-command-input" />
          <AgentCommandList {...args} />
        </Command>
      </div>
    </CenteredSurface>
  ),
};

/**
 * Empty — empty agents array renders the `CommandEmpty` slot with the default
 * "No agents match your search" copy.
 */
export const Empty: Story = {
  args: {
    agents: [],
    isSelected: () => false,
    onSelect: fn(),
  },
  render: args => (
    <CenteredSurface>
      <div className="w-full max-w-md">
        <Command>
          <CommandInput placeholder="Search agents..." data-testid="agent-command-input" />
          <AgentCommandList {...args} />
        </Command>
      </div>
    </CenteredSurface>
  ),
};
