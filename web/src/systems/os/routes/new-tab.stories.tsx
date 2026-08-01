import type { Meta, StoryObj } from "@storybook/react-vite";

import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/os/routes/NewTab",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "The empty tab created by ⌘T / the deck's `+` (US-005): a real `/new-tab` route whose window invites choosing a destination through the palette.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Empty tab window: neutral "New tab" identity and the destination-picker CTA. */
export const Default: Story = {
  args: {},
  parameters: appRouteParameters("/new-tab"),
  render: () => <StorybookWorkspaceSetup />,
};
