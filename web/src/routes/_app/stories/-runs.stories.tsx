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
    "routes/runs",
    "Run inventory stories covering success, loading, empty, and error states."
  ),
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Success: Story = {
  args: {},
  parameters: appRouteParameters("/runs"),
  render: () => <StorybookWorkspaceSetup />,
};

export const Loading: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/runs"),
    ...storybookMswParameters({
      runs: [
        http.get("/api/runs", async () => {
          await delay("infinite");
          return HttpResponse.json({ runs: [] });
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const Empty: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/runs"),
    ...storybookMswParameters({
      runs: [http.get("/api/runs", () => HttpResponse.json({ runs: [] }))],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const Error: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/runs"),
    ...storybookMswParameters({
      runs: [
        http.get("/api/runs", () =>
          HttpResponse.json(
            {
              code: "runs_unavailable",
              message: "Run inventory unavailable",
            },
            { status: 500 }
          )
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
