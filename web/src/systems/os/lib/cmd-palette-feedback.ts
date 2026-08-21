import type { UserFeedbackTone } from "@/lib/user-feedback";

import type { CmdPaletteInvokeResult, ResolvedPaletteCommand } from "./cmd-palette-types";

/**
 * What the operator is told after an execution (`_uiux.md` S14, US-017).
 *
 * Every string here either names the command or repeats the runtime's own
 * reason verbatim — the palette never paraphrases a failure into something
 * friendlier than the truth (BR-8), and it never reports progress the runtime
 * did not report (SD-007).
 */

/** The daemon's single-flight rejection (`_dx.md` § Errors, Safety Invariant 1). */
export const ALREADY_RUNNING_CODE = "already_running";
/** The daemon reports this while an approval decides the command's fate. */
export const APPROVAL_PENDING_STATUS = "approval_pending";

export interface PaletteFeedback {
  readonly message: string;
  readonly tone: UserFeedbackTone;
  /** Drives the Retry affordance on the toast (US-017.AC-3). */
  readonly retryable: boolean;
}

/**
 * Retry is offered only where re-running is safe by declaration, and never for a
 * single-flight rejection: that command is already running, so a second attempt
 * would be refused for the same reason.
 */
export function canRetry(command: ResolvedPaletteCommand, code: string | undefined): boolean {
  if (code === ALREADY_RUNNING_CODE) return false;
  return command.execution.retry_safe;
}

/**
 * The completion report for a daemon-executed command. An approval-gated
 * invocation has not run yet, so it says so instead of claiming success.
 */
export function invokeCompletedFeedback(
  command: ResolvedPaletteCommand,
  result: CmdPaletteInvokeResult
): PaletteFeedback {
  if (result.status === APPROVAL_PENDING_STATUS) {
    return {
      message: `${command.title} needs approval before it runs`,
      tone: "info",
      retryable: false,
    };
  }
  return { message: `${command.title} finished`, tone: "success", retryable: false };
}

/** The failure report: the command by name, then the runtime's reason verbatim. */
export function invokeFailedFeedback(
  command: ResolvedPaletteCommand,
  reason: string,
  code?: string
): PaletteFeedback {
  return {
    message: `${command.title} — ${reason}`,
    tone: "error",
    retryable: canRetry(command, code),
  };
}

/**
 * A landing that changes the active workspace names the switch, so the shell
 * moving underneath the operator is never a silent context jump (US-017.EC-3).
 */
export function workspaceSwitchFeedback(workspaceName: string, target: string): PaletteFeedback {
  return {
    message: `Switched to ${workspaceName} to open ${target}`,
    tone: "info",
    retryable: false,
  };
}
