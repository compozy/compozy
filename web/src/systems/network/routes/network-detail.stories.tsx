import type { Meta, StoryObj } from "@storybook/react-vite";
import { HttpResponse } from "msw";

import { aghApiMock } from "@/storybook/openapi-msw";
import { storyDefaultWorkspaceId, storyHeroNetworkChannel } from "@/storybook/fintech-scenario";
import { storybookMswParameters } from "@/storybook/msw";
import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";

const threadDetailRoute = `/network/${storyDefaultWorkspaceId}/${storyHeroNetworkChannel}/threads/thread_launch_command`;
const directDetailRoute = `/network/${storyDefaultWorkspaceId}/${storyHeroNetworkChannel}/directs/direct_story_launch_corridor`;

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/network/routes/NetworkDetail",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Full app-shell route stories for concrete network thread and direct detail routes.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const ThreadDetail: Story = {
  args: {},
  parameters: appRouteParameters(threadDetailRoute),
  render: () => <StorybookWorkspaceSetup />,
};

export const ThreadFullPage: Story = {
  args: {},
  parameters: appRouteParameters(`${threadDetailRoute}?view=full`),
  render: () => <StorybookWorkspaceSetup />,
};

export const DirectDetail: Story = {
  args: {},
  parameters: appRouteParameters(directDetailRoute),
  render: () => <StorybookWorkspaceSetup />,
};

export const ThreadNotFound: Story = {
  args: {},
  parameters: {
    ...appRouteParameters(
      `/network/${storyDefaultWorkspaceId}/${storyHeroNetworkChannel}/threads/thread_missing?view=full`
    ),
    ...storybookMswParameters({
      network: [
        aghApiMock.get(
          "/api/workspaces/{workspace_id}/network/channels/{channel}/threads/{thread_id}",
          ({ params }) =>
            HttpResponse.json(
              { error: `Thread not found: ${String(params.thread_id)}` },
              { status: 404 }
            )
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
