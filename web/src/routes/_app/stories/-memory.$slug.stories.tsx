import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, http, HttpResponse } from "msw";

import { emptyWorkflowMemoryIndexFixture } from "@/systems/memory/mocks";
import { storybookMswParameters } from "@/storybook/msw";
import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
  createRouteStoryMeta,
} from "@/storybook/route-story";

const meta: Meta<typeof StorybookRouteCanvas> = {
  ...createRouteStoryMeta(
    "routes/memory/detail",
    "Workflow memory detail stories for success, index loading, empty index, document error, and index failure states."
  ),
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Success: Story = {
  args: {},
  parameters: appRouteParameters("/memory/alpha"),
  render: () => <StorybookWorkspaceSetup />,
};

export const Loading: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/memory/alpha"),
    ...storybookMswParameters({
      memory: [
        http.get("/api/tasks/:slug/memory", async () => {
          await delay("infinite");
          return HttpResponse.json({ memory: emptyWorkflowMemoryIndexFixture });
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const Empty: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/memory/alpha"),
    ...storybookMswParameters({
      memory: [
        http.get("/api/tasks/:slug/memory", () =>
          HttpResponse.json({ memory: emptyWorkflowMemoryIndexFixture })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const DocumentError: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/memory/alpha"),
    ...storybookMswParameters({
      memory: [
        http.get("/api/tasks/:slug/memory/files/:file_id", () =>
          HttpResponse.json(
            {
              code: "memory_file_missing",
              message: "Memory file missing",
            },
            { status: 404 }
          )
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const Error: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/memory/alpha"),
    ...storybookMswParameters({
      memory: [
        http.get("/api/tasks/:slug/memory", () =>
          HttpResponse.json(
            {
              code: "memory_index_missing",
              message: "Memory index missing",
            },
            { status: 404 }
          )
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
