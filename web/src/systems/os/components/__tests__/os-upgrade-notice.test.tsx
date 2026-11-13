// Suite: upgrade notice
// Invariant: the notice shows while the snapshot's `migration` reports an unacknowledged
// migration, with the counts the upgrade reported; dismissing posts `ack` and hides it at once
// without coming back before or after the frame; a snapshot without `migration` (another client's
// ack) hides it; a failed migration offers Retry and shows "Retrying" while it runs; a retry the
// daemon would refuse because the layout changed since the failure (or a 409 that still arrives)
// is offered as a confirmed "Replace current layout": confirming sends `replace: true` once,
// cancelling sends nothing (US-017.AC-6, EC-6, EC-8; operator decision Q1 — a).
// Owning layer: OsUpgradeNotice + useOsUpgradeNotice over the shell projection of the cached
// snapshot. Boundary OUT: the migration HTTP adapter (mocked at the I/O boundary); the stream's
// same-revision frame handling is owned by the stream hook suite.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type * as React from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { WindowManagerApiError } from "../../adapters/window-manager-api";
import { DeskProfileContext } from "../../contexts/desk-profile-context";
import { OsShellContext, type OsShellHandle } from "../../contexts/os-shell-context";
import type { OsDesktopRuntimeStore } from "../../lib/os-types";
import { createWindowManagerProjectionAtom } from "../../lib/window-manager-projection";
import { windowManagerKeys } from "../../lib/window-manager-query";
import type {
  WindowManagerMigrationStatus,
  WindowManagerSnapshot,
} from "../../lib/window-manager-types";
import { upgradeNoticeDescription } from "../../lib/upgrade-notice-description";
import { OsUpgradeNotice } from "../os-upgrade-notice";

const api = vi.hoisted(() => ({ updateWindowManagerMigration: vi.fn() }));

vi.mock("../../adapters/window-manager-api", async importOriginal => ({
  ...(await importOriginal<typeof import("../../adapters/window-manager-api")>()),
  updateWindowManagerMigration: api.updateWindowManagerMigration,
}));

const SNAPSHOT_KEY = windowManagerKeys.snapshot("workspace:test", "default");
/** The revision of the rendered snapshot; a failure left at it can still be retried plainly. */
const REVISION = 7;

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

function snapshotWith(migration: WindowManagerMigrationStatus | null): WindowManagerSnapshot {
  return { workspaceId: "workspace:test", revision: REVISION, migration } as WindowManagerSnapshot;
}

function failedStatus(failureRevision: number): WindowManagerMigrationStatus {
  return status({
    state: "failed",
    summary: null,
    error: { code: "window_manager_invalid_snapshot", path: null, message: "bad tree" },
    failureRevision,
  });
}

/** The shell projection follows the cached snapshot, as the runtime publishes it. */
function renderNotice(migration: WindowManagerMigrationStatus | null) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  queryClient.setQueryData(SNAPSHOT_KEY, snapshotWith(migration));
  const view = () =>
    ({ snapshot: queryClient.getQueryData(SNAPSHOT_KEY) ?? null }) as OsDesktopRuntimeStore;
  const projection = createWindowManagerProjectionAtom(view());
  queryClient.getQueryCache().subscribe(() => projection.set(view()));
  const shell = { projection } as unknown as OsShellHandle;
  const wrapper = ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>
      <OsShellContext.Provider value={shell}>
        <DeskProfileContext value="default">{children}</DeskProfileContext>
      </OsShellContext.Provider>
    </QueryClientProvider>
  );
  /** A `snapshot` frame at the same revision, as the stream applies it. */
  const frame = (next: WindowManagerMigrationStatus | null) =>
    act(() => {
      queryClient.setQueryData(SNAPSHOT_KEY, snapshotWith(next));
    });
  return { ...render(<OsUpgradeNotice />, { wrapper }), frame };
}

beforeEach(() => {
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
  it("Should show the counts until dismissed, then hide at once and stay hidden after the frame", async () => {
    const user = userEvent.setup();
    let finishAck: (value: WindowManagerMigrationStatus) => void = () => {};
    api.updateWindowManagerMigration.mockReturnValue(
      new Promise(resolve => {
        finishAck = resolve;
      })
    );
    const { frame } = renderNotice(status());

    expect(screen.getByText("Your windows are now tabs")).toBeInTheDocument();
    expect(screen.getByText(/3 floating windows and 2 minimized windows/)).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Dismiss" }));
    expect(api.updateWindowManagerMigration).toHaveBeenCalledWith(
      "workspace:test",
      "default",
      "ack"
    );
    // Hidden while the ack is in flight, and still hidden once it settles before the frame.
    expect(screen.queryByText("Your windows are now tabs")).toBeNull();
    await act(async () => finishAck(status({ acknowledged: true })));
    expect(screen.queryByText("Your windows are now tabs")).toBeNull();
    frame(null);
    expect(screen.queryByText("Your windows are now tabs")).toBeNull();
  });

  it("Should hide when a same-revision snapshot arrives without a migration", () => {
    const { frame } = renderNotice(status());
    expect(screen.getByText("Your windows are now tabs")).toBeInTheDocument();

    frame(null);
    expect(screen.queryByText("Your windows are now tabs")).toBeNull();
    expect(screen.queryByRole("button", { name: "Dismiss" })).toBeNull();
  });

  it("Should show the failure when a snapshot reports a failed, unacknowledged migration", () => {
    const { frame } = renderNotice(null);
    expect(screen.queryByRole("button", { name: "Dismiss" })).toBeNull();

    frame(failedStatus(REVISION));
    expect(screen.getByText("We could not move your layout to tabs")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Retry" })).toBeEnabled();
  });

  it("Should offer Retry on a failed migration and show it running", async () => {
    const user = userEvent.setup();
    const failed = failedStatus(REVISION);
    let finishRetry: (value: WindowManagerMigrationStatus) => void = () => {};
    api.updateWindowManagerMigration.mockReturnValue(
      new Promise(resolve => {
        finishRetry = resolve;
      })
    );
    renderNotice(failed);

    expect(screen.getByText("We could not move your layout to tabs")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Retry" }));
    expect(api.updateWindowManagerMigration).toHaveBeenCalledWith(
      "workspace:test",
      "default",
      "retry",
      { replace: false }
    );
    expect(await screen.findByRole("button", { name: "Retrying" })).toBeDisabled();

    finishRetry(status());
    expect(await screen.findByText("Your windows are now tabs")).toBeInTheDocument();
  });

  it("Should offer a confirmed replace when the layout changed after the failure, sending replace once", async () => {
    const user = userEvent.setup();
    let finishReplace: (value: WindowManagerMigrationStatus) => void = () => {};
    api.updateWindowManagerMigration.mockReturnValue(
      new Promise(resolve => {
        finishReplace = resolve;
      })
    );
    renderNotice(failedStatus(REVISION - 3));

    expect(screen.getByText("We could not move your layout to tabs")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Retry" })).toBeNull();
    await user.click(screen.getByRole("button", { name: "Replace current layout" }));
    expect(screen.getByText("Replace current layout?")).toBeInTheDocument();
    expect(api.updateWindowManagerMigration).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "Replace layout" }));
    expect(api.updateWindowManagerMigration).toHaveBeenCalledTimes(1);
    expect(api.updateWindowManagerMigration).toHaveBeenCalledWith(
      "workspace:test",
      "default",
      "retry",
      { replace: true }
    );
    expect(await screen.findByRole("button", { name: "Retrying" })).toBeDisabled();
    expect(screen.queryByText("Replace current layout?")).toBeNull();

    finishReplace(status());
    expect(await screen.findByText("Your windows are now tabs")).toBeInTheDocument();
  });

  it("Should send nothing and keep the notice when the replace is cancelled", async () => {
    const user = userEvent.setup();
    renderNotice(failedStatus(REVISION - 3));

    await user.click(screen.getByRole("button", { name: "Replace current layout" }));
    await user.click(screen.getByRole("button", { name: "Cancel" }));

    expect(screen.queryByText("Replace current layout?")).toBeNull();
    expect(screen.getByRole("button", { name: "Replace current layout" })).toBeEnabled();
    expect(api.updateWindowManagerMigration).not.toHaveBeenCalled();
  });

  it("Should keep offering the replace when a replaced conversion fails again", async () => {
    const user = userEvent.setup();
    const failed = failedStatus(REVISION - 3);
    api.updateWindowManagerMigration.mockResolvedValue({
      ...failed,
      at: "2026-11-12T16:00:00Z",
      error: { code: "window_manager_invalid_snapshot", path: null, message: "still bad" },
    });
    renderNotice(failed);

    await user.click(screen.getByRole("button", { name: "Replace current layout" }));
    await user.click(screen.getByRole("button", { name: "Replace layout" }));

    expect(await screen.findByRole("button", { name: "Replace current layout" })).toBeEnabled();
    expect(screen.getByText("We could not move your layout to tabs")).toBeInTheDocument();
  });

  it("Should turn a refused plain retry into the confirmed replace", async () => {
    const user = userEvent.setup();
    api.updateWindowManagerMigration.mockRejectedValueOnce(
      new WindowManagerApiError("revision conflict", 409, null)
    );
    renderNotice(failedStatus(REVISION));

    await user.click(screen.getByRole("button", { name: "Retry" }));

    expect(await screen.findByRole("button", { name: "Replace current layout" })).toBeEnabled();
    expect(screen.queryByRole("button", { name: "Retry" })).toBeNull();
    expect(api.updateWindowManagerMigration).toHaveBeenCalledTimes(1);
  });
});
