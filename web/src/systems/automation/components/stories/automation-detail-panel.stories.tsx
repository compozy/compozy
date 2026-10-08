import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, waitFor, within } from "storybook/test";

import { PanelSurface } from "@/storybook/story-layout";

import {
  AutomationDetailPanel,
  type AutomationDetailPanelProps,
} from "../automation-detail/automation-detail-panel";
import type { AutomationEntity } from "../../lib/automation-entity";
import { toAutomationView } from "../../lib/automation-view";
import {
  dependencyReviewJob,
  deployWebhookTrigger,
  makeDetailRun,
  morningDigestJob,
  morningDigestRuns,
  releaseChecklistJob,
  rerunDeliveryRuns,
  rerunDeliveryTrigger,
} from "../../mocks/detail-fixtures";

const ctx = { workspaceName: (id: string) => (id === "ws_checkout_api" ? "checkout-api" : id) };

function detailArgs(
  entity: AutomationEntity | undefined,
  overrides: Partial<AutomationDetailPanelProps> = {}
): AutomationDetailPanelProps {
  return {
    status: entity ? "ready" : "missing",
    statusMessage: "This automation is no longer available.",
    entity,
    view: entity ? toAutomationView(entity, ctx) : undefined,
    sentenceContext: ctx,
    loopWorkspaceName: "checkout-api",
    loopMissing: false,
    lastRanAt: null,
    runs: [],
    runsError: null,
    runsLoading: false,
    state: {
      isDeleting: false,
      isRunNowDisabled: false,
      isRunNowPending: false,
      isTogglePending: false,
    },
    onBack: fn(),
    onDelete: fn(),
    onEdit: fn(),
    onRetryRuns: fn(),
    onRunNow: fn(),
    onSetUpRetries: fn(),
    onToggleEnabled: fn(),
    ...overrides,
  };
}

const meta: Meta<typeof AutomationDetailPanel> = {
  title: "systems/automation/components/AutomationDetailPanel",
  component: AutomationDetailPanel,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "One detail page for every automation (board automations-detail.html VC-01…09): sentence + switch, How it works, Runs, one-card rail, lockbar, Inspect, delete, states.",
      },
    },
  },
  decorators: [
    Story => (
      <PanelSurface>
        <Story />
      </PanelSurface>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/** VC-01 — a schedule that asks an agent: Run now, next 3 runs, mixed run history. */
export const Schedule: Story = {
  args: detailArgs(morningDigestJob, {
    lastRanAt: "2026-10-07T09:00:00Z",
    runs: morningDigestRuns,
  }),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.getByTestId("automation-detail-sentence")).toHaveTextContent(
      "Every weekday at 09:00 UTC, ask summarizer."
    );
    await expect(canvas.getByTestId("automation-next-runs")).toBeVisible();
  },
};

/** VC-02 — an event that starts a Loop: Only if, labeled Loop inputs, handed-off runs. */
export const EventStartsLoop: Story = {
  args: detailArgs(rerunDeliveryTrigger, {
    lastRanAt: "2026-10-07T13:41:00Z",
    runs: rerunDeliveryRuns,
  }),
};

/** VC-03 — a link another app calls: endpoint, signed example, Public link + Security. */
export const Webhook: Story = {
  args: detailArgs(deployWebhookTrigger),
};

/** VC-04 — Off and creating a task: pause line, no next run, Run now stays. */
export const OffCreatesTask: Story = {
  args: detailArgs(dependencyReviewJob, {
    runs: [
      makeDetailRun({
        id: "run_task",
        job_id: "dependency-review",
        status: "delegated",
        session_id: undefined,
        task_id: "task_review_42",
        ended_at: undefined,
      }),
    ],
  }),
};

/** VC-05 — from config: lockbar, On/Off and Run now only. */
export const FromConfig: Story = {
  args: detailArgs(releaseChecklistJob),
};

/** VC-06 — every run status and both skip reasons, one dictionary. */
export const RunStatuses: Story = {
  args: detailArgs(morningDigestJob, {
    runs: [
      makeDetailRun({
        id: "run_scheduled",
        status: "scheduled",
        session_id: undefined,
        ended_at: undefined,
      }),
      makeDetailRun({ id: "run_running", status: "running", ended_at: undefined }),
      makeDetailRun({
        id: "run_delegated",
        status: "delegated",
        session_id: undefined,
        loop_run_id: "looprun_8f3a2b",
      }),
      makeDetailRun({ id: "run_completed" }),
      makeDetailRun({
        id: "run_failed",
        status: "failed",
        session_id: undefined,
        error: "Agent summarizer was not available",
      }),
      makeDetailRun({ id: "run_canceled", status: "canceled", session_id: undefined }),
      makeDetailRun({
        id: "run_skipped",
        status: "canceled",
        session_id: undefined,
        metadata: { reason: "self_overlap" },
      }),
      makeDetailRun({
        id: "run_missed",
        status: "canceled",
        session_id: undefined,
        metadata: { reason: "misfire_grace_exceeded" },
      }),
    ],
  }),
};

/** VC-07 — Inspect: the machine truth for a job. */
export const Inspect: Story = {
  args: detailArgs(morningDigestJob),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByTestId("automation-inspect-btn"));
    const page = within(canvasElement.ownerDocument.body);
    await expect(await page.findByTestId("automation-inspect-tile-kind")).toHaveTextContent(
      "job · cron"
    );
  },
};

/** VC-08 — Delete by typing the name. */
export const DeleteConfirm: Story = {
  args: detailArgs(morningDigestJob),
  play: async ({ canvasElement }) => {
    const page = within(canvasElement.ownerDocument.body);
    await userEvent.click(page.getByTestId("automation-detail-overflow"));
    await userEvent.click(await page.findByTestId("automation-delete-btn"));
    await waitFor(() =>
      expect(page.getByRole("dialog", { name: "Delete automation?" })).toBeVisible()
    );
  },
};

/** VC-09 — loading keeps the geometry. */
export const Loading: Story = {
  args: detailArgs(undefined, { status: "loading" }),
};

/** VC-09 — a missing automation, with a way back. */
export const Missing: Story = {
  args: detailArgs(undefined),
};

/** VC-09 — an automation from another project, with a way back. */
export const OtherProject: Story = {
  args: detailArgs(undefined, {
    status: "elsewhere",
    statusMessage: "This automation belongs to another project. Switch to it to open this page.",
  }),
};
