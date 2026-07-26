import type { Meta, StoryObj } from "@storybook/react-vite";

import { KindChip } from "../kind-chip";

const meta: Meta<typeof KindChip> = {
  title: "components/custom/KindChip",
  component: KindChip,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Protocol kind marker — transparent surface, neutral border, mono UPPERCASE label, leading colored dot keyed off the protocol-kind registry (`--color-kind-say|greet|direct|receipt|capability|trace|whois`). Unknown kinds render without a dot unless `dotColor` is explicit.",
      },
    },
  },
  decorators: [
    Story => (
      <div className="flex flex-wrap items-center gap-2 bg-background p-4">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * All six canonical agh-network/v0 protocol kinds.
 */
export const ProtocolKinds: Story = {
  args: {},
  render: () => (
    <>
      <KindChip kind="greet" />
      <KindChip kind="whois" />
      <KindChip kind="say" />
      <KindChip kind="direct" />
      <KindChip kind="capability" />
      <KindChip kind="receipt" />
      <KindChip kind="trace" />
    </>
  ),
};

/**
 * Custom label and explicit dotColor for an unknown kind.
 */
export const CustomKind: Story = {
  args: {},
  render: () => (
    <>
      <KindChip kind="ack" label="ACK" dotColor="var(--color-success)" />
      <KindChip kind="lifecycle" />
    </>
  ),
};
