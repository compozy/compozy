import type { Meta, StoryObj } from "@storybook/react-vite";

import { appRouteParameters } from "@/storybook/route-story-meta";
import { StorybookRouteCanvas, StorybookWorkspaceSetup } from "@/storybook/route-story";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/settings/routes/SettingsDefaults",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component: "Profile-owned defaults used when CompozyOS starts new sessions.",
      },
    },
  },
};

export default meta;

type Story = StoryObj<typeof meta>;

/** The permanent profile's effective session defaults. */
export const Default: Story = {
  args: {},
  parameters: appRouteParameters("/settings/defaults"),
  render: () => <StorybookWorkspaceSetup />,
};
