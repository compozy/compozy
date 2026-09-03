/**
 * The write never reached a screen.
 *
 * `TerminalViewHandle.write` resolves only when the emulator has parsed the
 * bytes, and callers read that as "drawn" — the terminal client returns
 * flow-control credit and advances its resume point on it. A buffer that goes
 * away before parsing therefore has to reject rather than resolve: resolving
 * would claim bytes were shown that never were, and the stream would resume
 * past them.
 *
 * Callers that do not await the parse — a replay, a fixed preview — catch this
 * one and stay silent. It means the view closed, not that anything failed.
 */
export class TerminalWriteAbandonedError extends Error {
  constructor() {
    super("The terminal view was replaced before these bytes were drawn.");
    this.name = "TerminalWriteAbandonedError";
  }
}
