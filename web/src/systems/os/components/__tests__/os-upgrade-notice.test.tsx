// Suite: upgrade notice
// Invariant: the notice shows while the migration status reports an unacknowledged migration,
// with the counts the upgrade reported; dismissing posts `ack` and hides it at once; a status
// acknowledged by another client hides it on the next read; a failed migration offers Retry and
// shows "Retrying" while it runs (US-017.AC-6, EC-6, EC-8).
// Owning layer: OsUpgradeNotice + useOsUpgradeNotice over the shell projection and the migration
// status query. Boundary OUT: the migration status HTTP adapter (mocked at the I/O boundary).
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type * as React from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { DeskProfileContext } from "../../contexts/desk-profile-context";
import { OsShellContext, type OsShellHandle } from "../../contexts/os-shell-context";
import type { OsDesktopRuntimeStore } from "../../lib/os-types";
import { createWindowManagerProjectionAtom } from "../../lib/window-manager-projection";
import type { WindowManagerMigrationStatus } from "../../lib/window-manager-types";
import { OsUpgradeNotice, upgradeNoticeDescription } from "../os-upgrade-notice";

const api = vi.hoisted(() => ({
  fetchWindowManagerMigration: vi.fn(),
  updateWindowManagerMigration: vi.fn(),
}));

vi.mock("../../adapters/window-manager-api", async importOriginal => ({
  ...(await importOriginal<typeof import("../../adapters/window-manager-api")>()),
  fetchWindowManagerMigration: api.fetchWindowManagerMigration,
  updateWindowManagerMigration: api.updateWindowManagerMigration,
}));

const SUMMARY = {
  desktops: 2,
  tabs: 7,
  windows: 9,
  floatingToTabs: 3,
  minimizedToTabs: 2,
  islandsMerged: 1,
  pinsDropped: 1,
  zoomDesktopsMerged: 0,
};

function status(
  overrides: Partial<WindowManagerMigrationStatus> = {}
): WindowManagerMigrationStatus {
  return {
    state: "migrated",
    fromVersion: 5,
    at: "2026-11-12T15:00:00Z",
    acknowledged: false,
    summary: SUMMARY,
    error: null,
    failureRevision: null,
    ...overrides,
  };
}

function renderNotice(migration: WindowManagerMigrationStatus | null) {
  const state = {
    snapshot: { workspaceId: "workspace:test", migration },
  } as unknown as OsDesktopRuntimeStore;
  const shell = {
    projection: createWindowManagerProjectionAtom(state),
  } as unknown as OsShellHandle;
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>
      <OsShellContext.Provider value={shell}>
        <DeskProfileContext value="default">{children}</DeskProfileContext>
      </OsShellContext.Provider>
    </QueryClientProvider>
  );
  return render(<OsUpgradeNotice />, { wrapper });
}

beforeEach(() => {
  api.fetchWindowManagerMigration.mockReset();
  api.updateWindowManagerMigration.mockReset();
});

describe("upgradeNoticeDescription", () => {
  it("Should name what became tabs and the pins the upgrade dropped", () => {
    expect(upgradeNoticeDescription(SUMMARY)).toBe(
      "3 floating windows and 2 minimized windows are tabs now. 1 pin inside a split was removed."
    );
    expect(
      upgradeNoticeDescription({
        ...SUMMARY,
        floatingToTabs: 1,
        minimizedToTabs: 0,
        pinsDropped: 2,
      })
    ).toBe("1 floating window is a tab now. 2 pins inside splits were removed.");
    expect(
      upgradeNoticeDescription({
        ...SUMMARY,
        floatingToTabs: 0,
        minimizedToTabs: 0,
        pinsDropped: 0,
      })
    ).toBe("Floating and minimized windows are tabs now.");
  });
});

describe("OsUpgradeNotice", () => {
  it("Should show the counts until dismissed, then post ack and hide at once", async () => {
    const user = userEvent.setup();
    api.fetchWindowManagerMigration.mockResolvedValue(status());
    api.updateWindowManagerMigration.mockResolvedValue(status({ acknowledged: true }));
    renderNotice(status());

    expect(await screen.findByText("Your windows are now tabs")).toBeInTheDocument();
    expect(screen.getByText(/3 floating windows and 2 minimized windows/)).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Dismiss" }));
    expect(api.updateWindowManagerMigration).toHaveBeenCalledWith(
      "workspace:test",
      "default",
      "ack"
    );
    await waitFor(() => expect(screen.queryByText("Your windows are now tabs")).toBeNull());
  });

  it("Should stay hidden when another client already acknowledged it", async () => {
    api.fetchWindowManagerMigration.mockResolvedValue(status({ acknowledged: true }));
    renderNotice(status());

    await waitFor(() => expect(api.fetchWindowManagerMigration).toHaveBeenCalled());
    expect(screen.queryByText("Your windows are now tabs")).toBeNull();
  });

  it("Should ask for nothing while the snapshot reports no pending migration", () => {
    renderNotice(null);
    expect(api.fetchWindowManagerMigration).not.toHaveBeenCalled();
    expect(screen.queryByRole("button", { name: "Dismiss" })).toBeNull();
  });

  it("Should offer Retry on a failed migration and show it running", async () => {
    const user = userEvent.setup();
    const failed = status({
      state: "failed",
      summary: null,
      error: { code: "window_manager_invalid_snapshot", path: null, message: "bad tree" },
      failureRevision: 4,
    });
    api.fetchWindowManagerMigration.mockResolvedValue(failed);
    let finishRetry: (value: WindowManagerMigrationStatus) => void = () => {};
    api.updateWindowManagerMigration.mockReturnValue(
      new Promise(resolve => {
        finishRetry = resolve;
      })
    );
    renderNotice(failed);

    expect(await screen.findByText("We could not move your layout to tabs")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Retry" }));
    expect(api.updateWindowManagerMigration).toHaveBeenCalledWith(
      "workspace:test",
      "default",
      "retry"
    );
    expect(await screen.findByRole("button", { name: "Retrying" })).toBeDisabled();

    finishRetry(status());
    expect(await screen.findByText("Your windows are now tabs")).toBeInTheDocument();
  });
});
