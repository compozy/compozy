import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { Workspace } from "../types";
import { WorkspacePicker } from "./workspace-picker";

const presentWorkspace: Workspace = {
  id: "ws-present",
  name: "present",
  root_dir: "/tmp/present",
  filesystem_state: "present",
  read_only: false,
  has_catalog_data: true,
  workflow_count: 1,
  run_count: 0,
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
};

const missingWorkspace: Workspace = {
  id: "ws-missing",
  name: "missing",
  root_dir: "/tmp/missing",
  filesystem_state: "missing",
  read_only: true,
  has_catalog_data: true,
  workflow_count: 1,
  run_count: 2,
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
};

describe("WorkspacePicker", () => {
  it("Should group path-missing workspaces at the bottom without disabling selection", async () => {
    const onSelect = vi.fn();
    render(
      <WorkspacePicker onSelect={onSelect} workspaces={[missingWorkspace, presentWorkspace]} />
    );

    const availableSection = screen.getByTestId("workspace-picker-available");
    const missingSection = screen.getByTestId("workspace-picker-missing-section");
    expect(availableSection).toHaveTextContent("Available · 1");
    expect(availableSection).toHaveTextContent("present");
    expect(availableSection).not.toHaveTextContent("missing");
    expect(missingSection).toHaveTextContent("Path missing · 1");
    expect(missingSection).toHaveTextContent("missing");
    expect(missingSection.compareDocumentPosition(availableSection)).toBe(
      Node.DOCUMENT_POSITION_PRECEDING
    );
    expect(
      within(missingSection).getByTestId("workspace-picker-missing-ws-missing")
    ).toHaveTextContent("path missing");
    expect(
      within(missingSection).getByTestId("workspace-picker-readonly-ws-missing")
    ).toHaveTextContent("read-only");

    await userEvent.click(within(missingSection).getByTestId("workspace-picker-select-ws-missing"));
    expect(onSelect).toHaveBeenCalledWith("ws-missing");
  });

  it("Should expose manual sync controls and status messages", async () => {
    const onSync = vi.fn();
    render(
      <WorkspacePicker
        onSelect={vi.fn()}
        onSync={onSync}
        syncError="Sync failed"
        syncMessage="2 checked · 1 synced"
        workspaces={[presentWorkspace]}
      />
    );

    await userEvent.click(screen.getByTestId("workspace-picker-sync"));
    expect(onSync).toHaveBeenCalledTimes(1);
    expect(screen.getByTestId("workspace-picker-sync-success")).toHaveTextContent("2 checked");
    expect(screen.getByTestId("workspace-picker-sync-error")).toHaveTextContent("Sync failed");
  });
});
