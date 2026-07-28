import type { Meta, StoryObj } from "@storybook/react-vite";

import { BlockLoading } from "../block-loading";

const meta: Meta<typeof BlockLoading> = {
  title: "components/custom/BlockLoading",
  component: BlockLoading,
  parameters: {
    layout: "padded",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Panel: Story = {
  args: {
    label: "Loading vault metadata",
  },
};

export const BareSmall: Story = {
  args: {
    label: "Loading session secrets",
    size: "sm",
    surface: "bare",
  },
};
