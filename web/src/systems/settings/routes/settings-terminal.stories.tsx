import type { Meta, StoryObj } from "@storybook/react-vite";

import { appRouteParameters } from "@/storybook/route-story-meta";
import { StorybookRouteCanvas, StorybookWorkspaceSetup } from "@/storybook/route-story";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/settings/routes/SettingsTerminal",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component: "Live terminal process, retention, recording, and capacity policy.",
      },
    },
  },
};

export default meta;

type Story = StoryObj<typeof meta>;

/** The ten `[terminal]` keys on their own Settings section. */
export const Default: Story = {
  args: {},
  parameters: appRouteParameters("/settings/terminal"),
  render: () => <StorybookWorkspaceSetup />,
};
