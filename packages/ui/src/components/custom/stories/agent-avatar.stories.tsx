import type { Meta, StoryObj } from "@storybook/react-vite";

import {
  AGENT_AVATAR_STATE_LABEL,
  type AgentAvatarAppearance,
  type AgentAvatarState,
} from "../../../lib/agent-avatar";
import { AgentAvatar } from "../agent-avatar";

const STATES = Object.keys(AGENT_AVATAR_STATE_LABEL) as AgentAvatarState[];
const SIZES = [16, 20, 24, 32, 48, 64, 92, 112];
const GENERATED: AgentAvatarAppearance = { avatar: "generated", seed: "triage", generation: 1 };

const meta: Meta<typeof AgentAvatar> = {
  title: "components/custom/AgentAvatar",
  component: AgentAvatar,
  args: { appearance: GENERATED, label: "Triage Lead", size: 64 },
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "An agent's mark: a generated face, icon or emoji, with its pose, provider badge from 24 px and a state signal. Only `working` animates.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Generated: Story = { args: { provider: "claude" } };

export const Poses: Story = {
  render: args => (
    <div className="flex flex-wrap items-center gap-6">
      {STATES.map(state => (
        <AgentAvatar {...args} key={state} provider="claude" state={state} />
      ))}
    </div>
  ),
};

export const Emoji: Story = {
  args: { appearance: { avatar: "emoji", emoji: "🦊", color: "#e8b04a" }, state: "working" },
};

export const NeedsYou: Story = { args: { provider: "codex", state: "needs-you" } };

export const Sizes: Story = {
  render: args => (
    <div className="flex items-center gap-4">
      {SIZES.map(size => (
        <AgentAvatar {...args} key={size} provider="claude" size={size} state="working" />
      ))}
    </div>
  ),
};
