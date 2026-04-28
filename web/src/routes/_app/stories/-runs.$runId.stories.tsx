import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, http, HttpResponse } from "msw";

import type { RunStreamFactory } from "@/systems/runs";
import {
  completedRunSnapshotFixture,
  runSnapshotFixture,
  runTranscriptFixture,
} from "@/systems/runs/mocks";
import { storybookMswParameters } from "@/storybook/msw";
import {
  StorybookRouteCanvas,
  StorybookRunStreamSetup,
  StorybookWorkspaceSetup,
  appRouteParameters,
  createRouteStoryMeta,
} from "@/storybook/route-story";

const meta: Meta<typeof StorybookRouteCanvas> = {
  ...createRouteStoryMeta(
    "routes/runs/detail",
    "Run-detail stories for stable snapshots plus degraded stream overflow handling."
  ),
};

export default meta;
type Story = StoryObj<typeof meta>;

const overflowRunStreamFactory: RunStreamFactory = (_options, handler) => {
  queueMicrotask(() => {
    handler({ type: "open" });
    handler({
      type: "overflow",
      eventId: "2026-04-20T03:12:00Z|00000000000000000002",
      payload: { reason: "replay boundary exceeded" },
    });
  });

  return {
    close() {},
  };
};

export const Success: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/runs/run-task-02"),
    ...storybookMswParameters({
      runs: [
        http.get("/api/runs/:run_id/snapshot", () =>
          HttpResponse.json(completedRunSnapshotFixture)
        ),
        http.get("/api/runs/:run_id/transcript", () => HttpResponse.json(runTranscriptFixture)),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const Overflowed: Story = {
  args: {},
  parameters: appRouteParameters("/runs/run-task-02"),
  render: () => (
    <>
      <StorybookWorkspaceSetup />
      <StorybookRunStreamSetup factory={overflowRunStreamFactory} />
    </>
  ),
};

export const Loading: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/runs/run-task-02"),
    ...storybookMswParameters({
      runs: [
        http.get("/api/runs/:run_id/snapshot", async () => {
          await delay("infinite");
          return HttpResponse.json(runSnapshotFixture);
        }),
        http.get("/api/runs/:run_id/transcript", async () => {
          await delay("infinite");
          return HttpResponse.json(runTranscriptFixture);
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const Error: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/runs/run-task-02"),
    ...storybookMswParameters({
      runs: [
        http.get("/api/runs/:run_id/snapshot", () =>
          HttpResponse.json(
            {
              code: "run_snapshot_missing",
              message: "Run snapshot missing",
            },
            { status: 404 }
          )
        ),
        http.get("/api/runs/:run_id/transcript", () => HttpResponse.json(runTranscriptFixture)),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
