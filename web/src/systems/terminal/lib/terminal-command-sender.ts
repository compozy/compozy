/**
 * What this client puts on the wire.
 *
 * Every one of these runs inside someone else's event — a keystroke, a resize
 * observer, a click — and the socket can be closing underneath any of them. A
 * throw there takes the whole handler with it, so a failed send is reported as
 * a stream error and answered `false` rather than raised.
 */

import {
  clampTerminalDimensions,
  encodeTerminalDetach,
  encodeTerminalInputFrames,
  encodeTerminalResize,
  encodeTerminalRelease,
  encodeTerminalSignal,
  encodeTerminalTakeover,
  type TerminalFrameBytes,
} from "./terminal-wire";
import type { TerminalSocket } from "../adapters/terminal-socket";
import type { TerminalSignal } from "../types";

export interface TerminalCommandSenderPort {
  /** The connection right now, or null while there is none. */
  socket: () => TerminalSocket | null;
  reportError: (cause: unknown, fallback: string) => void;
}

export class TerminalCommandSender {
  /** The size this viewer last voted for, carried into the next upgrade. */
  private proposedSize: { cols: number; rows: number } | null = null;
  /** The vote the daemon has actually received — sent, or carried in a query. */
  private sentSize: { cols: number; rows: number } | null = null;
  private detached = false;

  constructor(private readonly port: TerminalCommandSenderPort) {}

  get proposed(): { cols: number; rows: number } | null {
    return this.proposedSize;
  }

  /** True once a polite detach has genuinely reached the daemon. */
  get detachSent(): boolean {
    return this.detached;
  }

  /** Puts one frame on the wire, answering false when it could not. */
  send(frame: TerminalFrameBytes, message: string): boolean {
    const socket = this.port.socket();
    if (!socket) return false;
    try {
      socket.send(frame);
      return true;
    } catch (cause) {
      this.port.reportError(cause, message);
      return false;
    }
  }

  /**
   * Sends typed or pasted input.
   *
   * A paste arrives as one event of any length, so it is framed rather than
   * refused — in order, and without splitting a character across frames.
   */
  input(data: string): void {
    for (const frame of encodeTerminalInputFrames(data)) {
      if (!this.send(frame, "The terminal could not send what you typed.")) return;
    }
  }

  /**
   * Remembers this viewer's preferred size without touching the wire, so a
   * later upgrade query can carry it. The daemon decides; nothing here resizes
   * the emulator.
   */
  recordProposal(cols: number, rows: number): void {
    const clamped = clampTerminalDimensions(cols, rows);
    if (clamped) this.proposedSize = clamped;
  }

  /**
   * Delivers the recorded vote when the daemon has not received it yet.
   *
   * A vote made while the socket was still opening must not be lost: recording
   * it first and flushing on open is what keeps the first fit from being stuck
   * at the daemon's default until the next remount.
   */
  flushProposal(): void {
    const size = this.proposedSize;
    if (!size) return;
    if (this.sentSize?.cols === size.cols && this.sentSize.rows === size.rows) return;
    const sent = this.send(
      encodeTerminalResize(size.cols, size.rows),
      "The terminal could not report its size."
    );
    if (sent) this.sentSize = size;
  }

  /** The upgrade query carried this vote; the daemon already has it. */
  markProposalCarried(size: { cols: number; rows: number } | null): void {
    this.sentSize = size;
  }

  signal(signal: TerminalSignal): void {
    this.send(encodeTerminalSignal(signal), "The terminal could not send that signal.");
  }

  /** Claims the write lease. `force` skips the human-vs-human confirmation. */
  takeover(force: boolean): boolean {
    return this.send(encodeTerminalTakeover(force), "The terminal could not ask for control.");
  }

  /** Explicitly returns control; unlike DETACH, this is a lease transition. */
  release(message: string): boolean {
    return this.send(encodeTerminalRelease(), message);
  }

  /** Politely closes this attachment without changing the lease directly. */
  detach(message: string): void {
    if (this.detached) return;
    if (!this.send(encodeTerminalDetach(), message)) return;
    this.detached = true;
  }

  /** Teardown's own detach, sent only if releasing control did not already. */
  detachOnClose(socket: TerminalSocket): void {
    if (this.detached) return;
    try {
      socket.send(encodeTerminalDetach());
    } catch {
      // The socket was already gone; closing is all that is left to do.
    }
  }
}
