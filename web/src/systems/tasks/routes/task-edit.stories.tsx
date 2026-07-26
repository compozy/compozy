import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, HttpResponse } from "msw";
import { aghApiMock } from "@/storybook/openapi-msw";

import { storybookMswParameters } from "@/storybook/msw";
import { taskDetailFixture } from "@/systems/tasks/mocks";
import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/tasks/routes/TaskEdit",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Task edit-route stories rendered inside the persistent tasks shell, covering loaded, loading, and missing-task states.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Default edit route with the loaded task contract pre-filled in the editor surface.
 */
export const Default: Story = {
  args: {},
  parameters: appRouteParameters("/tasks/task_001/edit"),
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Loading branch while the task detail used to prefill the editor is still pending.
 */
export const Loading: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/tasks/task_001/edit"),
    ...storybookMswParameters({
      tasks: [
        aghApiMock.get("/api/tasks/{id}", async () => {
          await delay("infinite");
          return HttpResponse.json({ task: taskDetailFixture });
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Empty fallback shown when the task cannot be loaded for editing.
 */
export const MissingTask: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/tasks/task_missing/edit"),
    ...storybookMswParameters({
      tasks: [
        aghApiMock.get("/api/tasks/{id}", ({ params }) =>
          HttpResponse.json({ error: `Task not found: ${String(params.id)}` }, { status: 404 })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
