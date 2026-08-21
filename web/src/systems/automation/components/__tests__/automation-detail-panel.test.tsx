// Suite: Automation job detail panel
// Invariant: A persisted job read renders its schedule, stored execution target, and run
// history without agent-only loss; destructive deletion requires explicit confirmation.
// Boundary IN: Job API read models and the job detail/run-history presentation.
// Boundary OUT: trigger detail (trigger-detail-panel.test.tsx); persistence and dispatch,
// owned by daemon/store suites.
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
  search?: { workspace?: string };
  to?: string;
}

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, params, search, to, ...props }: MockLinkProps) => (
    <a
      href={
        to === "/loop-runs/$runId"
          ? `/loop-runs/${params?.runId ?? ""}${search?.workspace ? `?workspace=${encodeURIComponent(search.workspace)}` : ""}`
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
  profile_id: "00000000000000000000000000",
  profile_name: "default",
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

const runFixture = {
  profile_id: "00000000000000000000000000",
  profile_name: "default",
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
  const onBack = vi.fn();
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
      onBack={onBack}
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

  return { onBack, onDelete, onEdit, onToggleEnabled, onTriggerNow };
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
    const { onBack, onDelete, onEdit, onToggleEnabled, onTriggerNow } = renderPanel();

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
    fireEvent.click(screen.getByRole("button", { name: "Back one level" }));

    expect(onToggleEnabled).toHaveBeenCalledWith(false);
    expect(onEdit).toHaveBeenCalledOnce();
    expect(onTriggerNow).toHaveBeenCalledOnce();
    expect(onBack).toHaveBeenCalledOnce();
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

  it("Should require explicit name confirmation before deleting a dynamic job", async () => {
    const user = userEvent.setup();
    const { onDelete } = renderPanel({ runs: [] });

    fireEvent.click(screen.getByTestId("automation-detail-overflow"));
    fireEvent.click(screen.getByTestId("delete-automation-btn"));
    expect(onDelete).not.toHaveBeenCalled();
    expect(screen.getByRole("dialog", { name: "Delete job?" })).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Cancel" }));
    expect(onDelete).not.toHaveBeenCalled();
    expect(screen.getByTestId("automation-detail-panel")).toBeInTheDocument();

    fireEvent.click(screen.getByTestId("automation-detail-overflow"));
    fireEvent.click(screen.getByTestId("delete-automation-btn"));
    const confirmButton = screen.getByTestId("confirm-delete-automation-btn");
    await user.type(screen.getByLabelText("Type to confirm"), `${jobFixture.name}-wrong`);
    expect(confirmButton).toBeDisabled();

    await user.clear(screen.getByLabelText("Type to confirm"));
    await user.type(screen.getByLabelText("Type to confirm"), jobFixture.name);
    expect(confirmButton).toBeEnabled();
    await user.click(confirmButton);

    expect(onDelete).toHaveBeenCalledOnce();
  });

  it("Should expose a failed delete and allow a second confirmed attempt", async () => {
    const user = userEvent.setup();
    const onDelete = vi
      .fn<() => Promise<void>>()
      .mockRejectedValueOnce(new Error("Delete failed"))
      .mockResolvedValueOnce(undefined);
    renderPanel({ onDelete });

    fireEvent.click(screen.getByTestId("automation-detail-overflow"));
    fireEvent.click(screen.getByTestId("delete-automation-btn"));
    await user.type(screen.getByLabelText("Type to confirm"), jobFixture.name);
    await user.click(screen.getByTestId("confirm-delete-automation-btn"));

    await waitFor(() =>
      expect(screen.getByTestId("automation-delete-error")).toHaveTextContent("Delete failed")
    );
    expect(screen.getByTestId("confirm-delete-automation-btn")).toBeEnabled();

    await user.click(screen.getByTestId("confirm-delete-automation-btn"));
    await waitFor(() => expect(onDelete).toHaveBeenCalledTimes(2));
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
          loop_name: "implement-tasks",
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

    expect(screen.getByTestId("automation-detail-meta")).toHaveTextContent("Loop: implement-tasks");
    expect(screen.getByTestId("automation-target-details")).toHaveTextContent("implement-tasks");
    expect(screen.getByTestId("automation-target-details")).toHaveTextContent("helix-v1-launch");
    expect(screen.queryByText("Prompt")).not.toBeInTheDocument();
    expect(screen.queryByText(/Agent:/)).not.toBeInTheDocument();
    expect(screen.getByTestId("automation-run-run_loop")).toHaveAttribute(
      "href",
      "/loop-runs/looprun_aeb24d4f17cf1feb?workspace=ws_alpha"
    );
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
        {
          ...runFixture,
          id: "run_002",
          status: "completed" as const,
        },
        {
          ...runFixture,
          id: "run_003",
          status: "failed" as const,
        },
      ],
    });

    const successRate = screen.getByTestId("automation-job-metric-success-rate");
    const runsShown = screen.getByTestId("automation-job-metric-runs");
    expect(successRate).toHaveTextContent("Recent success");
    expect(successRate).toHaveTextContent("67%");
    expect(runsShown).toHaveTextContent("Runs shown");
    expect(runsShown).toHaveTextContent("3");
  });
});
