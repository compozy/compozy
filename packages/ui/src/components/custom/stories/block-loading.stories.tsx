import type { Meta, StoryObj } from "@storybook/react-vite";

import { BlockLoading } from "../block-loading";

const meta: Meta<typeof BlockLoading> = {
  title: "components/custom/BlockLoading",
  component: BlockLoading,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Placeholder for a block of content that is still loading. `label` is always the spinner's accessible name; `showLabel` also renders it as visible text for surfaces where a silent spinner leaves the reader guessing.",
      },
    },
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

export const WithVisibleLabel: Story = {
  args: {
    label: "Working…",
    showLabel: true,
  },
};
