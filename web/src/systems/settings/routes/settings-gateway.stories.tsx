import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, HttpResponse } from "msw";

import { compozyApiMock } from "@/storybook/openapi-msw";
import { storybookMswParameters } from "@/storybook/msw";
import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";
import {
  connectivityProviderFixture,
  gatewayStatusFixture,
  gatewayTierFixture,
} from "@/systems/gateway/mocks";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/settings/routes/SettingsGateway",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Gateway settings route stories covering the local-only default, a verified private overlay, a degraded provider, and the request boundary states.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Fresh install: nothing reachable beyond this machine. */
export const Default: Story = {
  args: {},
  parameters: appRouteParameters("/settings/gateway"),
  render: () => <StorybookWorkspaceSetup />,
};

/** A private overlay whose address passed challenge verification. */
export const PrivateOverlayReachable: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/settings/gateway"),
    ...storybookMswParameters({
      settings: [
        compozyApiMock.get("/api/gateway/status", () =>
          HttpResponse.json(
            gatewayStatusFixture(
              gatewayTierFixture({ tier: "private", observed: "up", verified: true })
            )
          )
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** A supervised provider failure: nothing is advertised while it recovers. */
export const ProviderDegraded: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/settings/gateway"),
    ...storybookMswParameters({
      settings: [
        compozyApiMock.get("/api/gateway/status", () =>
          HttpResponse.json(
            gatewayStatusFixture(
              gatewayTierFixture({
                tier: "private",
                observed: "degraded",
                verified: false,
                cause: "provider handshake failed",
              })
            )
          )
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** An installed connectivity provider that is ready to carry a tier. */
export const ProviderInstalled: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/settings/gateway"),
    ...storybookMswParameters({
      settings: [
        compozyApiMock.get("/api/extensions", () =>
          HttpResponse.json({ extensions: [connectivityProviderFixture()] })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** Loading state while gateway posture is still resolving. */
export const Loading: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/settings/gateway"),
    ...storybookMswParameters({
      settings: [
        compozyApiMock.get("/api/gateway/status", async () => {
          await delay("infinite");
          return HttpResponse.json(gatewayStatusFixture());
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** Error branch shown when the gateway status request fails. */
export const Error: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/settings/gateway"),
    ...storybookMswParameters({
      settings: [
        compozyApiMock.get("/api/gateway/status", () =>
          HttpResponse.json({ error: "Failed to load gateway status" }, { status: 500 })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
