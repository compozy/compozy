import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, http, HttpResponse } from "msw";

import { storybookMswParameters } from "@/storybook/msw";
import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
  createRouteStoryMeta,
} from "@/storybook/route-story";

const meta: Meta<typeof StorybookRouteCanvas> = {
  ...createRouteStoryMeta(
    "routes/workflows",
    "Workflow inventory stories for success, loading, empty, and error branches rendered through the real app shell."
  ),
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Success: Story = {
  args: {},
  parameters: appRouteParameters("/workflows"),
  render: () => <StorybookWorkspaceSetup />,
};

export const Loading: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/workflows"),
    ...storybookMswParameters({
      workflows: [
        http.get("/api/tasks", async () => {
          await delay("infinite");
          return HttpResponse.json({ workflows: [] });
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const Empty: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/workflows"),
    ...storybookMswParameters({
      workflows: [http.get("/api/tasks", () => HttpResponse.json({ workflows: [] }))],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const Error: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/workflows"),
    ...storybookMswParameters({
      workflows: [
        http.get("/api/tasks", () =>
          HttpResponse.json(
            {
              code: "workflow_inventory_failed",
              message: "Workflow inventory failed",
            },
            { status: 500 }
          )
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
