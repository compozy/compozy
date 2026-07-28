import type { Meta, StoryObj } from "@storybook/react-vite";

import { CenteredSurface } from "@/storybook/story-layout";
import { FIXTURE_AGENT_DEFINITION_DIGEST } from "@/systems/agent/mocks";

import { AgentMcpServersPanel } from "../agent-mcp-servers-panel";

const meta: Meta<typeof AgentMcpServersPanel> = {
  title: "systems/agent/components/AgentMcpServersPanel",
  component: AgentMcpServersPanel,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Read-only MCP list for Configuration / Settings. Renders env and secret KEY NAMES only.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Empty: Story = {
  args: {
    agent: {
      name: "coder",
      provider: "claude",
      prompt: "x",
      definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
      origin: "workspace",
      mcp_servers: [],
    },
  },
  render: args => (
    <CenteredSurface>
      <AgentMcpServersPanel {...args} />
    </CenteredSurface>
  ),
};

export const WithServers: Story = {
  args: {
    agent: {
      name: "coder",
      provider: "claude",
      prompt: "x",
      definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
      origin: "workspace",
      mcp_servers: [
        {
          name: "github",
          transport: "stdio",
          command: "npx",
          env: { GITHUB_API_URL: "redacted" },
          secret_env: { GITHUB_TOKEN: "redacted" },
        },
      ],
    },
  },
  render: args => (
    <CenteredSurface>
      <AgentMcpServersPanel {...args} />
    </CenteredSurface>
  ),
};
