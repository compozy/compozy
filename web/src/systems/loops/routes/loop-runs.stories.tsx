import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, HttpResponse } from "msw";

import { aghApiMock } from "@/storybook/openapi-msw";
import { storybookMswParameters } from "@/storybook/msw";
import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";
import { loopRunAggregatesFixture } from "@/systems/loops/mocks";

const runningLoopRunRoute = "/loop-runs/looprun_running";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/loops/routes/LoopRuns",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Full app-shell route stories for the Loop runs list and a live run detail page.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const RunsList: Story = {
  args: {},
  parameters: appRouteParameters("/loop-runs"),
  render: () => <StorybookWorkspaceSetup />,
};

export const RunDetail: Story = {
  args: {},
  parameters: appRouteParameters(runningLoopRunRoute),
  render: () => <StorybookWorkspaceSetup />,
};

export const EmptyRuns: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/loop-runs"),
    ...storybookMswParameters({
      loops: [
        aghApiMock.get("/api/workspaces/{workspace_id}/loop-runs", () =>
          HttpResponse.json({ runs: [], aggregates: { ...loopRunAggregatesFixture, total: 0 } })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const LoadingRuns: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/loop-runs"),
    ...storybookMswParameters({
      loops: [
        aghApiMock.get("/api/workspaces/{workspace_id}/loop-runs", async () => {
          await delay("infinite");
          return HttpResponse.json({
            runs: [],
            aggregates: { ...loopRunAggregatesFixture, total: 0 },
          });
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
