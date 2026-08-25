/**
 * Surface words for the terminal.
 *
 * The plain-language register is part of the design contract: machine truth
 * always survives as demoted mono beside the human words, never as the primary
 * label. `project` is the surface alias for a workspace, per the glossary.
 */

import type {
  TerminalApproval,
  TerminalDetectedBy,
  TerminalExit,
  TerminalInputOutcome,
} from "../types";

/** Human sentence + the machine value that proves it. */
export interface TerminalOutcomeCopy {
  label: string;
  /** Rendered as demoted mono next to the label. */
  code: string;
  tone: "success" | "neutral";
  /** Present only when the machine value alone would leave a reader guessing. */
  note?: string;
}

/**
 * How a run ended. Only zero earns colour — a failing test run is information,
 * not an emergency — and a cause the daemon could not verify stays unknown.
 */
export function terminalExitCopy(exit: {
  cause: TerminalExit["cause"];
  code?: number | null;
  signal?: string | null;
}): TerminalOutcomeCopy {
  if (exit.cause === "signaled") {
    return { label: "Stopped", code: `signal ${exit.signal ?? "unknown"}`, tone: "neutral" };
  }
  if (exit.cause === "unknown" || exit.code === null || exit.code === undefined) {
    return {
      label: "Ended",
      code: "cause unknown",
      tone: "neutral",
      note: "CompozyOS couldn't see how this ended",
    };
  }
  if (exit.code === 0) {
    return { label: "Succeeded", code: "exit 0", tone: "success" };
  }
  return { label: "Finished with errors", code: `exit ${exit.code}`, tone: "neutral" };
}

/** Which permission covered a run. */
export function terminalApprovalCopy(approval: TerminalApproval): string {
  switch (approval) {
    case "allowlisted":
      return "always allowed";
    case "approved_always":
      return "always allowed";
    case "approved_once":
      return "approved once";
    case "human":
      return "approved by you";
    case "none":
      return "not needed";
  }
}

/** How sure the record is. Only the guess is marked. */
export function terminalConfidenceCopy(detectedBy: TerminalDetectedBy): {
  label: string;
  estimated: boolean;
} {
  switch (detectedBy) {
    case "exact":
      return { label: "exact", estimated: false };
    case "marker":
      return { label: "verified", estimated: false };
    case "idle":
      return { label: "estimated", estimated: true };
  }
}

/** What became of a question the agent asked. */
export function terminalInputOutcomeCopy(outcome: TerminalInputOutcome): string {
  switch (outcome) {
    case "answered":
      return "Answered";
    case "rejected":
      return "Declined";
    case "superseded":
      return "Superseded";
    case "expired":
      return "Expired";
  }
}

/**
 * Error copy, keyed by the daemon's machine code.
 *
 * The sentence is the primary carrier and the code rides beneath it as mono
 * truth. A code with no entry here renders its own message — never a fabricated
 * explanation.
 */
export const TERMINAL_ERROR_COPY: Record<string, { title: string; detail: string }> = {
  ticket_expired: {
    title: "Connection took too long.",
    detail: "The connection pass expired before it could be used — reconnecting gets a fresh one.",
  },
  ticket_invalid: {
    title: "This connection pass was already used.",
    detail: "Each attach needs its own — reconnecting gets a fresh one.",
  },
  subscriber_limit_reached: {
    title: "This terminal is full.",
    detail: "Someone has to leave before you can watch.",
  },
  slow_consumer: {
    title: "Your view fell too far behind and was disconnected.",
    detail: "The terminal kept running — reconnect to see the current screen.",
  },
  journal_unavailable: {
    title: "New commands are paused.",
    detail:
      "The journal can't record right now, so input waits until it can. Watching and output continue.",
  },
  terminal_expired: {
    title: "This terminal was cleaned up",
    // No duration here: how long a terminal may sit unwatched is
    // `[terminal].detached_ttl`, and this copy has no way to read it. The state
    // that does — `TerminalExpiredState` — names it when its caller supplies it.
    detail:
      "Nobody was watching for a while, so it was closed. Its command history is still in the journal.",
  },
  terminal_not_found: {
    title: "This terminal didn't survive the restart",
    detail:
      "CompozyOS restarted, and live terminals don't carry across. Everything that ran is in the journal.",
  },
  terminal_interactive_unavailable: {
    title: "Interactive terminals aren't available here yet",
    detail:
      "On this platform, agents can still run commands and everything lands in the journal — there's just no live screen to type into.",
  },
  terminal_limit_reached: {
    title: "This project is at its terminal limit",
    detail: "Close one to open another, or raise the limit in Settings.",
  },
};

export function terminalErrorCopy(
  code: string,
  message: string
): {
  title: string;
  detail: string;
} {
  return TERMINAL_ERROR_COPY[code] ?? { title: message, detail: "" };
}

/** "content was skipped · 48 KB" — the seam, never a silent splice. */
export function terminalGapCopy(droppedBytes: number): string {
  return `content was skipped · ${formatBytes(droppedBytes)}`;
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}
