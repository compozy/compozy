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
  GRAPH_ENG_RUN_ID,
  graphEngPendingRequests,
  loopRunAggregatesFixture,
  pendingEnumAskRequest,
  releaseTrainRunDetail,
} from "@/systems/loops/mocks";
import { taskDashboardFixture } from "@/systems/tasks/mocks";
import { sessionFixtures } from "@/systems/session/mocks";
import type { AttentionNotification } from "@/systems/notifications";
import { primaryWorkspaceFixture } from "@/systems/workspace/mocks";
import { loopRequestLocationPath } from "@/systems/loops";

const runningLoopRunRoute = "/loop-runs/looprun_running";
const loopRunDiffRoute = `/loop-runs/${GRAPH_ENG_RUN_ID}/diff?generation=3&against_generation=2`;
const attentionRequestTargetRoute = loopRequestLocationPath({
  workspaceId: primaryWorkspaceFixture.id,
  runId: GRAPH_ENG_RUN_ID,
  nodeId: "confirm-rollout",
  itemIndex: 0,
});

const attentionNotifications: AttentionNotification[] = [
  ...sessionFixtures
    .filter(
      session => session.badge === "waiting-for-input" || session.badge === "waiting-for-auth"
    )
    .map(session => ({
      id: `notification:${session.id}`,
      kind: "session",
      source_id: session.id,
      workspace_id: session.workspace_id ?? "",
      workspace_label: session.workspace_id ?? "Global",
      title: session.name ?? session.id,
      detail: session.badge,
      badge: session.badge,
      agent_name: session.agent_name,
      occurred_at: session.attention_changed_at ?? session.updated_at,
      finished: false,
      item_index: 0,
      generation: 0,
      redacted: false,
    })),
  ...graphEngPendingRequests.map(request => ({
    id: `notification:${request.loop_run_id}:${request.node_id}:${request.generation}:${request.item_index}`,
    kind: "loop-request",
    source_id: `${primaryWorkspaceFixture.id}:${request.loop_run_id}:${request.node_id}:${request.item_index}`,
    workspace_id: primaryWorkspaceFixture.id,
    workspace_label: primaryWorkspaceFixture.name,
    title: request.node_id,
    detail: request.prompt,
    occurred_at: request.opened_at,
    finished: false,
    run_id: request.loop_run_id,
    node_id: request.node_id,
    item_index: request.item_index,
    generation: request.generation,
    loop_name: request.loop_name,
    request_kind: request.kind,
    redacted: false,
  })),
];

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

export const RunEnumRequest: Story = {
  args: {},
  parameters: {
    ...appRouteParameters(`/loop-runs/${GRAPH_ENG_RUN_ID}`),
    ...storybookMswParameters({
      loops: [
        compozyApiMock.get("/api/workspaces/{workspace_id}/loop-runs/{run_id}", () =>
          HttpResponse.json({ ...releaseTrainRunDetail, requests: [pendingEnumAskRequest] })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const AttentionRequests: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/loop-runs"),
    ...storybookMswParameters({
      settings: [
        compozyApiMock.get("/api/notifications/attention", () =>
          HttpResponse.json({
            snapshot: "story-loop-attention",
            total: attentionNotifications.length,
            needs_you: attentionNotifications.length,
            finished: 0,
            items: attentionNotifications,
          })
        ),
      ],
      workspace: [
        compozyApiMock.get("/api/workspaces", () =>
          HttpResponse.json({ workspaces: [primaryWorkspaceFixture] })
        ),
      ],
      session: [
        compozyApiMock.get("/api/sessions/attention-summary", () =>
          HttpResponse.json({
            needs_you: 2,
            finished: 0,
            by_workspace: [{ workspace_id: primaryWorkspaceFixture.id, needs_you: 2, finished: 0 }],
          })
        ),
      ],
      tasks: [
        compozyApiMock.get("/api/observe/tasks/dashboard", () =>
          HttpResponse.json({
            dashboard: {
              ...taskDashboardFixture,
              totals: { ...taskDashboardFixture.totals, awaiting_approval_tasks: 0 },
            },
          })
        ),
      ],
      loops: [
        compozyApiMock.get("/api/workspaces/{workspace_id}/loop-requests", ({ request }) => {
          const runId = new URL(request.url).searchParams.get("run_id");
          const items = runId
            ? graphEngPendingRequests.filter(entry => entry.loop_run_id === runId)
            : graphEngPendingRequests;
          return HttpResponse.json({
            items,
            aggregates: { pending: graphEngPendingRequests.length },
            next_cursor: "",
          });
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const AttentionRequestTarget: Story = {
  args: {},
  parameters: appRouteParameters(attentionRequestTargetRoute),
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
