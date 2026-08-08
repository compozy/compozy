import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, HttpResponse } from "msw";

import { compozyApiMock } from "@/storybook/openapi-msw";
import { storybookMswParameters } from "@/storybook/msw";
import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";
import { extensionFixtures } from "@/systems/extensions/mocks";
import { marketplaceListings, marketplaceSearchFixture } from "@/systems/marketplace/mocks";

const updatableExtensionListing = {
  ...marketplaceListings.extension[0]!,
  installed: true,
  installed_name: "otel-bridge",
  installed_version: "0.5.2",
  update_available: true,
  version: "0.7.0",
};

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/marketplace/routes/Marketplace",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Unified marketplace kind pages: RouteNav kinds, Installed|Marketplace scope, cards-only grids.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const SkillsMarketplace: Story = {
  args: {},
  parameters: appRouteParameters("/marketplace/skills?tab=market"),
  render: () => <StorybookWorkspaceSetup />,
};

export const SkillsInstalled: Story = {
  args: {},
  parameters: appRouteParameters("/marketplace/skills"),
  render: () => <StorybookWorkspaceSetup />,
};

export const SkillsLoading: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/marketplace/skills?tab=market"),
    ...storybookMswParameters({
      marketplace: [
        compozyApiMock.get("/api/marketplace/{kind}", async () => {
          await delay("infinite");
          return HttpResponse.json(marketplaceSearchFixture.kinds[0]);
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const McpsMarketplace: Story = {
  args: {},
  parameters: appRouteParameters("/marketplace/mcps?tab=market"),
  render: () => <StorybookWorkspaceSetup />,
};

export const McpsInstalled: Story = {
  args: {},
  parameters: appRouteParameters("/marketplace/mcps"),
  render: () => <StorybookWorkspaceSetup />,
};

export const ExtensionsMarketplace: Story = {
  args: {},
  parameters: appRouteParameters("/marketplace/extensions?tab=market"),
  render: () => <StorybookWorkspaceSetup />,
};

export const ExtensionsInstalled: Story = {
  args: {},
  parameters: appRouteParameters("/marketplace/extensions"),
  render: () => <StorybookWorkspaceSetup />,
};

/** A retired or unknown Marketplace kind reaches the OS not-found boundary without an alias. */
export const RetiredKindNotFound: Story = {
  args: {},
  parameters: appRouteParameters("/marketplace/bundles"),
  render: () => <StorybookWorkspaceSetup />,
};

/** Landing count and card affordance for a catalog entry the daemon reports as updatable. */
export const ExtensionsUpdateAvailable: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/marketplace/extensions?tab=market"),
    ...storybookMswParameters({
      marketplace: [
        compozyApiMock.get("/api/marketplace/{kind}", () =>
          HttpResponse.json({
            items: [updatableExtensionListing],
            kind: "extension",
            stale: false,
            total: 1,
          })
        ),
      ],
      extensions: [
        compozyApiMock.get("/api/extensions", () =>
          HttpResponse.json({
            extensions: [
              {
                ...extensionFixtures[0]!,
                marketplace: updatableExtensionListing,
                remote_version: "0.7.0",
                update_available: true,
              },
            ],
          })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const SkillsError: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/marketplace/skills?tab=market"),
    ...storybookMswParameters({
      marketplace: [
        compozyApiMock.get("/api/marketplace/{kind}", () =>
          HttpResponse.json({ error: "unreachable" }, { status: 503 })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const SkillsEmpty: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/marketplace/skills?tab=market"),
    ...storybookMswParameters({
      marketplace: [
        compozyApiMock.get("/api/marketplace/{kind}", () =>
          HttpResponse.json({ items: [], kind: "skill", stale: false, total: 0 })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
