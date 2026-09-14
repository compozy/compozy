import type { Meta, StoryObj } from "@storybook/react-vite";
import { HttpResponse } from "msw";
import { expect, userEvent, within } from "storybook/test";

import { compozyApiMock } from "@/storybook/openapi-msw";
import { storybookMswParameters } from "@/storybook/msw";
import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";
import { devExtensionFixture } from "@/systems/extensions/mocks";
import { marketplaceCatalogDetailFixture } from "@/systems/marketplace/mocks";

import {
  kitDetailHandlers,
  kitExtensionFixture,
  kitInventoryItems,
} from "./marketplace-detail-story-fixtures";
import {
  installedExtension,
  installedListing,
  marketplaceStoryHandlers,
  storyCatalog,
  storyGithubServer,
} from "./marketplace-story-data";

const slackNotifyDetail = marketplaceCatalogDetailFixture("slack-notify")!;

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/marketplace/routes/MarketplaceDetail",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Marketplace entry detail: the lede carries the entry logo, the extension body fills the page, and the rail holds short collapsible property cards.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Browse mode: provenance leads the body, Install is the one head action, cards in the rail. */
export const DetailExtensionBrowse: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/marketplace/slack-notify?source=compozy-catalog"),
    ...storybookMswParameters({
      marketplace: [
        compozyApiMock.get("/api/marketplace/entries/{entry_id}", () =>
          HttpResponse.json(slackNotifyDetail)
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** An MCP-backed entry, not installed: the Server card summarises the manifest, no status or actions. */
export const DetailExtensionServerBrowse: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/marketplace/github?source=compozy-catalog"),
    ...marketplaceStoryHandlers({
      catalog: [storyCatalog.github],
      details: {
        github: {
          contents: { agents: 0, bridges: 0, hooks: 0, loops: 0, mcp_servers: 1, skills: 0 },
          mcp_servers: [storyGithubServer()],
        },
      },
      extensions: [],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Installed beside a manual `github`: the Server card reads Needs authorization, shows the
 * allocated runtime name `github.github`, and offers Authorize and Edit configuration on the
 * extension's own definition (owner `extension:github`).
 */
export const DetailExtensionServerInstalled: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/marketplace/github?source=compozy-catalog&installed_name=github"),
    ...marketplaceStoryHandlers({
      catalog: [installedListing(storyCatalog.github)],
      details: {
        github: {
          contents: { agents: 0, bridges: 0, hooks: 0, loops: 0, mcp_servers: 1, skills: 0 },
          mcp_servers: [
            storyGithubServer({ runtime_name: "github.github", status: "needs_authorization" }),
          ],
        },
      },
      extensions: [
        installedExtension(installedListing(storyCatalog.github), {
          contents: { agents: 0, bridges: 0, hooks: 0, loops: 0, mcp_servers: 1, skills: 0 },
          mcp_servers: [
            storyGithubServer({ runtime_name: "github.github", status: "needs_authorization" }),
          ],
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await canvas.findByTestId("marketplace-extension-server-runtime-name-github");
    await canvas.findByTestId("marketplace-extension-server-authorize-github");
  },
};

export const DetailExtensionInstalled: Story = {
  args: {},
  parameters: {
    ...appRouteParameters(
      "/marketplace/slack-notify?source=compozy-catalog&installed_name=slack-notify"
    ),
    ...storybookMswParameters({
      marketplace: [
        compozyApiMock.get("/api/marketplace/entries/{entry_id}", () =>
          HttpResponse.json({
            ...slackNotifyDetail,
            entry: {
              ...slackNotifyDetail.entry,
              installed: true,
              installed_name: "slack-notify",
              installed_version: "1.1.4",
            },
          })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** Workspace dev overlay: four distinct labels, crash-loop counters, origin path, and the log ring. */
export const DetailExtensionDevOverlay: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/marketplace/ops-dev-extension?installed_name=ops-dev-extension"),
    ...storybookMswParameters({
      marketplace: [
        compozyApiMock.get("/api/marketplace/entries/{entry_id}", () =>
          HttpResponse.json({
            entry: {
              description: "Workspace dev build linked from a local generation.",
              digest_sha256: "a".repeat(64),
              entry_id: "ops-dev-extension",
              install_slug: "",
              installable: true,
              installed: true,
              installed_name: "ops-dev-extension",
              installed_version: "0.2.0-dev",
              name: "ops-dev-extension",
              source: "compozy-catalog",
              source_ref: "catalog:compozy",
              update_available: false,
            },
          })
        ),
      ],
      extensions: [
        compozyApiMock.get("/api/extensions", () =>
          HttpResponse.json({ extensions: [devExtensionFixture] })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const devBadge = await canvas.findByTestId("extension-dev-badge");
    await userEvent.click(await canvas.findByTestId("extension-logs-follow"));
    await expect(canvas.findByTestId("extension-logs-lines")).resolves.toHaveTextContent(
      "tool.provider registered: archive"
    );
    // Anchor the capture on the overlay badges rather than wherever the toggle left the scroll.
    devBadge.scrollIntoView({ block: "center" });
  },
};

/** Shipped-vs-live kit truth beside the bound-env presence and the declared network digest. */
export const DetailExtensionKitInventory: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/marketplace/dep-kit-ops?installed_name=dep-kit-ops"),
    ...kitDetailHandlers(),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** Declared profiles, active-profile enablement, and the placement matrix on the real detail page. */
export const DetailExtensionProfilesPlacement: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/marketplace/dep-kit-ops?installed_name=dep-kit-ops"),
    ...kitDetailHandlers(),
  },
  render: () => <StorybookWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByRole("button", { name: "Placement matrix" }));
    const matrix = await canvas.findByText("campaign-brief");
    matrix.scrollIntoView({ block: "center" });
  },
};

/** A declared profile with an unfilled credential ask carries the real needs-setup signal. */
export const DetailExtensionProfileNeedsSetup: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/marketplace/dep-kit-ops?installed_name=dep-kit-ops"),
    ...kitDetailHandlers(),
  },
  render: () => <StorybookWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const badge = await within(canvasElement).findByText("Needs setup");
    badge.scrollIntoView({ block: "center" });
  },
};

/** An absent profile leaves its placed resource dormant and offers the canonical create flow. */
export const DetailExtensionDormantPlacement: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/marketplace/dep-kit-ops?installed_name=dep-kit-ops"),
    ...kitDetailHandlers(),
  },
  render: () => <StorybookWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const dormant = await within(canvasElement).findByTestId("extension-dormant-studio");
    dormant.scrollIntoView({ block: "center" });
  },
};

/** Enabled kit with a catalog update pending: body carries the kit, rail carries management. */
export const DetailExtensionEnabledUpdate: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/marketplace/dep-kit-ops?installed_name=dep-kit-ops"),
    ...storybookMswParameters({
      marketplace: [
        compozyApiMock.get("/api/marketplace/entries/{entry_id}", () =>
          HttpResponse.json({
            entry: {
              description: "Dependency review agents, a weekly sweep, and a review board layout.",
              digest_sha256: "a".repeat(64),
              entry_id: "dep-kit-ops",
              install_slug: "compozy/dep-kit-ops",
              installable: true,
              installed: true,
              installed_name: "dep-kit-ops",
              installed_version: "1.0.0",
              manage_path: "/marketplace/installed",
              name: "dep-kit-ops",
              source: "compozy-catalog",
              source_ref: "catalog:compozy",
              update_available: true,
              version: "1.1.0",
            },
          })
        ),
      ],
      extensions: [
        compozyApiMock.get("/api/extensions", () =>
          HttpResponse.json({
            extensions: [
              {
                ...kitExtensionFixture,
                enabled: true,
                network_confirmation_required: false,
                remote_version: "1.1.0",
                update_available: true,
              },
            ],
          })
        ),
        compozyApiMock.get("/api/extensions/{name}/inventory", () =>
          HttpResponse.json({
            enabled: true,
            extension: "dep-kit-ops",
            format: "compozy",
            items: kitInventoryItems.map(item => ({ ...item, live: true })),
          })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** The daemon refuses an unratified Live participation change; one affordance carries the digest. */
export const DetailExtensionNetworkConfirm: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/marketplace/dep-kit-ops?installed_name=dep-kit-ops"),
    ...kitDetailHandlers(true),
  },
  render: () => <StorybookWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByRole("button", { name: "Update" }));
    const dialog = within(document.body);
    await expect(dialog.findByTestId("extension-network-confirm-dialog")).resolves.toBeDefined();
    await expect(
      dialog.findByTestId("extension-network-confirm-digest")
    ).resolves.toHaveTextContent("sha256:6f1c0a94d3b27e58");
  },
};
