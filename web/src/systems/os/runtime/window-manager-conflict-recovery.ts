import { windowManagerStore } from "../stores/window-manager-store";
import { WINDOW_MANAGER_DIAGNOSTIC_TTL_MS } from "./window-manager-runtime-helpers";

/** What conflict recovery reads from and reports back to its runtime. */
export interface WindowManagerConflictRecoveryHost {
  /** The revision of the snapshot the runtime presents, `0` without one. */
  presentedRevision(): number;
  refreshSnapshot(options: { authoritative?: boolean }): Promise<boolean>;
  publish(): void;
}

/**
 * A revision conflict means another client or agent moved the layout first.
 * The surface re-reads the snapshot and reopens for commands on its own; the
 * refused action rolls back and the notice explains what happened.
 */
export class WindowManagerConflictRecovery {
  private readonly host: WindowManagerConflictRecoveryHost;
  private recovery: Promise<boolean> | null = null;
  private diagnosticTimer: ReturnType<typeof setTimeout> | null = null;

  constructor(host: WindowManagerConflictRecoveryHost) {
    this.host = host;
  }

  recover(): Promise<boolean> {
    if (this.recovery !== null) return this.recovery;
    const commandState = windowManagerStore.getSnapshot().context.commandState;
    // The daemon reporting a revision behind ours means it restarted from a
    // discarded or replaced arrangement; its re-read replaces the cache.
    const authoritative =
      commandState.status === "conflict" &&
      commandState.conflict.currentRevision < this.host.presentedRevision();
    this.recovery = this.host
      .refreshSnapshot({ authoritative })
      .then(refreshed => {
        if (!refreshed) return false;
        const commandState = windowManagerStore.getSnapshot().context.commandState;
        if (commandState.status === "conflict") {
          const diagnostic = commandState.diagnostic;
          windowManagerStore.trigger.conflictCleared();
          windowManagerStore.trigger.diagnosticReported({ diagnostic });
          this.scheduleDiagnosticExpiry();
        }
        return refreshed;
      })
      .finally(() => {
        this.recovery = null;
        this.host.publish();
      });
    return this.recovery;
  }

  /** Resolves once the surface has re-read the layout after a revision conflict. */
  awaitRecovery(): Promise<boolean> {
    if (windowManagerStore.getSnapshot().context.commandState.status !== "conflict") {
      return Promise.resolve(true);
    }
    return this.recover();
  }

  scheduleDiagnosticExpiry(): void {
    this.cancelDiagnosticExpiry();
    this.diagnosticTimer = setTimeout(() => {
      this.diagnosticTimer = null;
      windowManagerStore.trigger.diagnosticCleared();
      this.host.publish();
    }, WINDOW_MANAGER_DIAGNOSTIC_TTL_MS);
  }

  cancelDiagnosticExpiry(): void {
    if (this.diagnosticTimer === null) return;
    clearTimeout(this.diagnosticTimer);
    this.diagnosticTimer = null;
  }
}
