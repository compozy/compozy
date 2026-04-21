import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, http, HttpResponse } from "msw";

import { emptyTaskBoardFixture, taskBoardFixture } from "@/systems/workflows/mocks";
import { storybookMswParameters } from "@/storybook/msw";
import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
  createRouteStoryMeta,
} from "@/storybook/route-story";

const meta: Meta<typeof StorybookRouteCanvas> = {
  ...createRouteStoryMeta(
    "routes/workflows/task-board",
    "Task-board stories for the workflow drill-down route, covering success, loading, empty, and error states."
  ),
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Success: Story = {
  args: {},
  parameters: appRouteParameters("/workflows/alpha/tasks"),
  render: () => <StorybookWorkspaceSetup />,
};

export const Loading: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/workflows/alpha/tasks"),
    ...storybookMswParameters({
      workflows: [
        http.get("/api/tasks/:slug/board", async () => {
          await delay("infinite");
          return HttpResponse.json({ board: taskBoardFixture });
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const Empty: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/workflows/alpha/tasks"),
    ...storybookMswParameters({
      workflows: [
        http.get("/api/tasks/:slug/board", () =>
          HttpResponse.json({ board: emptyTaskBoardFixture })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const Error: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/workflows/alpha/tasks"),
    ...storybookMswParameters({
      workflows: [
        http.get("/api/tasks/:slug/board", () =>
          HttpResponse.json(
            {
              code: "task_board_failed",
              message: "Task board unavailable",
            },
            { status: 500 }
          )
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
