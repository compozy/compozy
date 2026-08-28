import { toast } from "@compozy/ui";

import { createTerminalQuoteHostActions, useSessionCreateActions } from "@/systems/session";
import {
  copySourcedTerminalQuote,
  holdChooseSessionTerminalQuote,
  type TerminalWindowActions,
} from "@/systems/terminal";

import { raisePaletteView } from "../../../lib/raise-palette-view";
import type { RoutingCoordinator } from "../../../lib/routing-coordinator";

export interface TerminalWindowHostActionInput {
  coordinator: RoutingCoordinator;
  getActiveSessionId: () => string | null;
  activateSession: (sessionId: string) => void;
  hasActiveSession: boolean;
  openTerminal: (() => void) | undefined;
  close: (terminalId: string) => void;
  stop: (terminalId: string) => void;
  wait: (terminalId: string) => void;
  stopRecording: (terminalId: string) => void;
  answer: TerminalWindowActions["onAnswerInputRequest"];
  reject: TerminalWindowActions["onRejectInputRequest"];
}

/**
 * Host seams the terminal window must call: quote, wait, settings, create.
 *
 * Quote actions go through `createTerminalQuoteHostActions`. Choose holds the
 * quote in the choose-only slot and raises the Sessions palette view.
 */
export function useTerminalWindowHostActions(
  input: TerminalWindowHostActionInput
): TerminalWindowActions {
  const sessionCreate = useSessionCreateActions();
  const quote = createTerminalQuoteHostActions({
    getActiveSessionId: input.getActiveSessionId,
    openSessionPicker: held => {
      holdChooseSessionTerminalQuote(held);
      raisePaletteView("sessions");
    },
    startSessionWithQuote: held => {
      sessionCreate.openWithTerminalQuote(held);
    },
    activateSession: input.activateSession,
  });
  return {
    onOpenTerminal: input.openTerminal,
    onCloseTerminal: input.close,
    onStop: input.stop,
    onWait: input.wait,
    onStopRecording: input.stopRecording,
    onAnswerInputRequest: input.answer,
    onRejectInputRequest: input.reject,
    onSendSelection: quote.onSendSelection,
    onCopySelection: (terminalId, selection) => {
      void copySourcedTerminalQuote(terminalId, selection).catch(() => toast.error("Copy failed"));
    },
    onChooseSession: quote.onChooseSession,
    onStartSession: quote.onStartSession,
    hasActiveSession: input.hasActiveSession,
    onOpenSettings: () => {
      void input.coordinator.userOpen({
        app: "settings",
        route: { pathname: "/settings/terminal", search: {} },
      });
    },
  };
}
