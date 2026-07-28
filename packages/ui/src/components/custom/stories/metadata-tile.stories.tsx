import type { Meta, StoryObj } from "@storybook/react-vite";
import { ClockIcon, FingerprintIcon, GitBranchIcon } from "lucide-react";

import { MetadataTile } from "../metadata-tile";

const meta: Meta<typeof MetadataTile> = {
  title: "components/custom/MetadataTile",
  component: MetadataTile,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Compact metadata tile with mono UPPERCASE eyebrow label, mono tabular-nums value, and an optional muted detail. Used in inspector panels and detail headers; flat on `--canvas-soft` with 1px `--line` ring.",
      },
    },
  },
  decorators: [
    Story => (
      <div className="grid w-[640px] grid-cols-3 gap-2 bg-background p-4">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * A row of operator metadata you'd see on a session detail header.
 */
export const Default: Story = {
  args: {},
  render: () => (
    <>
      <MetadataTile icon={FingerprintIcon} label="Session ID" value="sess_5f3a91" />
      <MetadataTile icon={ClockIcon} label="Started" value="04:21:15" detail="2026-05-10 UTC" />
      <MetadataTile
        icon={GitBranchIcon}
        label="Branch"
        value="redesign"
        detail="behind main by 2"
      />
    </>
  ),
};

/**
 * Detail line beneath the value (e.g. operator binding, age, source).
 */
export const WithDetail: Story = {
  args: {},
  render: () => <MetadataTile label="Provider home" value="~/.claude" detail="bound" />,
};
