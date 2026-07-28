import type { Meta, StoryObj } from "@storybook/react-vite";

import { DesignSystemShowcase } from "@/components/design-system-showcase";

const meta: Meta<typeof DesignSystemShowcase> = {
  title: "systems/design-system/components/DesignSystemShowcase",
  component: DesignSystemShowcase,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Full `/design-system` page shot: token swatch wall + nine primitive-group sections. Rendered without the authenticated app shell (the route lives outside `_app/`).",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * The default showcase state for the `/design-system` route outer frame.
 */
export const Default: Story = {
  args: {},
};
