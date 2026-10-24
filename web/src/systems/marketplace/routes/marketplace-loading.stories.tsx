import type { Meta, StoryObj } from "@storybook/react-vite";

import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";

import { marketplaceStoryHandlers } from "./marketplace-story-data";

import { defaultCatalog } from "./marketplace-route-story-fixtures";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/marketplace/routes/MarketplaceLoading",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Marketplace loading branches: the catalog and installed inventory reads in flight, with placeholder row-cards in the grid and the shelf's line held.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** First page skeleton rows in the real grid; the shelf waits for the inventory. */
export const Loading: Story = {
  parameters: {
    ...appRouteParameters("/marketplace"),
    ...marketplaceStoryHandlers({ catalogDelay: "infinite", extensions: [] }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** The inventory is still loading too: the shelf's line is held as well as the grid. */
export const LoadingWithShelf: Story = {
  parameters: {
    ...appRouteParameters("/marketplace"),
    ...marketplaceStoryHandlers({ catalogDelay: "infinite", extensionsDelay: "infinite" }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** The installed inventory never settles: placeholder row-cards in the Installed grid. */
export const InstalledLoading: Story = {
  parameters: {
    ...appRouteParameters("/marketplace/installed"),
    ...marketplaceStoryHandlers({ catalog: defaultCatalog, extensionsDelay: "infinite" }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
