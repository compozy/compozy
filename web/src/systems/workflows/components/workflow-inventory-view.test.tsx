import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
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

describe("WorkflowInventoryView", () => {
  it("Should render active and archived workflows in separate sections", () => {
    render(
      <WorkflowInventoryView
        {...defaults}
        onArchive={() => {}}
        onSyncAll={() => {}}
        onSyncOne={() => {}}
        workflows={workflows}
      />
    );
    expect(screen.getByTestId("workflow-inventory-active")).toHaveTextContent("alpha");
    expect(screen.getByTestId("workflow-inventory-archived")).toHaveTextContent("beta");
  });

  it("Should show the empty state when no workflows exist", () => {
    render(
      <WorkflowInventoryView
        {...defaults}
        onArchive={() => {}}
        onSyncAll={() => {}}
        onSyncOne={() => {}}
        workflows={[]}
      />
    );
    expect(screen.getByTestId("workflow-inventory-empty")).toBeInTheDocument();
  });

  it("Should fire sync-all, sync-one, and archive handlers", async () => {
    const onSyncAll = vi.fn();
    const onSyncOne = vi.fn();
    const onArchive = vi.fn();
    render(
      <WorkflowInventoryView
        {...defaults}
        onArchive={onArchive}
        onSyncAll={onSyncAll}
        onSyncOne={onSyncOne}
        workflows={[workflows[0]!]}
      />
    );
    await userEvent.click(screen.getByTestId("workflow-inventory-sync-all"));
    await userEvent.click(screen.getByTestId("workflow-sync-alpha"));
    await userEvent.click(screen.getByTestId("workflow-archive-alpha"));
    expect(onSyncAll).toHaveBeenCalledTimes(1);
    expect(onSyncOne).toHaveBeenCalledWith("alpha");
    expect(onArchive).toHaveBeenCalledWith("alpha");
  });

  it("Should surface load and action errors", () => {
    render(
      <WorkflowInventoryView
        {...defaults}
        error="load failed"
        lastActionError="sync blew up"
        onArchive={() => {}}
        onSyncAll={() => {}}
        onSyncOne={() => {}}
        workflows={workflows}
      />
    );
    expect(screen.getByTestId("workflow-inventory-load-error")).toHaveTextContent("load failed");
    expect(screen.getByTestId("workflow-inventory-error")).toHaveTextContent("sync blew up");
  });
});
