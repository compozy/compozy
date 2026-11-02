import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { agentInstanceFixture, agentInstanceFixtures } from "@/systems/agent/mocks";

import { StaysOnRow } from "../stays-on-row";

const CREW = Array.from({ length: 12 }, (_, index) =>
  agentInstanceFixture({
    id: `ainst-${index}`,
    agent_name: `agent-${index}`,
    label: `Agent ${index + 1}`,
    needs_attention: index === 7,
    activity: index % 3 === 0 ? "working" : "idle",
    resolved_appearance: {
      version: 1,
      avatar: "generated",
      seed: `agent-${index}`,
      generation: 1,
      color: "#2aa8a8",
    },
  })
);

const meta: Meta<typeof StaysOnRow> = {
  title: "systems/session/components/StaysOnRow",
  component: StaysOnRow,
  args: { onOpenSession: fn(), instances: agentInstanceFixtures, scopeName: "compozy" },
  parameters: { layout: "centered" },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const NeedsYouLeads: Story = {};

export const Overflow: Story = { args: { instances: CREW } };

export const OpenMainSession: Story = { args: { currentSessionId: "sess-main-orchestrator" } };
