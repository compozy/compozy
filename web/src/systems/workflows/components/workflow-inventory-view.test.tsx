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

import type { WorkflowSummary } from "../types";
import { WorkflowInventoryView } from "./workflow-inventory-view";

const workflows: WorkflowSummary[] = [
  { id: "wf-1", slug: "alpha", workspace_id: "ws-1", last_synced_at: "2026-01-01T00:00:00Z" },
  {
    id: "wf-2",
    slug: "beta",
    workspace_id: "ws-1",
    archived_at: "2026-02-01T00:00:00Z",
  },
];

const defaults = {
  isLoading: false,
  isRefetching: false,
  workspaceName: "one",
  isSyncingAll: false,
  pendingSyncSlug: null,
  pendingArchiveSlug: null,
};

type ViewProps = Parameters<typeof WorkflowInventoryView>[0];

async function renderInventory(props: ViewProps) {
  const rootRoute = createRootRoute();
  const indexRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: function IndexRoute(): ReactElement {
      return <WorkflowInventoryView {...props} />;
    },
  });
  const boardRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/workflows/$slug/tasks",
    component: function BoardStub(): ReactElement {
      return <div data-testid="board-stub" />;
    },
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([indexRoute, boardRoute]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    defaultPreload: false,
  });
  await router.load();
  await act(async () => {
    render(<RouterProvider router={router} />);
    await Promise.resolve();
  });
}

describe("WorkflowInventoryView", () => {
  it("Should render active and archived workflows in separate sections", async () => {
    await renderInventory({
      ...defaults,
      onArchive: () => {},
      onSyncAll: () => {},
      onSyncOne: () => {},
      workflows,
    });
    expect(screen.getByTestId("workflow-inventory-active")).toHaveTextContent("alpha");
    expect(screen.getByTestId("workflow-inventory-archived")).toHaveTextContent("beta");
    const openLink = screen.getByTestId("workflow-view-board-alpha") as HTMLAnchorElement;
    expect(openLink.getAttribute("href")).toBe("/workflows/alpha/tasks");
  });

  it("Should show the empty state when no workflows exist", async () => {
    await renderInventory({
      ...defaults,
      onArchive: () => {},
      onSyncAll: () => {},
      onSyncOne: () => {},
      workflows: [],
    });
    expect(screen.getByTestId("workflow-inventory-empty")).toBeInTheDocument();
  });

  it("Should fire sync-all, sync-one, and archive handlers", async () => {
    const onSyncAll = vi.fn();
    const onSyncOne = vi.fn();
    const onArchive = vi.fn();
    await renderInventory({
      ...defaults,
      onArchive,
      onSyncAll,
      onSyncOne,
      workflows: [workflows[0]!],
    });
    await userEvent.click(screen.getByTestId("workflow-inventory-sync-all"));
    await userEvent.click(screen.getByTestId("workflow-sync-alpha"));
    await userEvent.click(screen.getByTestId("workflow-archive-alpha"));
    expect(onSyncAll).toHaveBeenCalledTimes(1);
    expect(onSyncOne).toHaveBeenCalledWith("alpha");
    expect(onArchive).toHaveBeenCalledWith("alpha");
  });

  it("Should surface load and action errors", async () => {
    await renderInventory({
      ...defaults,
      error: "load failed",
      lastActionError: "sync blew up",
      onArchive: () => {},
      onSyncAll: () => {},
      onSyncOne: () => {},
      workflows,
    });
    expect(screen.getByTestId("workflow-inventory-load-error")).toHaveTextContent("load failed");
    expect(screen.getByTestId("workflow-inventory-error")).toHaveTextContent("sync blew up");
  });
});
