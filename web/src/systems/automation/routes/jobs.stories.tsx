import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, HttpResponse } from "msw";
import { aghApiMock } from "@/storybook/openapi-msw";
import { expect, userEvent, waitFor, within } from "storybook/test";

import { storybookMswParameters } from "@/storybook/msw";
import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/automation/routes/Jobs",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Full-page jobs route stories with the real shell, covering list/detail states, scope filtering, and editor flows.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {},
  parameters: appRouteParameters("/jobs"),
  render: () => <StorybookWorkspaceSetup />,
};

export const Cards: Story = {
  args: {},
  tags: ["play-fn"],
  parameters: appRouteParameters("/jobs?view=cards"),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await waitFor(() => expect(canvas.getByTestId("jobs-list-card-grid")).toBeVisible(), {
      timeout: 5000,
    });
  },
};

export const Empty: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/jobs"),
    ...storybookMswParameters({
      automation: [
        aghApiMock.get("/api/automation/jobs", () =>
          HttpResponse.json({ jobs: [], page: { has_more: false, limit: 50, total: 0 } })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const JobsError: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/jobs"),
    ...storybookMswParameters({
      automation: [
        aghApiMock.get("/api/automation/jobs", () =>
          HttpResponse.json({ error: "jobs unavailable" }, { status: 500 })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const EditorCreate: Story = {
  args: {},
  tags: ["play-fn"],
  parameters: appRouteParameters("/jobs"),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await waitFor(() => expect(canvas.getByTestId("create-job-btn")).toBeEnabled(), {
      timeout: 5000,
    });
    await userEvent.click(canvas.getByTestId("create-job-btn"));
    await expect(within(document.body).findByTestId("automation-job-form")).resolves.toBeDefined();
  },
};

export const Loading: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/jobs"),
    ...storybookMswParameters({
      automation: [
        aghApiMock.get("/api/automation/jobs", async () => {
          await delay("infinite");
          return HttpResponse.json({
            jobs: [],
            page: { has_more: false, limit: 50, total: 0 },
          });
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const JobDetail: Story = {
  args: {},
  tags: ["play-fn"],
  parameters: appRouteParameters("/jobs/job_launch_command_digest"),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await waitFor(() => expect(canvas.getByTestId("automation-detail-panel")).toBeVisible(), {
      timeout: 5000,
    });
  },
};
