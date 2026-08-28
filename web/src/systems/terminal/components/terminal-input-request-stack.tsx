import { terminalInputStackNeedsOrigin } from "../lib/terminal-input-identity";
import type { TerminalInputRequest, TerminalResolvedInputRequest } from "../types";

import { TerminalInputRequestCard, TerminalInputResolvedRow } from "./terminal-input-request";

export interface TerminalInputRequestStackProps {
  pending: readonly TerminalInputRequest[];
  resolved?: readonly TerminalResolvedInputRequest[];
  /** Terminal id → title, used only when origin must name the source. */
  titles?: ReadonlyMap<string, string>;
  canAnswerDirectly: boolean;
  onAnswer: (request: TerminalInputRequest, input: string) => void;
  onReject: (request: TerminalInputRequest) => void;
  now?: number;
}

/**
 * Pending pins plus resolved outcomes, in arrival order.
 *
 * Origin is named only when more than one terminal is asking — a single
 * terminal already owns the window title.
 */
export function TerminalInputRequestStack({
  pending,
  resolved = [],
  titles,
  canAnswerDirectly,
  onAnswer,
  onReject,
  now,
}: TerminalInputRequestStackProps) {
  const showOrigin = terminalInputStackNeedsOrigin(pending);
  return (
    <div className="flex flex-col" data-testid="terminal-input-request-stack">
      {pending.map(request => (
        <TerminalInputRequestCard
          canAnswerDirectly={canAnswerDirectly}
          key={request.id}
          now={now}
          onAnswer={input => onAnswer(request, input)}
          onReject={() => onReject(request)}
          request={request}
          showOrigin={showOrigin}
          terminalTitle={titles?.get(request.terminal_id)}
        />
      ))}
      {resolved.map(request => (
        <TerminalInputResolvedRow key={request.id} request={request} />
      ))}
    </div>
  );
}
