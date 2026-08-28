import type { Meta, StoryObj } from "@storybook/react-vite";

import { CenteredSurface } from "@/storybook/story-layout";
import type { AgentEventPayload } from "@/systems/session";

import { SessionAgentReportedBlock } from "../session-agent-reported-block";

const meta: Meta<typeof SessionAgentReportedBlock> = {
  title: "systems/session/components/SessionAgentReportedBlock",
  component: SessionAgentReportedBlock,
  parameters: { layout: "centered" },
  decorators: [
    Story => (
      <CenteredSurface>
        <div className="w-full max-w-2xl">
          <Story />
        </div>
      </CenteredSurface>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

const completeReport: AgentEventPayload = {
  type: "terminal_output",
  origin: "agent_reported",
  session_id: "sess-release",
  turn_id: "turn-tests",
  title: "bun test --filter terminal",
  text: "$ bun test --filter terminal\n12 tests passed\n",
  reported_terminal: {
    id: "reported-terminal-1",
    cwd: "/workspace/compozy",
    total_bytes: 49,
    exit_code: 0,
  },
};

export const Complete: Story = {
  args: {
    data: completeReport,
  },
};

export const LongOutput: Story = {
  args: {
    data: {
      ...completeReport,
      title: "bun install",
      text: [
        "bun install v1.2.4",
        " + @xterm/xterm@6.0.1",
        ...Array.from({ length: 20 }, (_, index) => `saved ${index + 1} packages`),
      ].join("\n"),
      reported_terminal: {
        id: "reported-terminal-long",
        total_bytes: 840,
        exit_code: 0,
      },
    },
  },
};

/** Specimen the agent clipped — last lines only; the note states the real size. */
export const Bounded: Story = {
  args: {
    data: {
      ...completeReport,
      text: "Last output lines remain visible.\n",
      reported_terminal: {
        id: "reported-terminal-large",
        total_bytes: 163_750,
        truncated: true,
        exit_code: 0,
      },
    },
  },
};
