/**
 * Flow-control credit, returned in grains.
 *
 * The daemon lets a writer fall only so far behind before it stops sending.
 * Credit is what says "this much is drawn" — so it is counted from the parse,
 * never from receipt, and returned in 16 KiB grains rather than per frame
 * (ADR-009). Acking every frame would spend a message on every keystroke's
 * echo; acking on receipt would let a viewer that cannot keep up pull more work
 * than it can render, which is the whole thing the window exists to prevent.
 */

import { encodeTerminalAck, type TerminalFrameBytes } from "./terminal-wire";

const ACK_GRAIN_BYTES = 16 * 1024;

export class TerminalCreditWindow {
  private pending = 0;

  /**
   * @param send Puts a frame on the wire, answering false when it could not.
   */
  constructor(private readonly send: (frame: TerminalFrameBytes) => boolean) {}

  /** Records parsed bytes and returns a grain once one has accumulated. */
  record(bytes: number): void {
    this.pending += bytes;
    if (this.pending < ACK_GRAIN_BYTES) return;
    const credit = this.pending;
    this.pending = 0;
    this.send(encodeTerminalAck(credit));
  }

  /** Starts a fresh window: a new attachment counts from zero. */
  reset(): void {
    this.pending = 0;
  }
}
