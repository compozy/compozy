import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, http, HttpResponse } from "msw";

import { degradedDashboardFixture, emptyDashboardFixture } from "@/systems/dashboard/mocks";
import { storybookMswParameters } from "@/storybook/msw";
import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
  createRouteStoryMeta,
} from "@/storybook/route-story";

const meta: Meta<typeof StorybookRouteCanvas> = {
  ...createRouteStoryMeta(
    "routes/dashboard",
    "Real-shell route stories for the daemon dashboard, covering success, loading, empty, degraded, and error states."
  ),
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Success: Story = {
  args: {},
  parameters: appRouteParameters("/"),
  render: () => <StorybookWorkspaceSetup />,
};

export const Loading: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/"),
    ...storybookMswParameters({
      dashboard: [
        http.get("/api/ui/dashboard", async () => {
          await delay("infinite");
          return HttpResponse.json({ dashboard: emptyDashboardFixture });
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const Empty: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/"),
    ...storybookMswParameters({
      dashboard: [
        http.get("/api/ui/dashboard", () =>
          HttpResponse.json({ dashboard: emptyDashboardFixture })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const Degraded: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/"),
    ...storybookMswParameters({
      dashboard: [
        http.get("/api/ui/dashboard", () =>
          HttpResponse.json({ dashboard: degradedDashboardFixture })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const Error: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/"),
    ...storybookMswParameters({
      dashboard: [
        http.get("/api/ui/dashboard", () =>
          HttpResponse.json(
            {
              code: "dashboard_unavailable",
              message: "Dashboard unavailable",
            },
            { status: 500 }
          )
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
