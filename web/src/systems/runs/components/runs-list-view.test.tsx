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

import { RunsListView, resolveStatusTone } from "./runs-list-view";
import type { Run, RunListModeFilter, RunListStatusFilter } from "../types";

interface RenderProps {
  runs?: Run[];
  isLoading?: boolean;
  error?: string | null;
  degradedReason?: string | null;
  isRefetching?: boolean;
  statusFilter?: RunListStatusFilter;
  modeFilter?: RunListModeFilter;
  onStatusChange?: (next: RunListStatusFilter) => void;
  onModeChange?: (next: RunListModeFilter) => void;
}

async function renderRunsList(props: RenderProps = {}) {
  const rootRoute = createRootRoute();
  const indexRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: function IndexRouteComponent(): ReactElement {
      return (
        <RunsListView
          degradedReason={props.degradedReason ?? null}
          error={props.error ?? null}
          isLoading={props.isLoading ?? false}
          isRefetching={props.isRefetching ?? false}
          modeFilter={props.modeFilter ?? "all"}
          onModeChange={props.onModeChange ?? (() => {})}
          onStatusChange={props.onStatusChange ?? (() => {})}
          runs={props.runs ?? []}
          statusFilter={props.statusFilter ?? "all"}
          workspaceName="one"
        />
      );
    },
  });
  const detailRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/runs/$runId",
    component: function DetailRouteComponent(): ReactElement {
      return <div data-testid="detail-stub" />;
    },
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([indexRoute, detailRoute]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    defaultPreload: false,
  });
  await router.load();
  await act(async () => {
    render(<RouterProvider router={router} />);
    await Promise.resolve();
  });
}

const runs: Run[] = [
  {
    run_id: "run-1",
    mode: "task",
    presentation_mode: "text",
    workspace_id: "ws-1",
    started_at: "2026-01-01T00:00:00Z",
    status: "running",
    workflow_slug: "alpha",
  },
  {
    run_id: "run-2",
    mode: "review",
    presentation_mode: "text",
    workspace_id: "ws-1",
    started_at: "2026-01-01T00:01:00Z",
    ended_at: "2026-01-01T00:02:00Z",
    status: "failed",
    workflow_slug: "beta",
    error_text: "exit 1",
  },
];

describe("RunsListView", () => {
  it("Should render run rows with status badges and workflow links", async () => {
    await renderRunsList({ runs });
    expect(screen.getByTestId("runs-list-items")).toBeInTheDocument();
    expect(screen.getByTestId("runs-list-row-run-1")).toHaveTextContent("alpha");
    expect(screen.getByTestId("runs-list-status-run-1")).toHaveTextContent("running");
    expect(screen.getByTestId("runs-list-error-run-2")).toHaveTextContent("exit 1");
  });

  it("Should render the empty state when no runs match filters", async () => {
    await renderRunsList({ runs: [] });
    expect(screen.getByTestId("runs-list-empty")).toBeInTheDocument();
  });

  it("Should render the loading state without rows", async () => {
    await renderRunsList({ runs: [], isLoading: true });
    expect(screen.getByTestId("runs-list-loading")).toBeInTheDocument();
  });

  it("Should render an error alert when the query fails", async () => {
    await renderRunsList({ error: "boom", runs: [] });
    expect(screen.getByTestId("runs-list-error")).toHaveTextContent("boom");
  });

  it("Should render a degraded notice when provided", async () => {
    await renderRunsList({ degradedReason: "daemon is degraded", runs });
    expect(screen.getByTestId("runs-list-degraded")).toHaveTextContent("daemon is degraded");
  });

  it("Should fire filter change handlers", async () => {
    const onStatusChange = vi.fn();
    const onModeChange = vi.fn();
    await renderRunsList({ runs, onStatusChange, onModeChange });
    await userEvent.selectOptions(screen.getByTestId("runs-filter-status"), "active");
    await userEvent.selectOptions(screen.getByTestId("runs-filter-mode"), "task");
    expect(onStatusChange).toHaveBeenCalledWith("active");
    expect(onModeChange).toHaveBeenCalledWith("task");
  });
});

describe("resolveStatusTone", () => {
  it("Should return tones for each status family", () => {
    expect(resolveStatusTone("running")).toBe("accent");
    expect(resolveStatusTone("completed")).toBe("success");
    expect(resolveStatusTone("failed")).toBe("danger");
    expect(resolveStatusTone("canceled")).toBe("warning");
    expect(resolveStatusTone("unknown")).toBe("info");
  });
});
