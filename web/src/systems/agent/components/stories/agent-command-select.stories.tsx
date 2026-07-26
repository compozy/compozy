import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, waitFor, within } from "storybook/test";

import { Command, CommandInput } from "@agh/ui";
import { CenteredSurface } from "@/storybook/story-layout";
import { storyAgentNames } from "@/storybook/fintech-scenario";
import { agentFixtures } from "@/systems/agent/mocks";
import type { AgentPayload } from "@/systems/agent/types";

import { AgentCommandSelect } from "../agent-command-select";
import { AgentCommandMultiSelect } from "../agent-command-multi-select";
import { AgentCommandList } from "../agent-command-list";

const categoryByName: Record<string, string[]> = {
  [storyAgentNames.cto]: ["Engineering", "Leadership"],
  [storyAgentNames.platform]: ["Engineering", "Platform"],
  [storyAgentNames.frontend]: ["Engineering", "Platform"],
  [storyAgentNames.release]: ["Engineering", "Platform"],
  [storyAgentNames.cfo]: ["Finance"],
  [storyAgentNames.marketing]: ["Marketing", "Campaigns"],
  [storyAgentNames.copywriter]: ["Marketing", "Campaigns"],
  [storyAgentNames.product]: ["Product"],
  [storyAgentNames.support]: ["Support"],
  [storyAgentNames.fraud]: ["Risk", "Fraud"],
  [storyAgentNames.compliance]: ["Risk", "Compliance"],
};

export function withStoryAgentCategories(agents: AgentPayload[]): AgentPayload[] {
  return agents.map(agent => {
    const categoryPath = categoryByName[agent.name];
    return categoryPath ? { ...agent, category_path: categoryPath } : agent;
  });
}

export const categorizedAgents: AgentPayload[] = withStoryAgentCategories(agentFixtures);

const flatAgents: AgentPayload[] = agentFixtures.slice(0, 4);
const groupedAgents: AgentPayload[] = [
  {
    ...agentFixtures[0],
    name: "launch-ops",
    prompt: "Coordinate uncategorized launch escalations and operator follow-up.",
  },
  ...categorizedAgents,
];

interface FrameProps {
  children: React.ReactNode;
}

function Frame({ children }: FrameProps) {
  return (
    <CenteredSurface>
      <div className="w-full max-w-md">{children}</div>
    </CenteredSurface>
  );
}

const meta: Meta<typeof AgentCommandSelect> = {
  title: "systems/agent/components/AgentCommandSelect",
  component: AgentCommandSelect,
  parameters: {
    layout: "fullscreen",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Trigger displays the selected agent's name + provider + formatted category label.
 */
export const Selected: Story = {
  args: {
    agents: categorizedAgents,
    value: storyAgentNames.cto,
    onChange: fn(),
    triggerTestId: "agent-command-select-trigger",
  },
  render: args => (
    <Frame>
      <AgentCommandSelect {...args} />
    </Frame>
  ),
};

/**
 * Empty value: the trigger shows the placeholder.
 */
export const Empty: Story = {
  args: {
    agents: flatAgents,
    value: null,
    onChange: fn(),
    triggerTestId: "agent-command-select-trigger",
    placeholder: "Select an agent",
  },
  render: args => (
    <Frame>
      <AgentCommandSelect {...args} />
    </Frame>
  ),
};

/**
 * Grouped: opening the popover reveals one CommandGroup per category and a
 * root-level "Agents" group for uncategorized entries.
 */
export const Grouped: Story = {
  args: {
    agents: groupedAgents,
    value: null,
    onChange: fn(),
    triggerTestId: "agent-command-select-trigger",
  },
  render: args => (
    <Frame>
      <AgentCommandSelect {...args} />
    </Frame>
  ),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByTestId("agent-command-select-trigger"));
    await waitFor(() => {
      expect(canvas.getByTestId("agent-command-group-agents:root")).toHaveTextContent("Agents");
      expect(
        canvas.getByTestId("agent-command-group-category:Engineering/Platform")
      ).toBeInTheDocument();
      expect(
        canvas.getByTestId("agent-command-group-category:Marketing/Campaigns")
      ).toBeInTheDocument();
    });
  },
};

/**
 * No-match: typing a query that matches no agent renders the empty state.
 */
export const NoMatch: Story = {
  args: {
    agents: flatAgents,
    value: null,
    onChange: fn(),
    triggerTestId: "agent-command-select-trigger",
  },
  render: args => (
    <Frame>
      <AgentCommandSelect {...args} />
    </Frame>
  ),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByTestId("agent-command-select-trigger"));
    await userEvent.type(canvas.getByTestId("agent-command-input"), "nonexistent-agent");
    await waitFor(() => expect(canvas.getByTestId("agent-command-empty")).toBeInTheDocument());
  },
};

/**
 * Base command list renders grouped agents without the popover wrapper used by
 * the single and multi-select controls.
 */
export const CommandList: StoryObj<typeof AgentCommandList> = {
  args: {},
  render: () => (
    <Frame>
      <Command>
        <CommandInput placeholder="Search agents..." data-testid="agent-command-input" />
        <AgentCommandList agents={groupedAgents} isSelected={() => false} onSelect={fn()} />
      </Command>
    </Frame>
  ),
};

/**
 * Multi-select harness: keeps the popover open across selections, marks each
 * picked item with `data-checked=true`, and surfaces a selected count.
 */
export const MultiSelect: StoryObj<typeof AgentCommandMultiSelect> = {
  args: {},
  render: () => {
    function Harness() {
      const [value, setValue] = useState<string[]>([storyAgentNames.cto]);
      return (
        <Frame>
          <AgentCommandMultiSelect
            agents={categorizedAgents}
            value={value}
            onToggle={setValue}
            triggerTestId="agent-command-multi-trigger"
          />
        </Frame>
      );
    }
    return <Harness />;
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByTestId("agent-command-multi-trigger"));
    await waitFor(() =>
      expect(canvas.getByTestId(`agent-command-item-${storyAgentNames.cto}`)).toHaveAttribute(
        "data-checked",
        "true"
      )
    );
    await userEvent.click(canvas.getByTestId(`agent-command-item-${storyAgentNames.cfo}`));
    await waitFor(() =>
      expect(canvas.getByTestId(`agent-command-item-${storyAgentNames.cfo}`)).toHaveAttribute(
        "data-checked",
        "true"
      )
    );
    await expect(canvas.getByTestId("agent-command-input")).toBeInTheDocument();
  },
};
