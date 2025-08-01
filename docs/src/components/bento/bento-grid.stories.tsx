import type { Meta, StoryObj } from "@storybook/react";
import CompozyBentoGrid from "./bento-grid";

const meta = {
  title: "Components/BentoGrid",
  component: CompozyBentoGrid,
  parameters: {
    layout: "fullscreen",
  },
} satisfies Meta<typeof CompozyBentoGrid>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
