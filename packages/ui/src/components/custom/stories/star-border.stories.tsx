import type { Meta, StoryObj } from "@storybook/react-vite";
import { MotionConfig } from "motion/react";

import { Button } from "../../button";
import { StarBorder } from "../star-border";

const meta: Meta<typeof StarBorder> = {
  title: "components/custom/StarBorder",
  component: StarBorder,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Rims one call-to-action control in the brand gradient and sends two soft glows along its top and bottom edges. The wrapped control keeps its fill, size, hit area and focus ring. `animated={false}` and reduced motion keep the static rim only.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const PILL = "rounded-pill bg-brand-tint px-3 text-brand-strong hover:bg-brand-tint-strong";

/** The glows travel the rim: top edge left to right, bottom edge right to left. */
export const Animated: Story = {
  render: () => (
    <StarBorder>
      <Button className={PILL} size="sm" variant="ghost">
        Update
      </Button>
    </StarBorder>
  ),
};

/** A present but busy control: the static rim, no motion. */
export const Static: Story = {
  render: () => (
    <StarBorder animated={false}>
      <Button className={`${PILL} disabled:text-brand-strong`} disabled size="sm" variant="ghost">
        Updating…
      </Button>
    </StarBorder>
  ),
};

/** Reduced motion: the same control shows the static gradient rim only. */
export const ReducedMotion: Story = {
  render: () => (
    <MotionConfig reducedMotion="always">
      <StarBorder>
        <Button className={PILL} size="sm" variant="ghost">
          Update
        </Button>
      </StarBorder>
    </MotionConfig>
  ),
};
