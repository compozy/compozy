import type { Meta, StoryObj } from "@storybook/react-vite";

import { JsonViewer } from "../json-viewer";

const meta: Meta<typeof JsonViewer> = {
  title: "components/custom/JsonViewer",
  component: JsonViewer,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Structured JSON viewer. Pretty-prints with 2-space indent and tokenises into `key`/`string`/`number`/`boolean`/`null`/`punct` spans coloured with the AGH signal palette. Use for inspector payloads and wire-card bodies.",
      },
    },
  },
  decorators: [
    Story => (
      <div className="w-[640px] bg-background p-4">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Realistic agh-network/v0 receipt payload covering all token kinds.
 */
export const Receipt: Story = {
  args: {},
  render: () => (
    <JsonViewer
      value={{
        kind: "receipt",
        from: "agh://workspace/personal",
        to: "agh://agent/anthropic",
        ref: "msg_5f3a91",
        delivered: true,
        attempt: 1,
        latencyMs: 87,
        cause: null,
      }}
    />
  ),
};

/**
 * Tight scalar — pure number/boolean/null example.
 */
export const Scalars: Story = {
  args: {},
  render: () => <JsonViewer value={{ ready: true, queue: 0, lastError: null, cpu: 0.42 }} />,
};
