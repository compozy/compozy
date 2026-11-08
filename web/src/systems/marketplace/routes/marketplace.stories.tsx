import { Toaster } from "@compozy/ui";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, within } from "storybook/test";

import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";

import { marketplaceStoryHandlers, storyCatalog, storySources } from "./marketplace-story-data";
import { installedExtension, installedListing } from "../mocks/installed";
import { storyGithubServer, storyPostgresInputs } from "./marketplace-story-servers";

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
  actionStatesCatalog,
  actionStatesExtensions,
  multiSourceCatalog,
  multiSourceSources,
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

/** Default landing: installed shelf → Featured → one section per non-empty category. */
export const Default: Story = {
  parameters: {
    ...appRouteParameters("/marketplace"),
    ...marketplaceStoryHandlers({ catalog: defaultCatalog, extensions: defaultExtensions }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** `?q=` replaces the sections with flat results, each naming its category; the shelf stays. */
export const Search: Story = {
  parameters: {
    ...appRouteParameters("/marketplace?q=re"),
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
      catalog: [storyCatalog.repositoryOrientation, storyCatalog.grafana, storyCatalog.github],
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

/** Add ▾ holds the two install doors and, after a rule, the plugin-marketplace and MCP server doors. */
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
    await expect(menu.findByTestId("marketplace-add-mcp-server")).resolves.toBeDefined();
  },
};

/** Three sources still read as categories: Browse never names a source as a heading. */
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

/**
 * `team-plugins` failed its last refresh: the stale line names it, and the plugins it last read
 * stay in their categories. The blocked package stays visibly blocked.
 */
export const DegradedSource: Story = {
  parameters: {
    ...appRouteParameters("/marketplace"),
    ...marketplaceStoryHandlers({
      catalog: multiSourceCatalog,
      catalogOverrides: { stale: true },
      extensions: defaultExtensions,
      sources: Object.values(storySources),
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** View all on a category: every entry, daemon-paged, with a search scoped to it. */
export const CategoryPage: Story = {
  parameters: {
    ...appRouteParameters("/marketplace/categories/development"),
    ...marketplaceStoryHandlers({
      catalog: multiSourceCatalog,
      extensions: defaultExtensions,
      sources: multiSourceSources,
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** A search inside a category that matches nothing names the category. */
export const CategorySearchEmpty: Story = {
  parameters: {
    ...appRouteParameters("/marketplace/categories/development?q=tailscale"),
    ...marketplaceStoryHandlers({ catalog: multiSourceCatalog, extensions: defaultExtensions }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** An id the daemon does not know: not-found with the way back to Marketplace. */
export const CategoryNotFound: Story = {
  parameters: {
    ...appRouteParameters("/marketplace/categories/apps"),
    ...marketplaceStoryHandlers({ catalog: multiSourceCatalog, extensions: defaultExtensions }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** One action per row, read from the installed inventory: Add · Set up · Ready · Turn on. */
export const ActionStates: Story = {
  parameters: {
    ...appRouteParameters("/marketplace"),
    ...marketplaceStoryHandlers({
      catalog: actionStatesCatalog,
      extensions: actionStatesExtensions,
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** Add installed the plugin but could not turn it on: the cause stays on the row with Retry. */
export const EnableFailed: Story = {
  parameters: {
    ...appRouteParameters("/marketplace"),
    ...marketplaceStoryHandlers({
      catalog: [storyCatalog.grafana],
      extensions: [],
      installEnableError: { code: "extension_start_failed", error: "grafana-mcp exited (code 1)" },
      installResult: installedExtension(installedListing(storyCatalog.grafana), { enabled: false }),
    }),
  },
  render: () => (
    <>
      <StorybookWorkspaceSetup />
      <Toaster />
    </>
  ),
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByRole("button", { name: "Add Grafana" }));
    await expect(
      canvas.findByRole("button", { name: "Retry turning on Grafana" }, { timeout: 5_000 })
    ).resolves.toBeDefined();
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
      catalog: logoLadderCatalog,
      extensions: [],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** Every catalog trail state on one screen: Add · Update · Ready · unverified · Blocked. */
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

/** Add in flight: the row dims and the trail reads Adding… until the daemon answers. */
export const Installing: Story = {
  parameters: {
    ...appRouteParameters("/marketplace"),
    ...marketplaceStoryHandlers({
      catalog: [storyCatalog.grafana, storyCatalog.github],
      extensions: [],
      installDelayMs: 60_000,
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByRole("button", { name: "Add Grafana" }));
    await expect(canvas.findByRole("button", { name: "Adding Grafana" })).resolves.toBeDefined();
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
      pendingUpdates: ["grafana"],
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
