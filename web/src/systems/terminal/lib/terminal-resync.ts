/**
 * Catching up after the daemon skipped output.
 *
 * A `GAP` frame, or an attach the daemon marks truncated, both mean the same
 * thing: the screen in front of this viewer is no longer a continuation of what
 * ran. Continuing to paint would be a lie about what happened, so the screen is
 * rebuilt from the server's own snapshot and the live tail is reconciled
 * against it by absolute sequence.
 *
 * Input stays shut for the whole pass. Re-opening it earlier would let a
 * keystroke answer a screen the emulator has not drawn yet.
 */

import type { TerminalGapBuffer } from "./terminal-gap-buffer";

export interface TerminalResyncPort {
  /** Fetches the current screen and the sequence it ends at. */
  readSnapshot: () => Promise<{ content: string; seq: number; busy: boolean }>;
  /** Clears the emulator. Runs inside the serialized emulator chain. */
  reset: () => void;
  write: (content: string) => Promise<void>;
  /** Runs one emulator operation after every operation queued before it. */
  enqueue: (run: () => Promise<void>) => Promise<void>;
  gapBuffer: TerminalGapBuffer;
  commit: (seqEnd: number) => void;
  setStatus: (status: "resyncing" | "connected") => void;
  setInputEnabled: (enabled: boolean) => void;
  /** Reports that the snapshot and held tail now form one continuous screen. */
  onRecovered: () => void;
  reportError: (cause: unknown, fallback: string) => void;
  /** Abandons the connection and asks again from the last drawn byte. */
  reconnectFromCommitted: () => void;
  /** True once this client is stopped for good. */
  isStopped: () => boolean;
  /** Identity of the connection this pass belongs to. */
  currentEpoch: () => number;
  /** Whether this viewer may type once the screen is whole again. */
  mayWrite: () => boolean;
}

export class TerminalResync {
  private running = false;
  /** A gap that arrived while an earlier one was still being reconciled. */
  private queued = false;
  private task: Promise<void> | null = null;

  constructor(private readonly port: TerminalResyncPort) {}

  get isRunning(): boolean {
    return this.running;
  }

  /**
   * Resolves once no catch-up is in flight.
   *
   * A pass owns the held tail for its whole lifetime, including the part spent
   * waiting on an HTTP snapshot. Opening the next connection before it lets go
   * would hand that connection's replay to a buffer this pass is about to
   * discard, and the screen would stay frozen.
   */
  idle(): Promise<void> {
    return this.task ?? Promise.resolve();
  }

  /** Runs a catch-up pass, or folds into the one already running. */
  async run(): Promise<void> {
    if (this.running) {
      // More was dropped while catching up: the snapshot in flight is already
      // stale, so another pass runs once this one finishes.
      this.queued = true;
      return this.task ?? undefined;
    }
    const task = this.execute();
    this.task = task.finally(() => {
      this.task = null;
    });
    return this.task;
  }

  private async execute(): Promise<void> {
    this.running = true;
    this.port.setInputEnabled(false);
    this.port.setStatus("resyncing");
    // The catch-up belongs to the connection that reported the gap. If that
    // connection dies while the snapshot is in flight, this pass has nothing to
    // say about the one that replaces it.
    const epoch = this.port.currentEpoch();
    try {
      if (!(await this.rebuildQueued(epoch))) return;
      this.port.setStatus("connected");
      this.port.setInputEnabled(this.port.mayWrite());
      this.port.onRecovered();
    } catch (cause) {
      if (this.port.isStopped() || epoch !== this.port.currentEpoch()) return;
      // This socket is abandoned below. Its held tail is replayed from the last
      // committed byte on the fresh connection, so none of it belongs here.
      this.port.gapBuffer.drop();
      this.port.reportError(cause, "The terminal could not catch up after skipped output.");
      // Carrying on over this socket would commit every later frame onto a
      // screen that still has a hole in it.
      this.port.reconnectFromCommitted();
    } finally {
      this.running = false;
    }
  }

  private async rebuildQueued(epoch: number): Promise<boolean> {
    this.queued = false;
    await this.rebuild(epoch);
    if (this.port.isStopped() || epoch !== this.port.currentEpoch()) {
      // The held tail belongs to a connection that no longer exists, and none
      // of it was drawn. The committed resume point still precedes that tail.
      this.port.gapBuffer.drop();
      return false;
    }
    return this.queued ? this.rebuildQueued(epoch) : true;
  }

  private async rebuild(epoch: number): Promise<void> {
    const snapshot = await this.port.readSnapshot();
    if (this.port.isStopped() || epoch !== this.port.currentEpoch()) return;
    if (snapshot.busy) {
      throw new Error("The terminal screen is still rebuilding.");
    }
    // Reset, snapshot and the held tail are one indivisible step on the screen:
    // nothing else may write between them, or a frame issued before the gap
    // would land on top of the screen that replaced it.
    await this.port.enqueue(async () => {
      // Checked before anything is drawn. Once the reset and the snapshot have
      // landed they are the screen, whatever happened to the socket meanwhile —
      // so they are committed, and the reconnection resumes after them instead
      // of replaying over a screen that already shows them.
      if (this.port.isStopped() || epoch !== this.port.currentEpoch()) return;
      this.port.reset();
      await this.port.write(snapshot.content);
      this.port.commit(snapshot.seq);
      // A tail that is not contiguous with this snapshot means more was dropped
      // behind it. Another pass runs before any of it reaches the screen.
      if (!(await this.port.gapBuffer.reconcile(snapshot.seq))) this.queued = true;
    });
  }
}
