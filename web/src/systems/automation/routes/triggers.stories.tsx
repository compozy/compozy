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
  title: "systems/automation/routes/Triggers",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Full-page triggers route stories with the real shell, covering list/detail states, scope filtering, and editor flows.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {},
  parameters: appRouteParameters("/triggers"),
  render: () => <StorybookWorkspaceSetup />,
};

export const Cards: Story = {
  args: {},
  tags: ["play-fn"],
  parameters: appRouteParameters("/triggers?view=cards"),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await waitFor(() => expect(canvas.getByTestId("triggers-list-card-grid")).toBeVisible(), {
      timeout: 5000,
    });
  },
};

export const Empty: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/triggers"),
    ...storybookMswParameters({
      automation: [
        compozyApiMock.get("/api/automation/triggers", () =>
          HttpResponse.json({
            page: { has_more: false, limit: 50, total: 0 },
            triggers: [],
          })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const FilteredEmpty: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/triggers?q=zzzz"),
    ...storybookMswParameters({
      automation: [
        compozyApiMock.get("/api/automation/triggers", () =>
          HttpResponse.json({
            page: { has_more: false, limit: 50, total: 0 },
            triggers: [],
          })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const TriggersError: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/triggers"),
    ...storybookMswParameters({
      automation: [
        compozyApiMock.get("/api/automation/triggers", () =>
          HttpResponse.json({ error: "triggers unavailable" }, { status: 500 })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const EditorCreate: Story = {
  args: {},
  tags: ["play-fn"],
  parameters: appRouteParameters("/triggers"),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await waitFor(() => expect(canvas.getByTestId("create-trigger-btn")).toBeEnabled(), {
      timeout: 5000,
    });
    await userEvent.click(canvas.getByTestId("create-trigger-btn"));
    await expect(
      within(document.body).findByTestId("automation-trigger-form")
    ).resolves.toBeDefined();
  },
};

export const Loading: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/triggers"),
    ...storybookMswParameters({
      automation: [
        compozyApiMock.get("/api/automation/triggers", async () => {
          await delay("infinite");
          return HttpResponse.json({
            page: { has_more: false, limit: 50, total: 0 },
            triggers: [],
          });
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** Agent target on an observer event — the canonical trigger detail read. */
export const TriggerDetail: Story = {
  args: {},
  tags: ["play-fn"],
  parameters: appRouteParameters("/triggers/trg_summarize_failures"),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await waitFor(() => expect(canvas.getByTestId("automation-detail-panel")).toBeVisible(), {
      timeout: 5000,
    });
    await expect(canvas.getByTestId("trigger-detail-sentence")).toHaveTextContent(
      "When a session stops"
    );
  },
};

/** Loop target: labeled input rows instead of a prompt, delegated runs. */
export const TriggerDetailLoop: Story = {
  args: {},
  tags: ["play-fn"],
  parameters: appRouteParameters("/triggers/trg_rerun_delivery"),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await waitFor(() => expect(canvas.getByTestId("trigger-loop-mapping")).toBeVisible(), {
      timeout: 5000,
    });
  },
};

/** Webhook event: local POST path in the rule, public reachability in the rail. */
export const TriggerDetailWebhook: Story = {
  args: {},
  tags: ["play-fn"],
  parameters: appRouteParameters("/triggers/trg_deploy_webhook"),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await waitFor(() => expect(canvas.getByTestId("trigger-webhook-endpoint")).toBeVisible(), {
      timeout: 5000,
    });
    await expect(canvas.getByTestId("automation-trigger-ingress")).toHaveTextContent("Live");
  },
};

/** The raw read: runtime enums and the activation envelope, behind Inspect. */
export const TriggerDetailInspect: Story = {
  args: {},
  tags: ["play-fn"],
  parameters: appRouteParameters("/triggers/trg_deploy_webhook"),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await waitFor(() => expect(canvas.getByTestId("trigger-inspect-btn")).toBeVisible(), {
      timeout: 5000,
    });
    await userEvent.click(canvas.getByTestId("trigger-inspect-btn"));
    await expect(
      within(document.body).findByTestId("trigger-inspect-sheet")
    ).resolves.toBeDefined();
  },
};

/** Disabled + config-owned: pause line, dashed lockbar, no Edit affordance. */
export const TriggerDetailLocked: Story = {
  args: {},
  tags: ["play-fn"],
  parameters: {
    ...appRouteParameters("/triggers/trg_summarize_failures"),
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
    await waitFor(() => expect(canvas.getByTestId("trigger-detail-lock")).toBeVisible(), {
      timeout: 5000,
    });
    await expect(canvas.getByTestId("trigger-pause-line")).toBeVisible();
    await expect(canvas.queryByTestId("edit-trigger-btn")).toBeNull();
  },
};
