import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, http, HttpResponse } from "msw";

import { taskDetailFixture } from "@/systems/workflows/mocks";
import { storybookMswParameters } from "@/storybook/msw";
import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
  createRouteStoryMeta,
} from "@/storybook/route-story";

const meta: Meta<typeof StorybookRouteCanvas> = {
  ...createRouteStoryMeta(
    "routes/workflows/task-detail",
    "Task-detail route stories for success, loading, and error cases."
  ),
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Success: Story = {
  args: {},
  parameters: appRouteParameters("/workflows/alpha/tasks/task_02"),
  render: () => <StorybookWorkspaceSetup />,
};

export const Loading: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/workflows/alpha/tasks/task_02"),
    ...storybookMswParameters({
      workflows: [
        http.get("/api/tasks/:slug/items/:task_id", async () => {
          await delay("infinite");
          return HttpResponse.json({ task: taskDetailFixture });
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const Error: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/workflows/alpha/tasks/task_02"),
    ...storybookMswParameters({
      workflows: [
        http.get("/api/tasks/:slug/items/:task_id", () =>
          HttpResponse.json(
            {
              code: "task_detail_missing",
              message: "Task detail missing",
            },
            { status: 404 }
          )
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
