import type { Meta, StoryObj } from "@storybook/react-vite";

import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/settings/routes/SettingsAppearance",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Appearance settings rendered through the daemon-authoritative app shell: this browser's theme (Light, Dark, System), the in-product reduce-motion preference, and desktop wallpaper.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Default appearance pane over the active desktop. */
export const Default: Story = {
  args: {},
  parameters: appRouteParameters("/settings/appearance"),
  render: () => <StorybookWorkspaceSetup />,
};
