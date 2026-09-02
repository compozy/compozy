import type { TerminalSelectionRange } from "@compozy/ui";

import type { TerminalInputRequest } from "../types";

/**
 * What the window asks its host to do.
 *
 * Only operations that are genuinely someone else's: creating and closing
 * terminals, stopping a program, answering a question, opening Settings. Taking
 * control and giving it back are deliberately absent — those are frames on the
 * terminal's own socket, and routing them through a parent callback would put a
 * second, slower authority in front of the daemon's lease.
 */
export interface TerminalWindowActions {
  onOpenTerminal?: () => void;
  /** Retargets the host route to a terminal the id-less window adopted. */
  retargetTerminal?: (terminalId: string) => void;
  /**
   * Opens another terminal beside this one — a new OS window joining this
   * frame as a tab. Without it the head's New falls back to `onOpenTerminal`,
   * which retargets the current window.
   */
  onOpenTerminalTab?: () => void;
  onCloseTerminal: (terminalId: string) => void;
  /** True while a close is already on its way to the daemon. */
  closePending?: boolean;
  onStop: (terminalId: string) => void;
  onWait: (terminalId: string) => void;
  onStopRecording?: (terminalId: string) => void;
  onAnswerInputRequest: (request: TerminalInputRequest, input: string) => void;
  onRejectInputRequest: (request: TerminalInputRequest) => void;
  /**
   * What a selection can become. Required, all of it.
   *
   * Every branch the menu can reach has to be callable before the menu exists,
   * or a host that wires half of it ships visible items that do nothing. The
   * window has no opinion about *which* conversation a quote goes to — that is
   * the host's — but it will not offer the gesture until the host can answer.
   */
  onSendSelection: (terminalId: string, selection: TerminalSelectionRange) => void;
  onCopySelection: (terminalId: string, selection: TerminalSelectionRange) => void;
  /** No conversation is open: the gesture offers a way in rather than failing. */
  onChooseSession: (terminalId: string, selection: TerminalSelectionRange) => void;
  onStartSession: (terminalId: string, selection: TerminalSelectionRange) => void;
  /** True while a conversation is open to send to. */
  hasActiveSession: boolean;
  /** Raising the cap lives in Settings; the window only points at it. */
  onOpenSettings?: () => void;
}
