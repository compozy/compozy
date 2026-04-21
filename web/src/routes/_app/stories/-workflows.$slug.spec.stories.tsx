import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, http, HttpResponse } from "msw";

import { partialWorkflowSpecFixture, workflowSpecFixture } from "@/systems/spec/mocks";
import { storybookMswParameters } from "@/storybook/msw";
import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
  createRouteStoryMeta,
} from "@/storybook/route-story";

const meta: Meta<typeof StorybookRouteCanvas> = {
  ...createRouteStoryMeta(
    "routes/spec",
    "Workflow spec stories for success, loading, partial-document fallback, and error states."
  ),
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Success: Story = {
  args: {},
  parameters: appRouteParameters("/workflows/alpha/spec"),
  render: () => <StorybookWorkspaceSetup />,
};

export const Loading: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/workflows/alpha/spec"),
    ...storybookMswParameters({
      spec: [
        http.get("/api/tasks/:slug/spec", async () => {
          await delay("infinite");
          return HttpResponse.json({ spec: workflowSpecFixture });
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const PartialDocuments: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/workflows/alpha/spec"),
    ...storybookMswParameters({
      spec: [
        http.get("/api/tasks/:slug/spec", () =>
          HttpResponse.json({ spec: partialWorkflowSpecFixture })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const Error: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/workflows/alpha/spec"),
    ...storybookMswParameters({
      spec: [
        http.get("/api/tasks/:slug/spec", () =>
          HttpResponse.json(
            {
              code: "workflow_spec_missing",
              message: "Workflow spec missing",
            },
            { status: 404 }
          )
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
