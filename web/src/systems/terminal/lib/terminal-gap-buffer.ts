/**
 * The live tail, held while a skipped-output gap is being reconciled.
 *
 * The daemon does not pause when it reports a gap: it keeps sending, and those
 * frames arrive while the client is still fetching the screen snapshot that is
 * supposed to replace everything before them. Writing them straight through
 * means the snapshot either erases them or lands interleaved with them, so they
 * are held here by absolute sequence and reconciled once the snapshot is on
 * screen.
 *
 * Flow-control credit is the other half of the job. The daemon counted every
 * held byte against this viewer's window, so credit is returned for all of them
 * — written or discarded — or the window stalls and the terminal freezes on top
 * of a failed catch-up.
 */

/** One frame held back while the screen is being rebuilt. */
interface HeldFrame {
  seq: number;
  bytes: Uint8Array;
}

export interface TerminalGapBufferOptions {
  /** Writes bytes and resolves once the emulator has parsed them. */
  write: (bytes: Uint8Array) => Promise<void>;
  /** Returns flow-control credit for bytes the daemon already counted. */
  returnCredit: (bytes: number) => void;
  /**
   * Records that everything up to this sequence is now on screen.
   *
   * Only called once a frame is genuinely shown — either written here or
   * already covered by the snapshot — so a reconnect never resumes past bytes
   * nobody drew.
   */
  commit: (seqEnd: number) => void;
  /** True once the connection is gone: stop writing, keep the books straight. */
  isCancelled: () => boolean;
}

export class TerminalGapBuffer {
  private readonly frames: HeldFrame[] = [];

  constructor(private readonly options: TerminalGapBufferOptions) {}

  get size(): number {
    return this.frames.length;
  }

  hold(seq: number, bytes: Uint8Array): void {
    this.frames.push({ seq, bytes });
  }

  /**
   * Writes the part of the held tail the snapshot did not already show.
   *
   * A frame entirely below `snapshotSeq` is already on screen and is dropped; a
   * frame straddling it is written from its first uncovered byte.
   *
   * The tail must be *contiguous* with the snapshot to be written at all. If a
   * frame starts past where the screen currently ends, something was dropped
   * between the two — a second gap — and writing it would paste a later screen
   * onto an earlier one and advance the resume cursor past bytes nobody drew.
   * Reconciliation stops there and reports it, leaving the rest held for the
   * fresh snapshot that has to come first.
   */
  async reconcile(snapshotSeq: number): Promise<boolean> {
    let covered = snapshotSeq;
    while (this.frames.length > 0) {
      const held = this.frames[0];
      if (held.seq > covered) return false;
      this.frames.shift();
      const size = held.bytes.byteLength;
      const end = held.seq + size;
      if (end > covered) {
        // Checked before the write starts, never after it resolves. Bytes that
        // reached the screen are committed whatever became of the connection —
        // asking for them again would draw them twice.
        if (this.options.isCancelled()) return false;
        await this.options.write(held.bytes.subarray(covered - held.seq));
      }
      covered = Math.max(covered, end);
      this.options.commit(end);
      // Credit is a courtesy to a socket that may already be gone; it no-ops.
      this.options.returnCredit(size);
    }
    return true;
  }

  /** Gives up on the held tail but still pays back what the daemon counted. */
  discard(): void {
    let bytes = 0;
    while (this.frames.length > 0) {
      bytes += this.frames.shift()?.bytes.byteLength ?? 0;
    }
    if (bytes > 0) this.options.returnCredit(bytes);
  }

  /** Drops the held tail outright: the connection it belonged to is gone. */
  drop(): void {
    this.frames.length = 0;
  }
}
