import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, http, HttpResponse } from "msw";

import { storybookMswParameters } from "@/storybook/msw";
import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";
import { DEFAULT_ACCOUNT_STATUS } from "@/systems/account/mocks/handlers";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/settings/routes/SettingsAccount",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Settings → Account route stories rendered through the real app shell, covering Pro, Free, signed-out, loading, and error states.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {},
  parameters: appRouteParameters("/settings/account"),
  render: () => <StorybookWorkspaceSetup />,
};

export const FreePlan: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/settings/account"),
    ...storybookMswParameters({
      settings: [
        http.get("/api/account", () =>
          HttpResponse.json({
            ...DEFAULT_ACCOUNT_STATUS,
            plan: "free",
            remote_computers_limit: 1,
            cloud_routes_limit: 0,
            subscription: null,
          })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const Loading: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/settings/account"),
    ...storybookMswParameters({
      settings: [
        http.get("/api/account", async () => {
          await delay("infinite");
          return HttpResponse.json(DEFAULT_ACCOUNT_STATUS);
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const Error: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/settings/account"),
    ...storybookMswParameters({
      settings: [
        http.get("/api/account", () =>
          HttpResponse.json({ error: "Failed to load account" }, { status: 500 })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
