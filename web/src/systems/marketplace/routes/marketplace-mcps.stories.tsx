import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, within } from "storybook/test";

import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";
import { mcpManagementServerFixtures } from "@/systems/settings/mocks";

import { defaultCatalog, defaultExtensions } from "./marketplace-route-story-fixtures";
import { marketplaceStoryHandlers } from "./marketplace-story-data";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/marketplace/routes/MarketplaceMcps",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "The marketplace MCPs view: managed MCP servers with status and sign-in per row, and the shared server editor.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Plugins | MCPs → MCPs: the managed MCP server list with status and sign-in per row. */
export const McpServers: Story = {
  parameters: {
    ...appRouteParameters("/marketplace?kind=mcps"),
    ...marketplaceStoryHandlers({
      catalog: defaultCatalog,
      extensions: defaultExtensions,
      mcpServers: mcpManagementServerFixtures,
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** `?q=` survives the kind switch and narrows the MCP server list by name. */
export const McpServersSearch: Story = {
  parameters: {
    ...appRouteParameters("/marketplace?kind=mcps&q=git"),
    ...marketplaceStoryHandlers({
      catalog: defaultCatalog,
      extensions: defaultExtensions,
      mcpServers: mcpManagementServerFixtures,
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** Add MCP server opens the shared MCP server editor over the MCPs view. */
export const McpServersAddServer: Story = {
  parameters: {
    ...appRouteParameters("/marketplace?kind=mcps"),
    ...marketplaceStoryHandlers({ catalog: defaultCatalog, extensions: defaultExtensions }),
  },
  render: () => <StorybookWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId("marketplace-mcp-create"));
    const body = within(document.body);
    await expect(body.findByTestId("settings-mcp-servers-editor")).resolves.toBeDefined();
  },
};
