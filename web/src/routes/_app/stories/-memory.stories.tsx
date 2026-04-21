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
    "routes/memory",
    "Workflow memory index stories for success, loading, empty, and error states."
  ),
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Success: Story = {
  args: {},
  parameters: appRouteParameters("/memory"),
  render: () => <StorybookWorkspaceSetup />,
};

export const Loading: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/memory"),
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
    ...appRouteParameters("/memory"),
    ...storybookMswParameters({
      workflows: [http.get("/api/tasks", () => HttpResponse.json({ workflows: [] }))],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const Error: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/memory"),
    ...storybookMswParameters({
      workflows: [
        http.get("/api/tasks", () =>
          HttpResponse.json(
            {
              code: "memory_index_failed",
              message: "Workflow memory index unavailable",
            },
            { status: 500 }
          )
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
