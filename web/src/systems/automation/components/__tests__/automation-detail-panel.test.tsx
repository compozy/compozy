// Suite: Automation detail panel
// Invariant: Persisted automation reads render the stored execution target without agent-only loss.
// Boundary IN: Job/Trigger API read models and the detail/run-history presentation.
// Boundary OUT: persistence and dispatch, owned by daemon/store suites.
import { fireEvent, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { AnchorHTMLAttributes } from "react";
import { describe, expect, it, vi } from "vitest";

import { renderWithTopbar } from "@/test/render-with-topbar";

interface MockLinkParams {
  id?: string;
  runId?: string;
}

interface MockLinkProps extends AnchorHTMLAttributes<HTMLAnchorElement> {
  params?: MockLinkParams;
  to?: string;
}

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, params, to, ...props }: MockLinkProps) => (
    <a
      href={
        to === "/loop-runs/$runId"
          ? `/loop-runs/${params?.runId ?? ""}`
          : `/session/${params?.id ?? ""}`
      }
      {...props}
    >
      {children}
    </a>
  ),
  useNavigate: () => vi.fn(),
}));

import { AutomationDetailPanel } from "../automation-detail-panel";

const jobFixture = {
  id: "job_daily_review",
  name: "daily-review",
  agent_name: "reviewer",
  prompt: "Review recent changes.",
  scope: "workspace" as const,
  workspace_id: "ws_alpha",
  source: "dynamic" as const,
  target_kind: "agent",
  enabled: true,
  schedule: { mode: "cron" as const, expr: "0 9 * * *" },
  retry: { strategy: "none" as const, max_retries: 3, base_delay: "2s" },
  fire_limit: { max: 12, window: "1h" },
  next_run: "2026-04-12T09:00:00Z",
  scheduler: {
    job_id: "job_daily_review",
    registered: true,
    next_run_at: "2026-04-12T09:00:00Z",
    last_run_at: "2026-04-11T09:00:01Z",
    last_scheduled_at: "2026-04-11T09:00:00Z",
    last_fire_id: "fire_daily_review_001",
    catch_up_policy: "skip_missed" as const,
    misfire_grace_seconds: 30,
    misfire_count: 1,
    last_misfire_at: "2026-04-10T09:00:00Z",
    updated_at: "2026-04-11T09:00:01Z",
  },
  created_at: "2026-04-11T09:00:00Z",
  updated_at: "2026-04-11T09:05:00Z",
};

const triggerFixture = {
  id: "trg_push_review",
  name: "push-review",
  agent_name: "reviewer",
  prompt: "Review push event {{ .Data.branch }}.",
  event: "webhook",
  filter: { "data.branch": "main" },
  scope: "workspace" as const,
  workspace_id: "ws_alpha",
  source: "config" as const,
  target_kind: "agent",
  enabled: false,
  retry: { strategy: "backoff" as const, max_retries: 4, base_delay: "5s" },
  fire_limit: { max: 12, window: "1h" },
  endpoint_slug: "push-review",
  webhook_id: "wbh_push_review",
  webhook_secret_present: false,
  created_at: "2026-04-11T08:00:00Z",
  updated_at: "2026-04-11T08:10:00Z",
};

const runFixture = {
  id: "run_001",
  status: "completed" as const,
  attempt: 1,
  job_id: "job_daily_review",
  fire_id: "fire_daily_review_001",
  session_id: "sess_001",
  scheduled_at: "2026-04-11T09:00:00Z",
  started_at: "2026-04-11T10:00:00Z",
  ended_at: "2026-04-11T10:05:00Z",
};

function renderPanel(overrides: Partial<Parameters<typeof AutomationDetailPanel>[0]> = {}) {
  const onDelete = vi.fn();
  const onEdit = vi.fn();
  const onToggleEnabled = vi.fn();
  const onTriggerNow = vi.fn();

  renderWithTopbar(
    <AutomationDetailPanel
      error={null}
      state={{
        isDeleting: false,
        isLoading: false,
        isTogglePending: false,
        isTriggerPending: false,
        ...overrides.state,
      }}
      item={jobFixture}
      kind="jobs"
      onDelete={onDelete}
      onEdit={onEdit}
      onToggleEnabled={onToggleEnabled}
      onTriggerNow={onTriggerNow}
      runs={[runFixture]}
      runsError={null}
      runsLoading={false}
      {...overrides}
    />
  );

  return { onDelete, onEdit, onToggleEnabled, onTriggerNow };
}

describe("AutomationDetailPanel", () => {
  it("renders loading state", () => {
    renderPanel({
      state: {
        isDeleting: false,
        isLoading: true,
        isTogglePending: false,
        isTriggerPending: false,
      },
      item: undefined,
    });
    expect(screen.getByTestId("automation-detail-loading")).toBeInTheDocument();
  });

  it("renders error state", () => {
    renderPanel({ error: new Error("boom"), item: undefined });
    expect(screen.getByTestId("automation-detail-error")).toBeInTheDocument();
  });

  it("renders the unavailable state when the routed item resolves to nothing", () => {
    renderPanel({ item: undefined });
    expect(screen.getByTestId("automation-detail-empty")).toBeInTheDocument();
    expect(screen.getByText("Job unavailable")).toBeInTheDocument();
  });

  it("renders dynamic job details and dispatches non-destructive action callbacks", () => {
    const { onDelete, onEdit, onToggleEnabled, onTriggerNow } = renderPanel();

    expect(screen.getByTestId("automation-detail-panel")).toBeInTheDocument();
    expect(screen.getByTestId("topbar-title-text")).toHaveTextContent("daily-review");
    expect(screen.getByTestId("automation-detail-header")).toBeInTheDocument();
    expect(screen.getByText("Review recent changes.")).toBeInTheDocument();
    expect(screen.getByTestId("automation-job-scheduler")).toHaveTextContent("Skip missed");
    expect(screen.getByTestId("automation-job-scheduler")).toHaveTextContent(
      "fire_daily_review_001"
    );
    expect(screen.getByTestId("automation-run-run_001")).toBeInTheDocument();
    expect(screen.getByTestId("automation-run-run_001")).toHaveAttribute(
      "href",
      "/session/sess_001"
    );

    fireEvent.click(screen.getByTestId("trigger-job-btn"));
    fireEvent.click(screen.getByTestId("automation-detail-overflow"));
    fireEvent.click(screen.getByTestId("edit-automation-btn"));
    fireEvent.click(screen.getByTestId("automation-detail-overflow"));
    fireEvent.click(screen.getByTestId("toggle-automation-btn"));

    expect(onToggleEnabled).toHaveBeenCalledWith(false);
    expect(onEdit).toHaveBeenCalledOnce();
    expect(onTriggerNow).toHaveBeenCalledOnce();
    expect(onDelete).not.toHaveBeenCalled();
  });

  it("Should disable Run now when the automation runtime is unavailable", () => {
    const { onTriggerNow } = renderPanel({
      state: {
        isDeleting: false,
        isLoading: false,
        isTogglePending: false,
        isTriggerDisabled: true,
        isTriggerPending: false,
      },
    });

    const trigger = screen.getByTestId("trigger-job-btn");
    expect(trigger).toBeDisabled();
    fireEvent.click(trigger);
    expect(onTriggerNow).not.toHaveBeenCalled();
  });

  it("Should render the target-aware Default catch-up label when the scheduler omits a policy, never the removed skip value", () => {
    renderPanel({
      item: {
        ...jobFixture,
        scheduler: { ...jobFixture.scheduler, catch_up_policy: undefined },
      },
    });

    const scheduler = screen.getByTestId("automation-job-scheduler");
    expect(scheduler).toHaveTextContent("Default");
    expect(scheduler).not.toHaveTextContent("skip");
  });

  it.each([
    { item: jobFixture, kind: "jobs" as const, noun: "job" },
    {
      item: { ...triggerFixture, source: "dynamic" as const },
      kind: "triggers" as const,
      noun: "trigger",
    },
  ])("Should require explicit name confirmation before deleting a dynamic $noun", async case_ => {
    const user = userEvent.setup();
    const { onDelete } = renderPanel({ item: case_.item, kind: case_.kind, runs: [] });

    fireEvent.click(screen.getByTestId("automation-detail-overflow"));
    fireEvent.click(screen.getByTestId("delete-automation-btn"));
    expect(onDelete).not.toHaveBeenCalled();
    expect(screen.getByRole("dialog", { name: `Delete ${case_.noun}?` })).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Cancel" }));
    expect(onDelete).not.toHaveBeenCalled();
    expect(screen.getByTestId("automation-detail-panel")).toBeInTheDocument();

    fireEvent.click(screen.getByTestId("automation-detail-overflow"));
    fireEvent.click(screen.getByTestId("delete-automation-btn"));
    const confirmButton = screen.getByTestId("confirm-delete-automation-btn");
    await user.type(screen.getByLabelText("Type to confirm"), `${case_.item.name}-wrong`);
    expect(confirmButton).toBeDisabled();

    await user.clear(screen.getByLabelText("Type to confirm"));
    await user.type(screen.getByLabelText("Type to confirm"), case_.item.name);
    expect(confirmButton).toBeEnabled();
    await user.click(confirmButton);

    expect(onDelete).toHaveBeenCalledOnce();
  });

  it("Should render a persisted Loop Job target, typed inputs, and delegated Loop correlation", () => {
    renderPanel({
      item: {
        ...jobFixture,
        agent_name: "",
        prompt: "",
        target_kind: "loop",
        loop_target: {
          workspace_id: "ws_alpha",
          loop_name: "software-delivery",
          inputs: { slug: "helix-v1-launch", dry_run: false },
          input_mapping: {},
        },
      },
      runs: [
        {
          ...runFixture,
          id: "run_loop",
          status: "delegated",
          session_id: undefined,
          loop_run_id: "looprun_aeb24d4f17cf1feb",
        },
      ],
    });

    expect(screen.getByTestId("automation-detail-meta")).toHaveTextContent(
      "Loop: software-delivery"
    );
    expect(screen.getByTestId("automation-target-details")).toHaveTextContent("software-delivery");
    expect(screen.getByTestId("automation-target-details")).toHaveTextContent("helix-v1-launch");
    expect(screen.queryByText("Prompt")).not.toBeInTheDocument();
    expect(screen.queryByText(/Agent:/)).not.toBeInTheDocument();
    expect(screen.getByTestId("automation-run-run_loop")).toHaveAttribute(
      "href",
      "/loop-runs/looprun_aeb24d4f17cf1feb"
    );
  });

  it("Should render a persisted Loop Trigger target and event input mapping", () => {
    renderPanel({
      item: {
        ...triggerFixture,
        source: "dynamic" as const,
        agent_name: "",
        prompt: "",
        target_kind: "loop",
        loop_target: {
          workspace_id: "ws_alpha",
          loop_name: "software-delivery",
          inputs: { slug: "helix-v1-launch" },
          input_mapping: { branch: "data.branch" },
        },
      },
      kind: "triggers",
      runs: [],
    });

    expect(screen.getByTestId("automation-detail-meta")).toHaveTextContent(
      "Loop: software-delivery"
    );
    expect(screen.getByTestId("automation-target-details")).toHaveTextContent("data.branch");
    expect(screen.queryByText("Prompt template")).not.toBeInTheDocument();
    expect(screen.queryByText(/Dispatches to/)).not.toBeInTheDocument();
  });

  it("Should render the detail header with the job name in the window-head slot", () => {
    renderPanel();

    const header = screen.getByTestId("automation-detail-header");
    expect(header).toBeInTheDocument();
    expect(screen.getByTestId("topbar-title-text")).toHaveTextContent("daily-review");
    expect(header.querySelector("[data-slot='page-head']")).toBeNull();
  });

  it("renders manual jobs without implying a cron schedule", () => {
    renderPanel({
      item: {
        ...jobFixture,
        schedule: undefined,
      },
    });

    expect(screen.getByText("manual")).toBeInTheDocument();
    expect(screen.getAllByText("Manual")).toHaveLength(2);
    expect(screen.queryByText("Cron schedule")).not.toBeInTheDocument();
  });

  it("renders truthful recent-window metrics from the fetched run sample", () => {
    renderPanel({
      item: jobFixture,
      runs: [
        runFixture,
        { ...runFixture, id: "run_002", status: "completed" as const },
        { ...runFixture, id: "run_003", status: "failed" as const },
      ],
    });

    const successRate = screen.getByTestId("automation-job-metric-success-rate");
    const runsShown = screen.getByTestId("automation-job-metric-runs");
    expect(successRate).toHaveTextContent("Recent success");
    expect(successRate).toHaveTextContent("67%");
    expect(runsShown).toHaveTextContent("Runs shown");
    expect(runsShown).toHaveTextContent("3");
  });

  it("renders the trigger hook Section with a KindChip for the source", () => {
    renderPanel({
      item: { ...triggerFixture, source: "dynamic" as const, event: "ext.github.push" },
      kind: "triggers",
      runs: [],
    });

    const kindChip = document.querySelector('[data-slot="kind-chip"][data-kind="ext.github.push"]');
    expect(kindChip).not.toBeNull();
  });

  it.each([
    {
      copy: "This automation is defined in configuration files. Only its enabled state can be changed here.",
      source: "config" as const,
    },
    {
      copy: "This automation is provided by an installed package. Only its enabled state can be changed here.",
      source: "package" as const,
    },
  ])("Should describe $source trigger ownership without mutable actions", ({ copy, source }) => {
    renderPanel({
      item: { ...triggerFixture, source },
      kind: "triggers",
      runs: [
        { ...runFixture, id: "run_trigger", trigger_id: "trg_push_review", job_id: undefined },
      ],
    });

    expect(screen.getByText(copy)).toBeInTheDocument();
    expect(screen.getByText("Webhook id")).toBeInTheDocument();
    expect(screen.getByText("wbh_push_review")).toBeInTheDocument();
    expect(screen.queryByTestId("edit-automation-btn")).not.toBeInTheDocument();
    expect(screen.queryByTestId("delete-automation-btn")).not.toBeInTheDocument();
    expect(screen.queryByTestId("trigger-job-btn")).not.toBeInTheDocument();
  });

  it("renders a dynamic webhook trigger endpoint URL and curl copy affordance", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });
    renderPanel({
      item: {
        ...triggerFixture,
        source: "dynamic" as const,
        scope: "global" as const,
        workspace_id: undefined,
      },
      kind: "triggers",
      runs: [],
    });

    expect(screen.getByText("Webhook endpoint")).toBeInTheDocument();
    expect(
      screen.getByText("/api/webhooks/global/push-review--wbh_push_review")
    ).toBeInTheDocument();
    expect(screen.getByText("curl")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Copy webhook URL" }));

    await waitFor(() =>
      expect(writeText).toHaveBeenCalledWith("/api/webhooks/global/push-review--wbh_push_review")
    );
    fireEvent.click(screen.getByTestId("automation-detail-overflow"));
    expect(screen.getByTestId("edit-automation-btn")).toBeInTheDocument();
    expect(screen.getByTestId("delete-automation-btn")).toBeInTheDocument();
  });
});
