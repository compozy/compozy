/**
 * What an exited terminal still says about itself.
 *
 * The live stream reports an exit once, as a frame. A terminal that ended
 * before you opened it never sends one — but the daemon recorded the outcome on
 * the terminal itself, and that record is the same truth. These read it.
 */

import type { TerminalExitNotice } from "../stores/terminal-store";
import type { TerminalInfo } from "../types";

/** The terminal's own recorded outcome, in the shape the exit bar reads. */
export function exitNoticeFromTerminal(terminal: TerminalInfo): TerminalExitNotice | null {
  if (terminal.state !== "exited" || !terminal.exit) return null;
  return {
    cause: terminal.exit.cause,
    code: terminal.exit.code ?? null,
    signal: terminal.exit.signal ?? null,
  };
}

const MINUTE_MS = 60_000;

/**
 * How much longer the screen stays readable.
 *
 * Phrased only when the retention window is actually known — `exit_retention`
 * is a config value, so a surface without it says nothing rather than promising
 * a window it cannot vouch for. A window that has already closed says nothing
 * either: the terminal is about to disappear, and counting down past zero would
 * be a claim the daemon never made.
 */
export function terminalRetentionNote(
  terminal: TerminalInfo,
  retentionMs: number | undefined,
  now: number = Date.now()
): string | undefined {
  if (retentionMs === undefined || retentionMs <= 0) return undefined;
  const endedAt = terminal.exit?.at ? Date.parse(terminal.exit.at) : Number.NaN;
  if (Number.isNaN(endedAt)) return undefined;
  const remaining = endedAt + retentionMs - now;
  if (remaining <= 0) return undefined;
  const minutes = Math.ceil(remaining / MINUTE_MS);
  if (minutes === 1) return "readable for 1 more minute";
  return `readable for ${minutes} more minutes`;
}
