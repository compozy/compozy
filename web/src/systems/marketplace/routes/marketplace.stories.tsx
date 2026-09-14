import Avatar from "boring-avatars";
import { Toaster } from "@compozy/ui";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, mocked, userEvent, within } from "storybook/test";

import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";

import {
  installedExtension,
  installedListing,
  marketplaceStoryHandlers,
  storyCatalog,
  storyGithubServer,
  storyMarketplacePlugins,
  storyPostgresInputs,
  storySources,
} from "./marketplace-story-data";

import {
  conflictingCatalog,
  context7Installed,
  defaultCatalog,
  defaultExtensions,
  githubInstalled,
  herdrInstalled,
  installedAuthorizeExtensions,
  installedExtensions,
  installedLocalExtensions,
  installedPartialUpdateExtensions,
  installedServerExtensions,
  installedUpdateExtensions,
  logoLadderCatalog,
  trailStatesCatalog,
  multiSourceCatalog,
  multiSourceSources,
  zeroPluginSources,
} from "./marketplace-route-story-fixtures";

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

/** The installed shelf caps its logo stack at six and names the remaining three. */
export const FullInstalledShelf: Story = {
  parameters: {
    ...appRouteParameters("/marketplace"),
    ...marketplaceStoryHandlers({
      catalog: defaultCatalog,
      extensions: defaultCatalog.map(entry => installedExtension(installedListing(entry))),
    }),
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

/** Add ▾ holds the two install doors and, after a rule, the plugin-marketplace door. */
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
    await expect(menu.findByTestId("marketplace-add-marketplace")).resolves.toBeDefined();
  },
};

/** Three sources listing something → three collapsible sections with authoritative counts. */
export const MultiSource: Story = {
  parameters: {
    ...appRouteParameters("/marketplace"),
    ...marketplaceStoryHandlers({
      catalog: multiSourceCatalog,
      extensions: defaultExtensions,
      sources: multiSourceSources,
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** A query narrows every section; sections with no match disappear, the gist counts matches. */
export const MultiSourceSearch: Story = {
  parameters: {
    ...appRouteParameters("/marketplace?q=review"),
    ...marketplaceStoryHandlers({
      catalog: multiSourceCatalog,
      extensions: defaultExtensions,
      sources: multiSourceSources,
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** Client-layout and blocked packages stay visible together in their owning source. */
export const MultiSourcePackages: Story = {
  ...MultiSource,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const grid = await within(canvasElement).findByTestId("marketplace-grid-team-plugins");
    grid.scrollIntoView({ block: "center" });
  },
};

/**
 * `team-plugins` failed its last refresh: its section keeps the plugins it last read and the gist
 * names the last read and that it could not refresh. The blocked package stays visibly blocked.
 */
export const DegradedSource: Story = {
  parameters: {
    ...appRouteParameters("/marketplace"),
    ...marketplaceStoryHandlers({
      catalog: multiSourceCatalog,
      extensions: defaultExtensions,
      sources: Object.values(storySources),
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const gist = await within(canvasElement).findByTestId(
      "marketplace-section-team-plugins-degraded"
    );
    gist.scrollIntoView({ block: "center" });
  },
};

/** A registered marketplace whose document lists zero plugins earns no section. */
export const ZeroPluginSource: Story = {
  parameters: {
    ...appRouteParameters("/marketplace"),
    ...marketplaceStoryHandlers({
      catalog: [...defaultCatalog, storyMarketplacePlugins.featureDev],
      extensions: defaultExtensions,
      sources: zeroPluginSources,
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
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
      catalog: logoLadderCatalog,
      extensions: [],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** Exercise the production boundary when the external marble generator throws. */
export const LogoGeneratorFailure: Story = {
  ...LogoLadder,
  beforeEach: () => {
    mocked(Avatar).mockImplementation(() => {
      throw new Error("Storybook marble generator failure");
    });
    return () => {
      mocked(Avatar).mockReset();
    };
  },
};

/** Every catalog trail state on one screen: Install · Update · Installed · unverified · Blocked. */
export const TrailStates: Story = {
  parameters: {
    ...appRouteParameters("/marketplace"),
    ...marketplaceStoryHandlers({
      catalog: trailStatesCatalog,
      extensions: [installedExtension(herdrInstalled), installedExtension(context7Installed)],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** A source-qualified competing origin cannot replace the occupied installation. */
export const NameInUse: Story = {
  parameters: {
    ...appRouteParameters("/marketplace"),
    ...marketplaceStoryHandlers({
      catalog: conflictingCatalog,
      extensions: [],
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
      extensions: installedExtensions,
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Rows whose extension provides an MCP server carry the daemon's status word: Needs
 * configuration (missing inputs) wins, Needs authorization adds Authorize on the row, Running,
 * and Disabled. GitHub also shows its allocated runtime name because a manual `github` exists.
 */
export const InstalledServerStatus: Story = {
  parameters: {
    ...appRouteParameters("/marketplace/installed"),
    ...marketplaceStoryHandlers({
      catalog: defaultCatalog,
      extensions: installedServerExtensions,
      details: { postgres: { inputs: storyPostgresInputs } },
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** Authorize on the Installed row starts the owner-qualified login for the extension's own server. */
export const InstalledAuthorize: Story = {
  tags: ["play-fn"],
  parameters: {
    ...appRouteParameters("/marketplace/installed"),
    ...marketplaceStoryHandlers({
      catalog: defaultCatalog,
      extensions: installedAuthorizeExtensions,
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const page = within(canvasElement.ownerDocument.body);
    const authorize = await canvas.findByTestId("marketplace-installed-authorize-github");
    await expect(authorize).toBeEnabled();
    await userEvent.click(authorize);
    await page.findByTestId("settings-page-mcp-authorize-url");
  },
};

/** Two updates waiting: the line names them and Update all runs the batch. */
export const InstalledUpdates: Story = {
  parameters: {
    ...appRouteParameters("/marketplace/installed"),
    ...marketplaceStoryHandlers({
      catalog: defaultCatalog,
      updateDelay: "infinite",
      extensions: installedUpdateExtensions,
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** An actual batch request remains pending at its HTTP boundary. */
export const InstalledUpdatesInFlight: Story = {
  ...InstalledUpdates,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId("marketplace-update-all"));
    await expect(canvas.getByTestId("marketplace-update-all")).toBeDisabled();
    await expect(canvas.getByTestId("marketplace-updates-line")).toHaveTextContent("0 of 2 done");
  },
};

/** A sideloaded extension with no catalog origin stays visible as itself, marble tile included. */
export const InstalledLocalEntry: Story = {
  parameters: {
    ...appRouteParameters("/marketplace/installed"),
    ...marketplaceStoryHandlers({
      catalog: defaultCatalog,
      extensions: installedLocalExtensions,
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

/** Installation can finish while the returned MCP server still needs OAuth authorization. */
export const InstallNeedsAuthorization: Story = {
  parameters: {
    ...appRouteParameters("/marketplace"),
    ...marketplaceStoryHandlers({
      catalog: [storyCatalog.github],
      extensions: [],
      installResult: installedExtension(githubInstalled, {
        mcp_servers: [storyGithubServer({ status: "needs_authorization" })],
      }),
    }),
  },
  render: () => (
    <>
      <StorybookWorkspaceSetup />
      <Toaster
        expand
        position="top-right"
        visibleToasts={5}
        offset={{ top: "3rem", right: "0.75rem" }}
        duration={Infinity}
      />
    </>
  ),
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const page = within(canvasElement.ownerDocument.body);
    await userEvent.click(await canvas.findByTestId("marketplace-action-github"));
    await userEvent.click(await page.findByTestId("extension-install-summary-confirm"));
    await expect(page.findByText("Needs authorization before it can run")).resolves.toBeVisible();
  },
};

/** One scoped request has completed while the other is still in flight. */
export const InstalledUpdatesPartialProgress: Story = {
  parameters: {
    ...appRouteParameters("/marketplace/installed"),
    ...marketplaceStoryHandlers({
      catalog: defaultCatalog,
      pendingUpdates: ["batuta"],
      extensions: installedPartialUpdateExtensions,
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId("marketplace-update-all"));
    await expect(canvas.findByText("1 of 2 done")).resolves.toBeVisible();
    await expect(
      canvas.getByTestId("marketplace-installed-switch-herdr-bridge")
    ).not.toHaveAttribute("aria-disabled", "true");
  },
};
