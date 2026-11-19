import type { Meta, StoryObj } from "@storybook/react-vite";

import { appRouteParameters } from "@/storybook/route-story-meta";
import { StorybookRouteCanvas, StorybookWorkspaceSetup } from "@/storybook/route-story";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/settings/routes/SettingsShortcuts",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Every registry command with its shortcuts and aliases, plus the global hotkeys. Bindings are daemon-owned and apply as they are recorded, so the page has no save bar.",
      },
    },
  },
};

export default meta;

type Story = StoryObj<typeof StorybookRouteCanvas>;

/** The whole registry for the active workspace. */
export const Default: Story = {
  parameters: appRouteParameters("/settings/shortcuts"),
  render: () => <StorybookWorkspaceSetup />,
};

/** A "Set shortcut…" deep link lands on the named command's row. */
export const FocusedCommand: Story = {
  parameters: appRouteParameters("/settings/shortcuts?command=palette.open"),
  render: () => <StorybookWorkspaceSetup />,
};
