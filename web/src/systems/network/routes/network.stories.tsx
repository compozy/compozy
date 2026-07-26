import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, HttpResponse } from "msw";
import { aghApiMock } from "@/storybook/openapi-msw";

import { storyDefaultWorkspaceId, storyHeroNetworkChannel } from "@/storybook/fintech-scenario";
import { storybookMswParameters } from "@/storybook/msw";
import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";
import { networkStatusFixture } from "@/systems/network/mocks";

const storybookNetworkStatus = networkStatusFixture;

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/network/routes/Network",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Real-shell stories for the channel-pivot network route, covering the threads tab, the directs tab, the activity tab, and MSW-backed empty / loading / disabled branches.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Network index route, which resolves to the active workspace and channel view.
 */
export const Overview: Story = {
  args: {},
  parameters: appRouteParameters("/network"),
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Default network route on the threads tab for the hero channel.
 */
export const ThreadsTab: Story = {
  args: {},
  parameters: appRouteParameters(
    `/network/${storyDefaultWorkspaceId}/${storyHeroNetworkChannel}/threads`
  ),
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Direct rooms tab for the hero channel.
 */
export const DirectsTab: Story = {
  args: {},
  parameters: appRouteParameters(
    `/network/${storyDefaultWorkspaceId}/${storyHeroNetworkChannel}/directs`
  ),
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Activity tab for the hero channel , read-only cross-surface feed.
 */
export const ActivityTab: Story = {
  args: {},
  parameters: appRouteParameters(
    `/network/${storyDefaultWorkspaceId}/${storyHeroNetworkChannel}/activity`
  ),
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Empty network branch when no channels have materialized yet.
 */
export const EmptyChannels: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/network"),
    ...storybookMswParameters({
      network: [
        aghApiMock.get("/api/workspaces/{workspace_id}/network/channels", () =>
          HttpResponse.json({ channels: [] })
        ),
        aghApiMock.get("/api/workspaces/{workspace_id}/network/peers", () =>
          HttpResponse.json({ peers: [] })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Disabled network branch shown when the daemon reports network support off.
 */
export const Disabled: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/network"),
    ...storybookMswParameters({
      network: [
        aghApiMock.get("/api/network/status", () =>
          HttpResponse.json({
            network: {
              ...storybookNetworkStatus,
              channels: 0,
              enabled: false,
              local_peers: 0,
              status: "disabled",
            },
          })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * First paint while the network status request is still pending.
 */
export const Loading: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/network"),
    ...storybookMswParameters({
      network: [
        aghApiMock.get("/api/network/status", async () => {
          await delay("infinite");
          return HttpResponse.json({ network: storybookNetworkStatus });
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
