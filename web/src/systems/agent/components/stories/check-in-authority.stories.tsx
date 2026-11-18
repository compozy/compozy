import type { Meta, StoryObj } from "@storybook/react-vite";

import { primaryAgentFixture } from "../../mocks/fixtures";
import { authorityCheckInsModelFixture } from "../../mocks/editor-fixtures";
import type { AgentPayload } from "../../types";
import { AgentCheckInsView } from "../agent-editors/check-ins-view";

const agent: AgentPayload = { ...primaryAgentFixture, label: "Triage Lead" };

const meta: Meta<typeof AgentCheckInsView> = {
  title: "systems/agent/components/CheckInAuthority",
  component: AgentCheckInsView,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "What check-ins may change on their own and the actions a proposal runs as, with the operator's approval of that standing consent.",
      },
    },
  },
  decorators: [
    Story => (
      <div className="mx-auto w-full max-w-5xl">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/** U6: writes picked from the agent's write tools, and two actions, one done without asking. */
export const ChangesAndActions: Story = {
  render: () => (
    <AgentCheckInsView agent={agent} model={authorityCheckInsModelFixture()} state="idle" />
  ),
};

/** U7: the block waits for the operator; check-ins only read until it's approved. */
export const WaitingForApproval: Story = {
  render: () => (
    <AgentCheckInsView
      agent={agent}
      model={authorityCheckInsModelFixture({ waiting: true })}
      state="idle"
    />
  ),
};

/** U7: a save that changes the block shows it and approves only on confirm. */
export const SaveConfirm: Story = {
  render: () => (
    <AgentCheckInsView
      agent={agent}
      model={authorityCheckInsModelFixture({ confirming: true })}
      state="idle"
    />
  ),
};
