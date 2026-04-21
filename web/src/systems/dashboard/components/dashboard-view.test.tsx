import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";
import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactElement } from "react";
import { describe, expect, it, vi } from "vitest";

import type { DashboardPayload } from "../types";
import { DashboardView } from "./dashboard-view";

function buildDashboard(overrides: Partial<DashboardPayload> = {}): DashboardPayload {
  return {
    workspace: {
      id: "ws-1",
      name: "one",
      root_dir: "/tmp/one",
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
    },
    daemon: {
      active_run_count: 0,
      http_port: 2123,
      pid: 42,
      started_at: "2026-01-01T00:00:00Z",
      version: "0.2.0",
      workspace_count: 1,
    },
    health: { ready: true, degraded: false, details: [] },
    pending_reviews: 2,
    queue: { active: 1, canceled: 0, completed: 5, failed: 2, total: 8 },
    workflows: [
      {
        workflow: { id: "wf-1", slug: "alpha", workspace_id: "ws-1" },
        active_runs: 1,
        review_round_count: 0,
        task_completed: 4,
        task_pending: 2,
        task_total: 6,
      },
    ],
    active_runs: [],
    ...overrides,
  } as DashboardPayload;
}

interface RenderProps {
  dashboard?: DashboardPayload;
  isRefetching?: boolean;
  isSyncing?: boolean;
  lastSyncError?: string | null;
  lastSyncMessage?: string | null;
  onSyncAll?: () => void;
}

async function renderDashboardView(props: RenderProps = {}): Promise<void> {
  const rootRoute = createRootRoute();
  const indexRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: function IndexRouteComponent(): ReactElement {
      return (
        <DashboardView
          dashboard={props.dashboard ?? buildDashboard()}
          isRefetching={props.isRefetching ?? false}
          isSyncing={props.isSyncing ?? false}
          lastSyncError={props.lastSyncError ?? null}
          lastSyncMessage={props.lastSyncMessage ?? null}
          onSyncAll={props.onSyncAll ?? (() => {})}
        />
      );
    },
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([indexRoute]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    defaultPreload: false,
  });
  await router.load();
  await act(async () => {
    render(<RouterProvider router={router} />);
    await Promise.resolve();
  });
}

describe("DashboardView", () => {
  it("Should render pending reviews, queue stats, and workflow rows", async () => {
    await renderDashboardView();
    expect(await screen.findByTestId("dashboard-view")).toBeInTheDocument();
    expect(screen.getByTestId("dashboard-queue-active")).toHaveTextContent("1");
    expect(screen.getByTestId("dashboard-workflow-row-alpha")).toBeInTheDocument();
  });

  it("Should fire the sync-all action", async () => {
    const onSync = vi.fn();
    await renderDashboardView({ onSyncAll: onSync });
    await userEvent.click(screen.getByTestId("dashboard-sync-all"));
    expect(onSync).toHaveBeenCalledTimes(1);
  });

  it("Should show a success banner when sync completes", async () => {
    await renderDashboardView({
      lastSyncMessage: "Sync completed — 2 workflows scanned.",
    });
    expect(screen.getByTestId("dashboard-sync-success")).toHaveTextContent("Sync completed");
  });

  it("Should show an error banner when sync fails", async () => {
    await renderDashboardView({
      lastSyncError: "Sync failed — server error",
    });
    expect(screen.getByTestId("dashboard-sync-error")).toHaveTextContent("server error");
  });

  it("Should show the empty state when no workflows exist", async () => {
    await renderDashboardView({ dashboard: buildDashboard({ workflows: [] }) });
    expect(await screen.findByTestId("dashboard-workflows-empty")).toBeInTheDocument();
  });
});
