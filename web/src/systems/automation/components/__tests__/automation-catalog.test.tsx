// Suite: Automation catalog rows + shell
// Invariant: Catalog rows summarize loop targets by Loop name and typed inputs
// instead of blank agent content, and the shell's load-more/page-error chrome
// reflects pagination state. Ported from the deleted automation-list-panel
// suite when selection moved to the `$jobId`/`$triggerId` routes.
// Boundary IN: Job/Trigger list read models and row/shell presentation.
// Boundary OUT: routing, detail rendering, and data fetching (canonical suites).
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import type { AutomationJob, AutomationTrigger } from "../../types";
import { AutomationCatalogShell } from "../automation-catalog-shell";
import { AutomationJobRow } from "../automation-job-row";
import { AutomationTriggerRow } from "../automation-trigger-row";

vi.mock("@tanstack/react-router", () => ({
  Link: ({
    children,
    params: _params,
    to,
    ...props
  }: {
    children?: ReactNode;
    params?: Record<string, string>;
    to?: string;
  }) => (
    <a href={to} {...props}>
      {children}
    </a>
  ),
}));

const jobFixture: AutomationJob = {
  id: "job_daily_review",
  name: "daily-review",
  agent_name: "reviewer",
  prompt: "Review recent changes.",
  scope: "workspace",
  workspace_id: "ws_alpha",
  source: "dynamic",
  target_kind: "agent",
  enabled: true,
  schedule: { mode: "cron", expr: "0 9 * * *" },
  retry: { strategy: "none", max_retries: 3, base_delay: "2s" },
  fire_limit: { max: 12, window: "1h" },
  next_run: "2026-04-12T09:00:00Z",
  created_at: "2026-04-11T09:00:00Z",
  updated_at: "2026-04-11T09:05:00Z",
};

const loopTriggerFixture: AutomationTrigger = {
  id: "trg_loop",
  name: "loop-launch",
  agent_name: "",
  prompt: "",
  event: "ext.github.push",
  filter: {},
  scope: "workspace",
  workspace_id: "ws_alpha",
  source: "dynamic",
  target_kind: "loop",
  loop_target: {
    workspace_id: "ws_alpha",
    loop_name: "software-delivery",
    inputs: { slug: "helix-v1-launch" },
    input_mapping: { branch: "data.branch" },
  },
  enabled: true,
  retry: { strategy: "none", max_retries: 3, base_delay: "2s" },
  fire_limit: { max: 12, window: "1h" },
  webhook_secret_present: false,
  created_at: "2026-04-11T08:00:00Z",
  updated_at: "2026-04-11T08:10:00Z",
};

describe("AutomationTriggerRow", () => {
  it("Should summarize a loop-target trigger by Loop name and typed input instead of blank prompt", () => {
    render(<AutomationTriggerRow trigger={loopTriggerFixture} />);

    const row = screen.getByTestId("automation-item-trg_loop");
    expect(row).toHaveTextContent("Loop software-delivery");
    expect(row).toHaveTextContent("slug: helix-v1-launch");
  });
});

describe("AutomationJobRow", () => {
  it("Should render schedule meta and the run-now control keyed by job id", async () => {
    const user = userEvent.setup();
    const onRun = vi.fn();
    render(
      <AutomationJobRow isRunPending={false} job={jobFixture} onRun={onRun} runDisabled={false} />
    );

    const row = screen.getByTestId("automation-item-job_daily_review");
    expect(row).toHaveTextContent("Cron 0 9 * * *");

    await user.click(screen.getByTestId("automation-run-now-job_daily_review"));
    expect(onRun).toHaveBeenCalledWith("job_daily_review");
  });

  it("Should disable manual runs while the automation runtime is unavailable", () => {
    render(<AutomationJobRow isRunPending={false} job={jobFixture} onRun={vi.fn()} runDisabled />);

    expect(screen.getByTestId("automation-run-now-job_daily_review")).toBeDisabled();
  });
});

describe("AutomationCatalogShell", () => {
  function renderShell(
    overrides: Partial<Parameters<typeof AutomationCatalogShell>[0]> = {},
    children: ReactNode = <div data-testid="catalog-child" />
  ) {
    return render(
      <AutomationCatalogShell
        errorMessage={null}
        hasActiveFilters={false}
        isLoading={false}
        itemCount={1}
        kind="jobs"
        onClearFilters={vi.fn()}
        onCreate={vi.fn()}
        pagination={{ hasNextPage: false, isFetchingNextPage: false }}
        view="rows"
        {...overrides}
      >
        {children}
      </AutomationCatalogShell>
    );
  }

  it("Should load the next page explicitly while preserving rows and the page error", async () => {
    const user = userEvent.setup();
    const onLoadMore = vi.fn();
    renderShell({
      errorMessage: "Next page failed",
      pagination: { hasNextPage: true, isFetchingNextPage: false, onLoadMore },
    });

    expect(screen.getByTestId("catalog-child")).toBeInTheDocument();
    expect(screen.getByTestId("jobs-list-page-error")).toHaveTextContent("Next page failed");

    await user.click(screen.getByTestId("jobs-list-load-more"));
    expect(onLoadMore).toHaveBeenCalledTimes(1);
  });

  it("Should disable and mark the load-more control busy while fetching", () => {
    renderShell({
      pagination: { hasNextPage: true, isFetchingNextPage: true, onLoadMore: vi.fn() },
    });

    const loadMore = screen.getByTestId("jobs-list-load-more");
    expect(loadMore).toBeDisabled();
    expect(loadMore).toHaveAttribute("aria-busy", "true");
  });

  it("Should stretch the empty envelope so Empty can fill remaining listing height", () => {
    renderShell({ itemCount: 0 });

    const empty = screen.getByTestId("jobs-list-empty");
    expect(empty.className).toContain("flex-1");
    expect(empty.className).toContain("min-h-0");
    expect(empty.querySelector('[data-slot="empty"]')).toHaveAttribute("data-fill", "true");
  });

  it("Should preserve the selected card geometry while the catalog is loading", () => {
    renderShell({ isLoading: true, itemCount: 0, view: "cards" });

    expect(screen.getByRole("status", { name: "Loading jobs as cards" })).toBeInTheDocument();
    expect(screen.queryByRole("status", { name: "Loading jobs as rows" })).not.toBeInTheDocument();
  });
});
