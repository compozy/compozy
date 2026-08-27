import type { Meta, StoryObj } from "@storybook/react-vite";

import { MetadataList } from "../metadata-list";

const meta: Meta<typeof MetadataList> = {
  title: "components/custom/MetadataList",
  component: MetadataList,
  parameters: {
    layout: "centered",
  },
  decorators: [
    Story => (
      <div className="w-[420px] rounded-md border border-line bg-canvas-soft p-4">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Shortcut: `label` injects Term + Value on the kv grid.
 */
export const LabeledRows: Story = {
  args: {},
  render: () => (
    <MetadataList>
      <MetadataList.Row label="caller">parent session</MetadataList.Row>
      <MetadataList.Row label="child">
        <span className="font-mono">ses_8f21</span>
      </MetadataList.Row>
      <MetadataList.Row label="depth">2</MetadataList.Row>
    </MetadataList>
  ),
};

export const Short: Story = {
  args: {},
  render: () => (
    <MetadataList>
      <MetadataList.Row>
        <MetadataList.Term>session</MetadataList.Term>
        <MetadataList.Value className="font-mono">sess_123</MetadataList.Value>
      </MetadataList.Row>
      <MetadataList.Row>
        <MetadataList.Term>agent</MetadataList.Term>
        <MetadataList.Value>Claude Code</MetadataList.Value>
      </MetadataList.Row>
    </MetadataList>
  ),
};

export const LongValues: Story = {
  args: {},
  render: () => (
    <MetadataList>
      <MetadataList.Row className="items-baseline justify-between">
        <MetadataList.Term>path</MetadataList.Term>
        <MetadataList.Value className="break-all text-right font-mono">
          /Users/operator/projects/compozy/.compozy/sessions/sess_123/ledger.json
        </MetadataList.Value>
      </MetadataList.Row>
      <MetadataList.Row className="items-baseline justify-between">
        <MetadataList.Term>checksum</MetadataList.Term>
        <MetadataList.Value className="break-all text-right font-mono">
          b8d8a62c9c2f4a6cb2a4ce7b4c95e9f7
        </MetadataList.Value>
      </MetadataList.Row>
    </MetadataList>
  ),
};
