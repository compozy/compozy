import { fetchWindowManagerSnapshot } from "../adapters/window-manager-api";
import type { WindowManagerSnapshot } from "../lib/window-manager-types";

type SnapshotRefreshResult =
  | { status: "applied"; snapshot: WindowManagerSnapshot }
  | { status: "failed"; error: Error }
  | { status: "stale" };

/** Owns cancellation and stale-response rejection for snapshot refreshes. */
export class WindowManagerSnapshotRefresher {
  private controller: AbortController | null = null;

  reset(): void {
    this.controller?.abort();
    this.controller = null;
  }

  async refresh(
    workspaceId: string,
    profileId: string,
    bindingIsCurrent: () => boolean
  ): Promise<SnapshotRefreshResult> {
    this.controller?.abort();
    const controller = new AbortController();
    this.controller = controller;

    try {
      const snapshot = await fetchWindowManagerSnapshot(workspaceId, profileId, controller.signal);
      if (this.isStale(controller, bindingIsCurrent)) return { status: "stale" };
      return { status: "applied", snapshot };
    } catch (error) {
      if (this.isStale(controller, bindingIsCurrent)) return { status: "stale" };
      return {
        status: "failed",
        error: error instanceof Error ? error : new Error("Unable to reload the window layout."),
      };
    } finally {
      if (this.controller === controller) this.controller = null;
    }
  }

  private isStale(controller: AbortController, bindingIsCurrent: () => boolean): boolean {
    return controller.signal.aborted || this.controller !== controller || !bindingIsCurrent();
  }
}
