import type { Meta, StoryObj } from "@storybook/react-vite";

import { StorybookRouteCanvas, appRouteParameters } from "@/storybook/route-story-meta";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/design-system/routes/DesignSystem",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Route-level `/design-system` story rendered through the generated TanStack route tree.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {},
  parameters: appRouteParameters("/design-system"),
};
