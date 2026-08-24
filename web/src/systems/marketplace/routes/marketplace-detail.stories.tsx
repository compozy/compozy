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
import { marketplaceDetails } from "@/systems/marketplace/mocks";
import { mcpManagementCollectionFixture } from "@/systems/settings/mocks";
import { skillFixtures } from "@/systems/skill/mocks";

import {
  kitDetailHandlers,
  kitExtensionFixture,
  kitInventoryItems,
} from "./marketplace-detail-story-fixtures";

const gitFlowSkill = {
  ...skillFixtures[0]!,
  name: "git-flow",
  dir: "/opt/compozy/skills/git-flow",
  version: "1.4.2",
  metadata: {
    ...skillFixtures[0]!.metadata,
    capabilities: ["git.inspect", "git.review", "tests.verify"],
    recent_calls: [
      {
        label: "Review pull request",
        status: "success",
        timestamp: "2026-07-18T12:00:00Z",
      },
      { label: "Inspect release branch", status: "pending" },
    ],
  },
};

const inactiveGitFlowSkill = {
  ...gitFlowSkill,
  activation: {
    active: false,
    reasons: [
      {
        gate: "requires_tools",
        code: "missing_tool" as const,
        missing: ["compozy__browser_screenshot"],
        message: "gate requires_tools unmet: compozy__browser_screenshot",
      },
    ],
  },
};

const gitFlowShadows = {
  name: "git-flow",
  winner: {
    detected_at: "2026-04-17T16:41:00Z",
    path: "/opt/compozy/skills/git-flow/SKILL.md",
    resolved_to_winner: true,
    tier: "workspace",
  },
  shadows: [
    {
      detected_at: "2026-04-17T16:41:00Z",
      path: "/opt/compozy/skills/git-flow/SKILL.md",
      resolved_to_winner: true,
      tier: "workspace",
    },
    {
      detected_at: "2026-04-17T16:42:00Z",
      path: "/opt/compozy/marketplace/git-flow/SKILL.md",
      resolved_to_winner: false,
      tier: "marketplace",
    },
  ],
};

function detailSkillHandlers(disableFailure = false, skill = gitFlowSkill) {
  return storybookMswParameters({
    marketplace: [
      compozyApiMock.get("/api/skills/{name}", () => HttpResponse.json({ skill })),
      compozyApiMock.get("/api/skills/{name}/content", () =>
        HttpResponse.json({
          content:
            "# Git Flow\n\nBranch, review, and land changes using the repository's own checks as the gate.\n",
        })
      ),
      compozyApiMock.get("/api/skills/{name}/shadows", () => HttpResponse.json(gitFlowShadows)),
      compozyApiMock.post("/api/skills/{name}/enable", () => HttpResponse.json({ ok: true })),
      compozyApiMock.post("/api/skills/{name}/disable", () =>
        disableFailure
          ? HttpResponse.json({ error: "Skill policy rejected the update" }, { status: 409 })
          : HttpResponse.json({ ok: true })
      ),
    ],
  });
}

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/marketplace/routes/MarketplaceDetail",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Marketplace entry detail per kind: the kind-specific story fills the body and the rail holds short collapsible property cards.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Browse mode: readme as the hero, Install as the one head action, property cards in the rail. */
export const DetailSkillBrowse: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/marketplace/skill/git-flow"),
    ...storybookMswParameters({
      marketplace: [
        compozyApiMock.get("/api/marketplace/{kind}/{entry_id}", () =>
          HttpResponse.json({
            ...marketplaceDetails["skill:git-flow"],
            entry: {
              ...marketplaceDetails["skill:git-flow"]!.entry,
              installed: false,
              installed_name: undefined,
              installed_version: undefined,
              manage_path: undefined,
            },
          })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const DetailSkillInstalled: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/marketplace/skill/git-flow"),
    ...detailSkillHandlers(),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** Enabled skill withheld from agent prompts because a required tool is unavailable. */
export const DetailSkillInactive: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/marketplace/skill/git-flow"),
    ...detailSkillHandlers(false, inactiveGitFlowSkill),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** A rejected enablement mutation remains visible beside the owning switch. */
export const DetailSkillToggleError: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/marketplace/skill/git-flow"),
    ...detailSkillHandlers(true),
  },
  render: () => <StorybookWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId("skill-enabled-switch"));
    await expect(canvas.findByTestId("marketplace-skill-toggle-error")).resolves.toHaveTextContent(
      "Skill policy rejected the update"
    );
  },
};

export const DetailExtensionInstalled: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/marketplace/extension/slack-notify"),
    ...storybookMswParameters({
      marketplace: [
        compozyApiMock.get("/api/marketplace/{kind}/{entry_id}", () =>
          HttpResponse.json({
            ...marketplaceDetails["extension:slack-notify"],
            entry: {
              ...marketplaceDetails["extension:slack-notify"]!.entry,
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
    ...appRouteParameters("/marketplace/extension/ops-dev-extension"),
    ...storybookMswParameters({
      marketplace: [
        compozyApiMock.get("/api/marketplace/{kind}/{entry_id}", () =>
          HttpResponse.json({
            entry: {
              description: "Workspace dev build linked from a local generation.",
              entry_id: "ops-dev-extension",
              installed: true,
              installed_name: "ops-dev-extension",
              installed_version: "0.2.0-dev",
              kind: "extension",
              name: "ops-dev-extension",
              source: "dev",
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
    ...appRouteParameters("/marketplace/extension/dep-kit-ops"),
    ...kitDetailHandlers(),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** Declared profiles, active-profile enablement, and the placement matrix on the real detail page. */
export const DetailExtensionProfilesPlacement: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/marketplace/extension/dep-kit-ops"),
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
    ...appRouteParameters("/marketplace/extension/dep-kit-ops"),
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
    ...appRouteParameters("/marketplace/extension/dep-kit-ops"),
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
    ...appRouteParameters("/marketplace/extension/dep-kit-ops"),
    ...storybookMswParameters({
      marketplace: [
        compozyApiMock.get("/api/marketplace/{kind}/{entry_id}", () =>
          HttpResponse.json({
            entry: {
              description: "Dependency review agents, a weekly sweep, and a review board layout.",
              entry_id: "dep-kit-ops",
              installed: true,
              installed_name: "dep-kit-ops",
              installed_version: "1.0.0",
              kind: "extension",
              name: "dep-kit-ops",
              source: "registry",
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
    ...appRouteParameters("/marketplace/extension/dep-kit-ops"),
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

export const DetailMcpInstalled: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/marketplace/mcp/linear"),
    ...storybookMswParameters({
      marketplace: [
        compozyApiMock.get("/api/marketplace/{kind}/{entry_id}", () =>
          HttpResponse.json({
            ...marketplaceDetails["mcp:linear"],
            entry: {
              ...marketplaceDetails["mcp:linear"]!.entry,
              installed: true,
              installed_name: "linear",
              installed_version: "1.0.0",
            },
          })
        ),
        compozyApiMock.get("/api/settings/mcp-servers", () =>
          HttpResponse.json(mcpManagementCollectionFixture)
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
