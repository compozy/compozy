import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { automationTriggerDetailFixtures } from "@/systems/automation/mocks";

import { TriggerInspectSheet } from "../trigger-detail/trigger-inspect-sheet";

const [agentTrigger, loopTrigger, webhookTrigger] = automationTriggerDetailFixtures;

const meta: Meta<typeof TriggerInspectSheet> = {
  title: "systems/automation/components/TriggerInspectSheet",
  component: TriggerInspectSheet,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "The raw read behind the trigger detail page: runtime enums, dispatch internals, and the activation envelope the target actually receives.",
      },
    },
  },
  args: { onOpenChange: fn(), open: true },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Agent: Story = {
  args: { trigger: agentTrigger },
};

export const Loop: Story = {
  args: { trigger: loopTrigger },
};

/** Webhook swaps the retry tile for the secret's presence — never its value. */
export const Webhook: Story = {
  args: { trigger: webhookTrigger },
};
