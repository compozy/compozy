// Suite: Profile lifecycle dialogs
// Invariant: A daemon-blocked lifecycle plan never exposes an actionable confirmation.
// Boundary IN: Archive/delete dialog behavior for returned plan blockers.
// Boundary OUT: Plan calculation and mutation enforcement owned by the Go profile suites.

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { ProfileArchiveDialog } from "../profile-archive-dialog";
import { ProfileDeleteDialog } from "../profile-delete-dialog";
import { ProfileRenameDialog } from "../profile-rename-dialog";
import { ProfileUnarchiveDialog } from "../profile-unarchive-dialog";

const EMPTY_REMOVAL = {
  agents: 0,
  config_keys: 0,
  credential_overrides: 0,
  desktop_partitions: 0,
  event_summaries: 0,
  loops: 0,
  mcp_servers: 0,
  memory_entries: 0,
  palette_pins: 0,
  palette_query_hits: 0,
  palette_usage: 0,
  skills: 0,
  terminal_approvals: 0,
};

describe("Profile lifecycle dialogs", () => {
  it("Should block archive while a run lease is active", async () => {
    const user = userEvent.setup();
    const onArchive = vi.fn();
    render(
      <ProfileArchiveDialog
        open
        onOpenChange={vi.fn()}
        profile="marketing"
        plan={{
          approval_blockers: [],
          automations_to_pause: [],
          leased_runs: 2,
          queued_runs_to_freeze: 0,
          revision: "archive-revision",
          running_sessions: [],
        }}
        planLoading={false}
        isPending={false}
        onArchive={onArchive}
      />
    );

    expect(screen.getByText(/2 leased runs are still active/)).toBeInTheDocument();
    const confirm = screen.getByTestId("profile-archive-confirm");
    expect(confirm).toBeDisabled();
    await user.click(confirm);
    expect(onArchive).not.toHaveBeenCalled();
  });

  it("Should block delete while an approval still owns profile work", async () => {
    const user = userEvent.setup();
    const onDelete = vi.fn();
    render(
      <ProfileDeleteDialog
        open
        onOpenChange={vi.fn()}
        profile="marketing"
        plan={{
          approval_blockers: ["approval-42"],
          removed: EMPTY_REMOVAL,
          revision: "delete-revision",
          selections_to_sweep: 0,
        }}
        planLoading={false}
        workItems={0}
        isPending={false}
        onDelete={onDelete}
        onArchiveInstead={vi.fn()}
      />
    );

    expect(screen.getByText("approval-42")).toBeInTheDocument();
    const confirm = screen.getByTestId("profile-delete-confirm");
    expect(confirm).toBeDisabled();
    await user.click(confirm);
    expect(onDelete).not.toHaveBeenCalled();
  });

  it("Should disable rename while its current plan is refetching", () => {
    render(
      <ProfileRenameDialog
        open
        onOpenChange={vi.fn()}
        profile="marketing"
        newName="growth"
        onNewNameChange={vi.fn()}
        plan={{
          revision: "rename-revision",
          machine_folders: [],
          repo_candidates: [],
          dormant_placements: [],
          vault_ref_rewrites: 0,
        }}
        planLoading
        acceptedRepos={[]}
        onToggleRepo={vi.fn()}
        isPending={false}
        onRename={vi.fn()}
      />
    );

    expect(screen.getByTestId("profile-rename-confirm")).toBeDisabled();
  });

  it("Should disable archive and delete while their plans are refetching", () => {
    const archive = render(
      <ProfileArchiveDialog
        open
        onOpenChange={vi.fn()}
        profile="marketing"
        plan={{
          approval_blockers: [],
          automations_to_pause: [],
          leased_runs: 0,
          queued_runs_to_freeze: 0,
          revision: "archive-revision",
          running_sessions: [],
        }}
        planLoading
        isPending={false}
        onArchive={vi.fn()}
      />
    );
    expect(screen.getByTestId("profile-archive-confirm")).toBeDisabled();
    archive.unmount();

    render(
      <ProfileDeleteDialog
        open
        onOpenChange={vi.fn()}
        profile="marketing"
        plan={{
          approval_blockers: [],
          removed: EMPTY_REMOVAL,
          revision: "delete-revision",
          selections_to_sweep: 0,
        }}
        planLoading
        workItems={0}
        isPending={false}
        onDelete={vi.fn()}
        onArchiveInstead={vi.fn()}
      />
    );
    expect(screen.getByTestId("profile-delete-confirm")).toBeDisabled();
  });

  it("Should reactivate each paused automation only after the daemon accepts it", async () => {
    const user = userEvent.setup();
    const onSetAutomationEnabled = vi.fn().mockResolvedValue(undefined);
    render(
      <ProfileUnarchiveDialog
        open
        onOpenChange={vi.fn()}
        profile="marketing"
        pausedAutomations={["job:weekly-digest", "trigger:invoice-reminder"]}
        isPending={false}
        onUnarchive={vi.fn()}
        onSetAutomationEnabled={onSetAutomationEnabled}
        onDone={vi.fn()}
      />
    );

    const list = screen.getByTestId("profile-unarchive-paused");
    expect(list.closest('[data-slot="dialog-description"]')).toBeNull();
    expect(list).toHaveTextContent("weekly-digest");
    expect(list).toHaveTextContent("invoice-reminder");

    const weekly = screen.getByRole("switch", { name: "Reactivate weekly-digest" });
    expect(weekly).not.toBeChecked();
    await user.click(weekly);

    await waitFor(() =>
      expect(onSetAutomationEnabled).toHaveBeenCalledWith("job:weekly-digest", true)
    );
    expect(screen.getByRole("switch", { name: "Pause weekly-digest" })).toBeChecked();
  });
});
