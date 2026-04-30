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
    "routes/reviews/round",
    "Review round detail stories covering issue inventory, loading, and issue loading errors."
  ),
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Success: Story = {
  args: {},
  parameters: appRouteParameters("/reviews/alpha/2"),
  render: () => <StorybookWorkspaceSetup />,
};

export const Loading: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/reviews/alpha/2"),
    ...storybookMswParameters({
      reviews: [
        http.get("/api/reviews/:slug/rounds/:round", async () => {
          await delay("infinite");
          return HttpResponse.json({});
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const IssueLoadError: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/reviews/alpha/2"),
    ...storybookMswParameters({
      reviews: [
        http.get("/api/reviews/:slug/rounds/:round/issues", () =>
          HttpResponse.json(
            {
              code: "review_issues_failed",
              message: "Failed to load issues for alpha",
            },
            { status: 500 }
          )
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
