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
export class TerminalEmulatorQueue {
  private chain: Promise<void> = Promise.resolve();

  /** Runs one operation after every operation queued before it. */
  run(operation: () => Promise<void>): Promise<void> {
    const next = this.chain.then(operation, operation);
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
