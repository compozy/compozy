import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, http, HttpResponse } from "msw";

import { storybookMswParameters } from "@/storybook/msw";
import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";
import { DEFAULT_COMPUTERS_LIST } from "@/systems/computers/mocks/handlers";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/settings/routes/SettingsComputers",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Settings → Computers route stories rendered through the real app shell, including the retired /settings/gateway redirect, empty list, loading, and error states.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {},
  parameters: appRouteParameters("/settings/computers"),
  render: () => <StorybookWorkspaceSetup />,
};

export const RetiredGatewayRedirect: Story = {
  args: {},
  parameters: appRouteParameters("/settings/gateway"),
  render: () => <StorybookWorkspaceSetup />,
};

export const Empty: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/settings/computers"),
    ...storybookMswParameters({
      settings: [
        http.get("/api/computers", () =>
          HttpResponse.json({ local: { run_agents: true }, computers: [] })
        ),
        http.get("/api/computers/available", () => HttpResponse.json({ computers: [] })),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const Loading: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/settings/computers"),
    ...storybookMswParameters({
      settings: [
        http.get("/api/computers", async () => {
          await delay("infinite");
          return HttpResponse.json(DEFAULT_COMPUTERS_LIST);
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const Error: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/settings/computers"),
    ...storybookMswParameters({
      settings: [
        http.get("/api/computers", () =>
          HttpResponse.json({ error: "Failed to load computers" }, { status: 500 })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
