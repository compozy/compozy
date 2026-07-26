import type { Meta, StoryObj } from "@storybook/react-vite";

import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/settings/routes/SettingsShell",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Settings root route redirecting to the default General section inside the real app shell.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Settings shell route before the child index redirect resolves.
 */
export const Shell: Story = {
  args: {},
  parameters: appRouteParameters("/settings"),
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Settings index route redirecting to the default General section.
 */
export const IndexRedirect: Story = {
  args: {},
  parameters: appRouteParameters("/settings/"),
  render: () => <StorybookWorkspaceSetup />,
};
