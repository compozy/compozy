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
    "routes/reviews/detail",
    "Review-issue detail stories for success, loading, and missing/error states."
  ),
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Success: Story = {
  args: {},
  parameters: appRouteParameters("/reviews/alpha/2/issue_004"),
  render: () => <StorybookWorkspaceSetup />,
};

export const Loading: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/reviews/alpha/2/issue_004"),
    ...storybookMswParameters({
      reviews: [
        http.get("/api/reviews/:slug/rounds/:round/issues/:issue_id", async () => {
          await delay("infinite");
          return HttpResponse.json({});
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const Error: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/reviews/alpha/2/issue_004"),
    ...storybookMswParameters({
      reviews: [
        http.get("/api/reviews/:slug/rounds/:round/issues/:issue_id", () =>
          HttpResponse.json(
            {
              code: "review_issue_missing",
              message: "Review issue missing",
            },
            { status: 404 }
          )
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
