import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, HttpResponse } from "msw";
import { aghApiMock } from "@/storybook/openapi-msw";
import { expect, userEvent, within } from "storybook/test";

import { storybookMswParameters } from "@/storybook/msw";
import { buildDetailFixture, taskDetailFixture } from "@/systems/tasks/mocks";
import type { TaskRecord } from "@/systems/tasks";
import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/tasks/routes/TaskDetail",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Task detail route stories rendered inside the persistent tasks shell, covering overview, tabbed panels, loading, and not-found branches.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Default overview tab for the primary Storybook task detail route.
 */
export const Overview: Story = {
  args: {},
  parameters: appRouteParameters("/tasks/task_001"),
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Runs tab listing prior attempts and deep-links to run detail pages.
 */
export const RunsTab: Story = {
  args: {},
  parameters: appRouteParameters("/tasks/task_001"),
  render: () => <StorybookWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId("tasks-detail-tab-runs"));
    await expect(canvas.findByTestId("tasks-runs-panel")).resolves.toBeDefined();
  },
};

/**
 * Activity tab: humanized newest-first feed with the category filter group.
 */
export const ActivityTab: Story = {
  args: {},
  parameters: appRouteParameters("/tasks/task_001"),
  render: () => <StorybookWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId("tasks-detail-tab-activity"));
    await expect(canvas.findByTestId("tasks-activity-panel")).resolves.toBeDefined();
  },
};

const BLOCKED_REASONS: NonNullable<TaskRecord["blocked_reasons"]> = [
  { source: "dependency", depends_on_task_ids: ["task_dep_001"] },
  { source: "approval", reason: "Awaiting operator approval" },
  {
    source: "block",
    kind: "transient",
    reason: "Upstream provider returned 503; retrying after cooldown",
    block_id: "block_001",
  },
];

/**
 * Overview tab for a blocked task carrying dependency + approval + typed block
 * causes simultaneously — one chip per `blocked_reasons` entry.
 */
export const BlockedReasons: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/tasks/task_001"),
    ...storybookMswParameters({
      tasks: [
        aghApiMock.get("/api/tasks/{id}", () =>
          HttpResponse.json({
            task: buildDetailFixture({
              task: {
                status: "blocked",
                blocked_reasons: BLOCKED_REASONS,
              } as TaskRecord,
            }),
          })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Escalated task: the unblock-loop breaker raised `needs_attention`, so the
 * header shows the escalation badge + Recover action and the wake indicator
 * reflects an agent-created task that opted out of creator wake.
 */
export const NeedsAttention: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/tasks/task_001"),
    ...storybookMswParameters({
      tasks: [
        aghApiMock.get("/api/tasks/{id}", () =>
          HttpResponse.json({
            task: buildDetailFixture({
              task: {
                status: "needs_attention",
                needs_attention: true,
                needs_attention_at: "2026-04-17T10:12:00Z",
                needs_attention_reason: "Block recurrence limit reached for transient blocks (2/2)",
                created_by: { kind: "agent_session", ref: "session_orchestrator" },
                wake_creator: false,
                blocked_reasons: BLOCKED_REASONS,
              } as TaskRecord,
            }),
          })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Loading branch while the detail payload is still being fetched.
 */
export const Loading: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/tasks/task_001"),
    ...storybookMswParameters({
      tasks: [
        aghApiMock.get("/api/tasks/{id}", async () => {
          await delay("infinite");
          return HttpResponse.json({ task: taskDetailFixture });
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Not-found branch for a missing task id while the shell remains mounted.
 */
export const NotFound: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/tasks/task_missing"),
    ...storybookMswParameters({
      tasks: [
        aghApiMock.get("/api/tasks/{id}", ({ params }) =>
          HttpResponse.json({ error: `Task not found: ${String(params.id)}` }, { status: 404 })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
