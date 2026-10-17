/**
 * One screen, one queue.
 *
 * Writes, resets and snapshot replays all mutate the same emulator, and a write
 * only resolves when the emulator has finished parsing it. Left unordered, a
 * write issued before a gap can land *after* the reset that was meant to
 * replace it, putting pre-gap bytes back on a screen that had already moved on.
 *
 * Everything that touches the screen goes through here, so the order on screen
 * is the order the operations were issued in.
 */
export const TERMINAL_RENDER_BUFFER_BYTES = 1024 * 1024;
export const TERMINAL_RENDER_BUFFER_FRAMES = 1024;

type PendingOperation = {
  operation: (() => Promise<void>) | null;
  bytes: number;
};

export class TerminalEmulatorQueue {
  private chain: Promise<void> = Promise.resolve();
  private readonly pending = new Set<PendingOperation>();
  private pendingBytes = 0;

  constructor(private readonly onOverflow: (through: bigint) => void) {}

  /** Serializes mutations; overflowing byte writes discard queued work and request a snapshot. */
  run(operation: () => Promise<void>, bytes = 0, through = 0n): Promise<void> {
    if (
      bytes > 0 &&
      (this.pendingBytes + bytes > TERMINAL_RENDER_BUFFER_BYTES ||
        this.pending.size >= TERMINAL_RENDER_BUFFER_FRAMES)
    ) {
      // Keep the write already being parsed, but release queued byte buffers.
      // Snapshot recovery is serialized after that write and replaces the loss.
      for (const pending of this.pending) {
        pending.operation = null;
        this.pendingBytes -= pending.bytes;
      }
      this.pending.clear();
      this.onOverflow(through);
      return Promise.resolve();
    }
    const pending: PendingOperation = { operation, bytes };
    this.pending.add(pending);
    this.pendingBytes += bytes;
    const invoke = async () => {
      this.pending.delete(pending);
      const work = pending.operation;
      pending.operation = null;
      if (!work) return;
      try {
        await work();
      } finally {
        this.pendingBytes -= bytes;
      }
    };
    const next = this.chain.then(invoke, invoke);
    // The queue survives a failed operation; the caller still sees the error.
    this.chain = next.then(
      () => undefined,
      () => undefined
    );
    return next;
  }

  /** Resolves once everything queued so far has finished. */
  async drain(): Promise<void> {
    await this.chain;
  }
}
