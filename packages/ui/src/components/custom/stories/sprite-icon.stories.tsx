import type { Meta, StoryObj } from "@storybook/react-vite";
import spriteUrl from "lucide-static/sprite.svg?url";

import { SpriteIcon } from "../sprite-icon";

const meta: Meta<typeof SpriteIcon> = {
  title: "components/custom/SpriteIcon",
  component: SpriteIcon,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Renders one icon out of an external SVG sprite by symbol id. One cached sprite request serves every icon on the surface — no per-icon JS.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** One sprite request renders any Lucide slug, tinted by `currentColor`. */
export const Default: Story = {
  args: {
    spriteUrl,
    name: "rocket",
    className: "size-6 text-fg",
  },
};

/** Sizes and tints come from utilities, exactly like a bundled icon component. */
export const TintedRow: Story = {
  args: { spriteUrl, name: "rocket" },
  render: args => (
    <div className="flex items-center gap-3 text-info">
      <SpriteIcon {...args} name="compass" className="size-4" />
      <SpriteIcon {...args} name="megaphone" className="size-5" />
      <SpriteIcon {...args} name="banana" className="size-6" />
      <SpriteIcon {...args} name="gem" className="size-7" strokeWidth={1.5} />
    </div>
  ),
};
