import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";

import { CenteredSurface } from "@/storybook/story-layout";
import { storyAgentNames } from "@/storybook/fintech-scenario";
import { agentFixtures } from "@/systems/agent/mocks";

import { AgentCommandMultiSelect } from "../agent-command-multi-select";
import { withStoryAgentCategories } from "./agent-command-select.stories";

const categorizedAgents = withStoryAgentCategories(agentFixtures);

const meta: Meta<typeof AgentCommandMultiSelect> = {
  title: "systems/agent/components/AgentCommandMultiSelect",
  component: AgentCommandMultiSelect,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Popover-driven multi-selector wrapping `AgentCommandList`. The trigger surfaces the selected count via a mono `Pill`. The harness preserves selection across opens and toggles each agent on click.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

interface HarnessProps {
  initial?: string[];
}

function Harness({ initial = [] }: HarnessProps) {
  const [value, setValue] = useState<string[]>(initial);
  return (
    <CenteredSurface>
      <div className="w-full max-w-md">
        <AgentCommandMultiSelect
          agents={categorizedAgents}
          value={value}
          onToggle={setValue}
          triggerTestId="agent-command-multi-trigger"
          countTestId="agent-command-multi-count"
        />
      </div>
    </CenteredSurface>
  );
}

/**
 * Default — closed state with no selection. Trigger shows the placeholder
 * and the count Pill renders `0`.
 */
export const Default: Story = {
  args: {
    agents: categorizedAgents,
    value: [],
    onToggle: () => undefined,
  },
  render: () => <Harness />,
};

/**
 * WithSelection — two agents pre-selected; trigger label switches to "2 selected"
 * and the mono count Pill reads `2`.
 */
export const WithSelection: Story = {
  args: {
    agents: categorizedAgents,
    value: [storyAgentNames.cto, storyAgentNames.cfo],
    onToggle: () => undefined,
  },
  render: () => <Harness initial={[storyAgentNames.cto, storyAgentNames.cfo]} />,
};

/**
 * Disabled — trigger is non-interactive; the popover cannot be opened.
 */
export const Disabled: Story = {
  args: {
    agents: categorizedAgents,
    value: [storyAgentNames.cto],
    onToggle: () => undefined,
    disabled: true,
  },
  render: args => (
    <CenteredSurface>
      <div className="w-full max-w-md">
        <AgentCommandMultiSelect
          {...args}
          triggerTestId="agent-command-multi-trigger"
          countTestId="agent-command-multi-count"
        />
      </div>
    </CenteredSurface>
  ),
};
