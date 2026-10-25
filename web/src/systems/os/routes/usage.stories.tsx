import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, http } from "msw";

import { storybookMswParameters } from "@/storybook/msw";
import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";
import {
  claudeOkTarget,
  codexOkTarget,
  providerUsageHandlers,
  providerUsageStreamResponse,
  statusTargets,
  usageEnvelope,
} from "@/systems/provider-usage/mocks";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/os/routes/Usage",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "The Usage app window (S3): every target of the shell's profile expanded, a header Refresh, and the link to Settings › Provider usage — live from the same stream as the tray.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Claude, Codex, and a provider whose fix is configuration, all expanded. */
export const Default: Story = {
  parameters: {
    ...appRouteParameters("/usage"),
    ...storybookMswParameters({
      "provider-usage": providerUsageHandlers(() => {
        const now = Date.now();
        return usageEnvelope(
          [claudeOkTarget(now), codexOkTarget(now), statusTargets(now).noPlan],
          {},
          now
        );
      }),
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** Feature turned off while the window is open: the reason and the way back to settings. */
export const TurnedOff: Story = {
  parameters: {
    ...appRouteParameters("/usage"),
    ...storybookMswParameters({
      "provider-usage": providerUsageHandlers(() => usageEnvelope([], { enabled: false })),
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** The first read has not answered: the rows hold their place in the window, under the real header. */
export const Loading: Story = {
  parameters: {
    ...appRouteParameters("/usage"),
    ...storybookMswParameters({
      "provider-usage": [
        http.get("*/api/provider-usage/stream", () => providerUsageStreamResponse()),
        http.get("*/api/provider-usage", async () => {
          await delay("infinite");
          return new Response(null, { status: 204 });
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
