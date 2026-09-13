import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, within } from "storybook/test";

import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";

import {
  installedExtension,
  installedListing,
  localExtension,
  marketplaceStoryHandlers,
  storyCatalog,
} from "./marketplace-story-data";

const herdrInstalled = installedListing(storyCatalog.herdrBridge, {
  installedVersion: "0.3.2",
  updateAvailable: true,
});
const context7Installed = installedListing(storyCatalog.context7);
const githubInstalled = installedListing(storyCatalog.github);
const repositoryInstalled = installedListing(storyCatalog.repositoryOrientation);

/** Today's catalog with two installed rows, one of them updatable. */
const defaultCatalog = [
  storyCatalog.repositoryOrientation,
  storyCatalog.batuta,
  herdrInstalled,
  context7Installed,
  storyCatalog.github,
  storyCatalog.postgres,
  storyCatalog.slackNotify,
  storyCatalog.policyBlocked,
  storyCatalog.acmeTools,
];

const defaultExtensions = [
  installedExtension(herdrInstalled, {
    contents: { agents: 0, bridges: 1, hooks: 0, loops: 0, mcp_servers: 0, skills: 2 },
  }),
  installedExtension(context7Installed, {
    contents: { agents: 0, bridges: 0, hooks: 0, loops: 0, mcp_servers: 1, skills: 0 },
  }),
];

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/marketplace/routes/Marketplace",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "One marketplace surface: the whole catalog as row-cards with the installed shelf on top, search only in the strip, and the Installed drill-in with management on the row.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Default landing: one source → flat grid; installed rows read Installed in place. */
export const Default: Story = {
  parameters: {
    ...appRouteParameters("/marketplace"),
    ...marketplaceStoryHandlers({ catalog: defaultCatalog, extensions: defaultExtensions }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** `?q=` narrows the list; the shelf stays because it is not part of the results. */
export const Search: Story = {
  parameters: {
    ...appRouteParameters("/marketplace?q=herdr"),
    ...marketplaceStoryHandlers({ catalog: defaultCatalog, extensions: defaultExtensions }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** First page skeleton rows in the real grid; the shelf waits for the inventory. */
export const Loading: Story = {
  parameters: {
    ...appRouteParameters("/marketplace"),
    ...marketplaceStoryHandlers({ catalogDelay: "infinite", extensions: [] }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** The daemon serves the last projection it could load under one stale line. */
export const Stale: Story = {
  parameters: {
    ...appRouteParameters("/marketplace"),
    ...marketplaceStoryHandlers({
      catalog: defaultCatalog,
      catalogOverrides: {
        error: "catalog fetch: dial tcp: lookup raw.githubusercontent.com: no such host",
        error_class: "network",
        stale: true,
      },
      extensions: defaultExtensions,
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** No source answered and nothing is cached: the framed empty with its cause and Retry. */
export const Unreachable: Story = {
  parameters: {
    ...appRouteParameters("/marketplace"),
    ...marketplaceStoryHandlers({ catalogStatus: 503, extensions: defaultExtensions }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** Zero installed: the shelf is absent, not empty. */
export const NothingInstalled: Story = {
  parameters: {
    ...appRouteParameters("/marketplace"),
    ...marketplaceStoryHandlers({
      catalog: [storyCatalog.repositoryOrientation, storyCatalog.batuta, storyCatalog.github],
      extensions: [],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const QueryEmpty: Story = {
  parameters: {
    ...appRouteParameters("/marketplace?q=tailscale"),
    ...marketplaceStoryHandlers({ catalog: defaultCatalog, extensions: defaultExtensions }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const CatalogEmpty: Story = {
  parameters: {
    ...appRouteParameters("/marketplace"),
    ...marketplaceStoryHandlers({ catalog: [], extensions: [] }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** Add ▾ holds exactly the two install doors; both open the production install dialog. */
export const AddMenuOpen: Story = {
  parameters: {
    ...appRouteParameters("/marketplace"),
    ...marketplaceStoryHandlers({ catalog: defaultCatalog, extensions: defaultExtensions }),
  },
  render: () => <StorybookWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId("marketplace-add"));
    const menu = within(document.body);
    await expect(menu.findByTestId("marketplace-add-github")).resolves.toBeDefined();
    await expect(menu.findByTestId("marketplace-add-local")).resolves.toBeDefined();
  },
};

/** Under 960px of window width the grid is one column and the description may wrap. */
export const NarrowWindow: Story = {
  parameters: {
    ...appRouteParameters("/marketplace"),
    ...marketplaceStoryHandlers({ catalog: defaultCatalog, extensions: defaultExtensions }),
    viewport: { defaultViewport: "ipad" },
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** Logo ladder rungs side by side: feed icon (Context7) · brand (GitHub) · marble (the rest). */
export const LogoLadder: Story = {
  parameters: {
    ...appRouteParameters("/marketplace"),
    ...marketplaceStoryHandlers({
      catalog: [
        storyCatalog.context7,
        storyCatalog.github,
        storyCatalog.batuta,
        storyCatalog.postgres,
      ],
      extensions: [],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** Every catalog trail state on one screen: Install · Update · Installed · unverified · Blocked. */
export const TrailStates: Story = {
  parameters: {
    ...appRouteParameters("/marketplace"),
    ...marketplaceStoryHandlers({
      catalog: [
        storyCatalog.batuta,
        herdrInstalled,
        context7Installed,
        storyCatalog.slackNotify,
        storyCatalog.policyBlocked,
      ],
      extensions: [installedExtension(herdrInstalled), installedExtension(context7Installed)],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** Install in flight: the row dims and the trail reads Installing… until the daemon answers. */
export const Installing: Story = {
  parameters: {
    ...appRouteParameters("/marketplace"),
    ...marketplaceStoryHandlers({
      catalog: [storyCatalog.batuta, storyCatalog.github],
      extensions: [],
      installDelayMs: 60_000,
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByRole("button", { name: "Install Batuta" }));
    await expect(canvas.findByRole("button", { name: "Installing Batuta" })).resolves.toBeDefined();
  },
};

/** Installed drill-in: flat list, contents after the name, management on the row. */
export const Installed: Story = {
  parameters: {
    ...appRouteParameters("/marketplace/installed"),
    ...marketplaceStoryHandlers({
      catalog: defaultCatalog,
      extensions: [
        ...defaultExtensions,
        installedExtension(githubInstalled, {
          contents: { agents: 0, bridges: 0, hooks: 0, loops: 0, mcp_servers: 1, skills: 0 },
          enabled: false,
        }),
        installedExtension(repositoryInstalled, {
          contents: { agents: 0, bridges: 0, hooks: 0, loops: 0, mcp_servers: 0, skills: 1 },
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** Two updates waiting: the line names them and Update all runs the batch. */
export const InstalledUpdates: Story = {
  parameters: {
    ...appRouteParameters("/marketplace/installed"),
    ...marketplaceStoryHandlers({
      catalog: defaultCatalog,
      extensions: [
        installedExtension(herdrInstalled, {
          contents: { agents: 0, bridges: 1, hooks: 0, loops: 0, mcp_servers: 0, skills: 2 },
        }),
        installedExtension(
          installedListing(storyCatalog.batuta, {
            installedVersion: "0.4.0",
            updateAvailable: true,
          }),
          { contents: { agents: 0, bridges: 0, hooks: 0, loops: 2, mcp_servers: 0, skills: 1 } }
        ),
        installedExtension(context7Installed),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** A sideloaded extension with no catalog origin stays visible as itself, marble tile included. */
export const InstalledLocalEntry: Story = {
  parameters: {
    ...appRouteParameters("/marketplace/installed"),
    ...marketplaceStoryHandlers({
      catalog: defaultCatalog,
      extensions: [
        installedExtension(context7Installed),
        localExtension("ops-notes"),
        localExtension("acme-runbooks", { workspace_id: "ws_story_fintech" }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** Reachable by URL at zero: the teaching empty with the CLI hint and one door back to Browse. */
export const InstalledEmpty: Story = {
  parameters: {
    ...appRouteParameters("/marketplace/installed"),
    ...marketplaceStoryHandlers({ catalog: defaultCatalog, extensions: [] }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const InstalledQueryEmpty: Story = {
  parameters: {
    ...appRouteParameters("/marketplace/installed?q=tailscale"),
    ...marketplaceStoryHandlers({ catalog: defaultCatalog, extensions: defaultExtensions }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** Remove from the overflow: the production type-to-confirm dialog on the local installed name. */
export const InstalledRemoveConfirm: Story = {
  parameters: {
    ...appRouteParameters("/marketplace/installed"),
    ...marketplaceStoryHandlers({ catalog: defaultCatalog, extensions: defaultExtensions }),
  },
  render: () => <StorybookWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId("marketplace-installed-more-context7"));
    const body = within(document.body);
    await userEvent.click(await body.findByTestId("marketplace-installed-remove-context7"));
    await expect(body.findByTestId("remove-extension-dialog")).resolves.toBeDefined();
  },
};

/** Retired kind paths redirect to Browse for one release. */
export const RedirectSkills: Story = {
  parameters: {
    ...appRouteParameters("/marketplace/skills"),
    ...marketplaceStoryHandlers({ catalog: defaultCatalog, extensions: defaultExtensions }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const RedirectMcps: Story = {
  parameters: {
    ...appRouteParameters("/marketplace/mcps?tab=market"),
    ...marketplaceStoryHandlers({ catalog: defaultCatalog, extensions: defaultExtensions }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const RedirectExtensions: Story = {
  parameters: {
    ...appRouteParameters("/marketplace/extensions?q=herdr"),
    ...marketplaceStoryHandlers({ catalog: defaultCatalog, extensions: defaultExtensions }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** A retired kind detail path lands on the one-catalog entry with its installed identity. */
export const RedirectKindDetail: Story = {
  parameters: {
    ...appRouteParameters("/marketplace/extension/herdr-bridge?installed_name=herdr-bridge"),
    ...marketplaceStoryHandlers({ catalog: defaultCatalog, extensions: defaultExtensions }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
