import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";
import { act, render, screen } from "@testing-library/react";
import type { ReactElement } from "react";
import { describe, expect, it } from "vitest";

import { MemoryIndexView } from "./memory-index-view";
import type { WorkflowSummary } from "../types";

const workflows: WorkflowSummary[] = [
  {
    id: "wf-1",
    slug: "alpha",
    workspace_id: "ws-1",
    last_synced_at: "2026-01-02T00:00:00Z",
  },
  {
    id: "wf-2",
    slug: "beta",
    workspace_id: "ws-1",
    archived_at: "2026-01-02T00:00:00Z",
  },
];

interface RenderProps {
  workflows?: WorkflowSummary[];
  isLoading?: boolean;
  isRefetching?: boolean;
  error?: string | null;
}

async function renderIndex(props: RenderProps = {}) {
  const rootRoute = createRootRoute();
  const indexRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: function IndexRoute(): ReactElement {
      return (
        <MemoryIndexView
          error={props.error ?? null}
          isLoading={props.isLoading ?? false}
          isRefetching={props.isRefetching ?? false}
          workflows={props.workflows ?? workflows}
          workspaceName="one"
        />
      );
    },
  });
  const memoryRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/memory/$slug",
    component: function MemoryStub(): ReactElement {
      return <div data-testid="memory-stub" />;
    },
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([indexRoute, memoryRoute]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    defaultPreload: false,
  });
  await router.load();
  await act(async () => {
    render(<RouterProvider router={router} />);
    await Promise.resolve();
  });
}

describe("MemoryIndexView", () => {
  it("Should render workflow cards with deep links", async () => {
    await renderIndex();
    expect(screen.getByTestId("memory-index-view")).toBeInTheDocument();
    expect(screen.getByTestId("memory-index-card-alpha")).toBeInTheDocument();
    const link = screen.getByTestId("memory-index-open-alpha") as HTMLAnchorElement;
    expect(link.getAttribute("href")).toBe("/memory/alpha");
  });

  it("Should render the loading state", async () => {
    await renderIndex({ workflows: [], isLoading: true });
    expect(screen.getByTestId("memory-index-loading")).toBeInTheDocument();
  });

  it("Should render the empty state when no workflows exist", async () => {
    await renderIndex({ workflows: [] });
    expect(screen.getByTestId("memory-index-empty")).toBeInTheDocument();
  });

  it("Should render the error alert", async () => {
    await renderIndex({ workflows: [], error: "workspace stale" });
    expect(screen.getByTestId("memory-index-error")).toHaveTextContent("workspace stale");
  });

  it("Should render the refreshing indicator", async () => {
    await renderIndex({ isRefetching: true });
    expect(screen.getByTestId("memory-index-refreshing")).toBeInTheDocument();
  });
});
