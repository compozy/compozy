import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, HttpResponse } from "msw";

import { compozyApiMock } from "@/storybook/openapi-msw";
import { storybookMswParameters } from "@/storybook/msw";
import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";
import { GRAPH_ENG_RUN_ID, loopRunAggregatesFixture } from "@/systems/loops/mocks";

const runningLoopRunRoute = "/loop-runs/looprun_running";
const loopRunDiffRoute = `/loop-runs/${GRAPH_ENG_RUN_ID}/diff?generation=3&against_generation=2`;

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

export const RunRequests: Story = {
  args: {},
  parameters: appRouteParameters(`/loop-runs/${GRAPH_ENG_RUN_ID}`),
  render: () => <StorybookWorkspaceSetup />,
};

export const Diff: Story = {
  args: {},
  parameters: appRouteParameters(loopRunDiffRoute),
  render: () => <StorybookWorkspaceSetup />,
};

export const EmptyRuns: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/loop-runs"),
    ...storybookMswParameters({
      loops: [
        compozyApiMock.get("/api/workspaces/{workspace_id}/loop-runs", () =>
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
        compozyApiMock.get("/api/workspaces/{workspace_id}/loop-runs", async () => {
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
