// Suite: home dashboard loading shapes
// Invariant: a Home zone whose read has not answered shows its loading shape under one named busy
// region and never claims a zero or an empty list; a read that already has data keeps the loaded
// zone through a refetch.
// Boundary IN: the overview's loading branch and the zones that read on their own.
// Boundary OUT: query composition and scope isolation (use-home-dashboard.test.tsx).
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { statusFixture } from "@/systems/status/mocks";

import type { HomeDashboardModel } from "../hooks/use-home-dashboard";
import { buildHomeSystemModel } from "../lib/home-system";
import type { HomeActivityEvent, HomeRunCardModel } from "../types";

vi.mock("@tanstack/react-router", async importOriginal => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    Link: ({
      to,
      params: _params,
      search: _search,
      children,
      ...props
    }: Record<string, unknown>) => (
      <a href={typeof to === "string" ? to : "#"} {...(props as Record<string, unknown>)}>
        {children as React.ReactNode}
      </a>
    ),
  };
});

vi.mock("@/systems/session/hooks/use-session-create", () => ({
  useSessionCreateActions: () => ({ openForAgent: vi.fn() }),
}));

const dashboardModel = vi.hoisted(() => ({ current: null as unknown }));
vi.mock("../hooks/use-home-dashboard", () => ({
  useHomeDashboard: () => dashboardModel.current,
}));

const { DashboardWindowSkeleton } = await import("../components/dashboard-window-skeleton");
const { HomeActivityFeed } = await import("../components/home-activity-feed");
const { HomeAgentsPanel } = await import("../components/home-agents-panel");
const { HomeDashboard } = await import("../components/home-dashboard");
const { HomeKpiStrip } = await import("../components/home-kpi-strip");
const { HomeSystemPanel } = await import("../components/home-system-panel");
const { HomeWorkingNow } = await import("../components/home-working-now");
const { makeHomeOverview } = await import("../mocks/fixtures");

const AGENT_ROW = {
  name: "writer",
  active: 1,
  sessions: 12,
  failed: 0,
  runtimeSeconds: 7200,
  runtimeShare: 1,
};
const ACTIVITY_EVENT: HomeActivityEvent = {
  id: "evt-1",
  type: "session.started",
  summary: "started a session",
  agent_name: "writer",
  timestamp: "2026-07-23T11:30:00Z",
} as HomeActivityEvent;
const RUN_CARD: HomeRunCardModel = {
  key: "session:one",
  kind: "session",
  agentName: "writer",
  title: "Draft the launch post",
  subtitle: "Working on the current turn",
  elapsedBaseSeconds: 60,
  baseAtMs: Date.parse("2026-07-23T12:00:00Z"),
  sessionLink: { agentName: "writer", sessionId: "sess-1" },
};
const SYSTEM = buildHomeSystemModel(statusFixture, {
  hookRunsToday: 4,
  hookFailuresToday: 0,
  retentionDays: 30,
});

function loadingDashboardModel(): HomeDashboardModel {
  return {
    scope: { dataScope: { kind: "all" } },
    activeWorkspaceName: "All projects",
    usageWindow: 7,
    overview: undefined,
    overviewStatus: "loading",
    overviewErrorMessage: null,
  } as unknown as HomeDashboardModel;
}

describe("Home dashboard loading shapes", () => {
  beforeEach(() => {
    dashboardModel.current = loadingDashboardModel();
  });

  it("Should hold the whole dashboard under one named busy region that keeps the page meta", () => {
    const { container } = render(<HomeDashboard />);

    expect(screen.getAllByRole("status")).toHaveLength(1);
    expect(screen.getByRole("status")).toHaveTextContent("Loading home dashboard");
    expect(container.querySelector('[data-slot="home-page-meta"]')).not.toBeNull();
    // The usage tile follows the window the person picked, not a default.
    expect(screen.getByText("Usage, last 7 days")).toBeInTheDocument();
    // Nothing that waits for a loaded zone can mistake a placeholder for it.
    expect(container.querySelector('[data-slot="home-kpi-strip"]')).toBeNull();
    expect(container.querySelector('[data-slot="home-pulse"]')).toBeNull();
  });

  it("Should show the Home window as one busy region with its date line before any code or data", () => {
    const { container } = render(<DashboardWindowSkeleton />);

    expect(screen.getAllByRole("status")).toHaveLength(1);
    expect(screen.getByRole("status")).toHaveTextContent("Loading home dashboard");
    expect(container.querySelector('[data-slot="home-page-meta"]')).not.toBeNull();
    expect(container.querySelector("a, button, input, [tabindex]")).toBeNull();
  });

  it.each([
    {
      zone: "agents",
      region: "Loading agents",
      claim: "No agents in this project",
      pending: <HomeAgentsPanel loading rows={[]} />,
      refetching: <HomeAgentsPanel loading rows={[AGENT_ROW]} />,
      kept: "writer",
    },
    {
      zone: "activity",
      region: "Loading activity",
      claim: "No recent activity",
      pending: <HomeActivityFeed events={[]} status="loading" />,
      refetching: <HomeActivityFeed events={[ACTIVITY_EVENT]} status="loading" />,
      kept: "started a session",
    },
    {
      zone: "live work",
      region: "Loading live work",
      claim: "No agents working right now.",
      pending: <HomeWorkingNow cards={[]} status="loading" total={0} />,
      refetching: <HomeWorkingNow cards={[RUN_CARD]} status="loading" total={1} />,
      kept: "Draft the launch post",
    },
  ])(
    "Should wait on its own read in the $zone zone instead of claiming it is empty",
    ({ region, claim, pending, refetching, kept }) => {
      const { rerender } = render(pending);

      expect(screen.getByRole("status")).toHaveTextContent(region);
      expect(screen.queryByText(claim)).not.toBeInTheDocument();

      rerender(refetching);

      expect(screen.queryByRole("status")).not.toBeInTheDocument();
      expect(screen.getByText(kept, { exact: false })).toBeInTheDocument();
    }
  );

  it("Should wait on the daemon status before judging the system row", () => {
    const props = { onOpenChange: vi.fn(), open: false, system: SYSTEM };
    const { rerender } = render(<HomeSystemPanel {...props} loading />);

    expect(screen.getByRole("status")).toHaveTextContent("Loading system status");
    expect(screen.queryByText("Needs a look")).not.toBeInTheDocument();
    expect(screen.queryByText("All systems normal")).not.toBeInTheDocument();

    rerender(<HomeSystemPanel {...props} />);

    expect(screen.queryByRole("status")).not.toBeInTheDocument();
    expect(
      screen.getByText(SYSTEM.allNormal ? "All systems normal" : "Needs a look")
    ).toBeVisible();
  });

  it("Should not report zero live work while the live-work reads are out", () => {
    const overview = makeHomeOverview();
    const props = { overview, workingNowDetail: "no live work", workingNowTotal: 0 };
    const { rerender } = render(<HomeKpiStrip {...props} workingNowLoading />);

    expect(screen.queryByText("no live work")).not.toBeInTheDocument();
    expect(screen.getByText("Working now")).toBeInTheDocument();

    rerender(<HomeKpiStrip {...props} />);

    expect(screen.getByText("no live work")).toBeInTheDocument();
  });
});
