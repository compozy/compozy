import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, http, HttpResponse } from "msw";

import { reviewsDashboardFixture } from "@/systems/dashboard/mocks";
import { storybookMswParameters } from "@/storybook/msw";
import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
  createRouteStoryMeta,
} from "@/storybook/route-story";

const meta: Meta<typeof StorybookRouteCanvas> = {
  ...createRouteStoryMeta(
    "routes/reviews",
    "Review index stories covering success, loading, empty, partial issues failure, and full error states."
  ),
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Success: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/reviews"),
    ...storybookMswParameters({
      dashboard: [
        http.get("/api/ui/dashboard", () =>
          HttpResponse.json({ dashboard: reviewsDashboardFixture })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const Loading: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/reviews"),
    ...storybookMswParameters({
      dashboard: [
        http.get("/api/ui/dashboard", async () => {
          await delay("infinite");
          return HttpResponse.json({ dashboard: reviewsDashboardFixture });
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const Empty: Story = {
  args: {},
  parameters: appRouteParameters("/reviews"),
  render: () => <StorybookWorkspaceSetup />,
};

export const PartialIssuesError: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/reviews"),
    ...storybookMswParameters({
      dashboard: [
        http.get("/api/ui/dashboard", () =>
          HttpResponse.json({ dashboard: reviewsDashboardFixture })
        ),
      ],
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

export const Error: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/reviews"),
    ...storybookMswParameters({
      dashboard: [
        http.get("/api/ui/dashboard", () =>
          HttpResponse.json(
            {
              code: "reviews_failed",
              message: "Reviews unavailable",
            },
            { status: 500 }
          )
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
