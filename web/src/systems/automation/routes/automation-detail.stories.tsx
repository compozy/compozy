import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, HttpResponse } from "msw";
import { compozyApiMock } from "@/storybook/openapi-msw";
import { expect, userEvent, waitFor, within } from "storybook/test";

import { storybookMswParameters } from "@/storybook/msw";
import { automationTriggerDetailFixtures } from "@/systems/automation/mocks";
import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/automation/routes/AutomationDetail",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Full-page automation detail route stories (`/automations/jobs/:id`, `/automations/triggers/:id`) with the real shell.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const JobDetail: Story = {
  args: {},
  tags: ["play-fn"],
  parameters: appRouteParameters("/automations/jobs/job_launch_command_digest"),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await waitFor(() => expect(canvas.getByTestId("automation-detail-panel")).toBeVisible(), {
      timeout: 5000,
    });
  },
};

/** Agent target on an observer event — the canonical trigger detail read. */
export const TriggerDetail: Story = {
  args: {},
  tags: ["play-fn"],
  parameters: appRouteParameters("/automations/triggers/trg_summarize_failures"),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await waitFor(() => expect(canvas.getByTestId("automation-detail-panel")).toBeVisible(), {
      timeout: 5000,
    });
    await expect(canvas.getByTestId("automation-detail-sentence")).toHaveTextContent(
      "When a session stops"
    );
  },
};

/** Loop target: labeled input rows instead of a prompt, handed-off runs. */
export const TriggerDetailLoop: Story = {
  args: {},
  tags: ["play-fn"],
  parameters: appRouteParameters("/automations/triggers/trg_rerun_delivery"),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await waitFor(() => expect(canvas.getByTestId("automation-loop-inputs")).toBeVisible(), {
      timeout: 5000,
    });
  },
};

/** Webhook event: local POST path in the rule, public reachability in the rail. */
export const TriggerDetailWebhook: Story = {
  args: {},
  tags: ["play-fn"],
  parameters: appRouteParameters("/automations/triggers/trg_deploy_webhook"),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await waitFor(() => expect(canvas.getByTestId("automation-webhook-endpoint")).toBeVisible(), {
      timeout: 5000,
    });
    await expect(canvas.getByTestId("automation-rail-public-link")).toHaveTextContent("Live");
  },
};

/** The machine truth: runtime enums and a sample event, behind Inspect. */
export const TriggerDetailInspect: Story = {
  args: {},
  tags: ["play-fn"],
  parameters: appRouteParameters("/automations/triggers/trg_deploy_webhook"),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await waitFor(() => expect(canvas.getByTestId("automation-inspect-btn")).toBeVisible(), {
      timeout: 5000,
    });
    await userEvent.click(canvas.getByTestId("automation-inspect-btn"));
    await expect(
      within(document.body).findByTestId("automation-inspect-sheet")
    ).resolves.toBeDefined();
  },
};

/** Disabled + config-owned: pause line, dashed lockbar, no Edit affordance. */
export const TriggerDetailLocked: Story = {
  args: {},
  tags: ["play-fn"],
  parameters: {
    ...appRouteParameters("/automations/triggers/trg_summarize_failures"),
    ...storybookMswParameters({
      automation: [
        compozyApiMock.get("/api/automation/triggers/{id}", () =>
          HttpResponse.json({
            trigger: {
              ...automationTriggerDetailFixtures[0],
              enabled: false,
              source: "config",
            },
          })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await waitFor(() => expect(canvas.getByTestId("automation-lockbar")).toBeVisible(), {
      timeout: 5000,
    });
    await expect(canvas.getByTestId("automation-pause-line")).toBeVisible();
    await expect(canvas.queryByTestId("automation-edit-btn")).toBeNull();
  },
};

/** The job read never settles: the page grid holds the head, the rule card, the runs and the rail. */
export const JobDetailLoading: Story = {
  args: {},
  tags: ["play-fn"],
  parameters: {
    ...appRouteParameters("/automations/jobs/job_launch_command_digest"),
    ...storybookMswParameters({
      automation: [
        compozyApiMock.get("/api/automation/jobs/{id}", async () => {
          await delay("infinite");
          return HttpResponse.json({ error: "never" }, { status: 500 });
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await waitFor(() => expect(canvas.getByTestId("automation-detail-loading")).toBeVisible(), {
      timeout: 5000,
    });
  },
};

/** The trigger read never settles: an event has no next runs and no Schedule band. */
export const TriggerDetailLoading: Story = {
  args: {},
  tags: ["play-fn"],
  parameters: {
    ...appRouteParameters("/automations/triggers/trg_summarize_failures"),
    ...storybookMswParameters({
      automation: [
        compozyApiMock.get("/api/automation/triggers/{id}", async () => {
          await delay("infinite");
          return HttpResponse.json({ error: "never" }, { status: 500 });
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await waitFor(() => expect(canvas.getByTestId("automation-detail-loading")).toBeVisible(), {
      timeout: 5000,
    });
  },
};
