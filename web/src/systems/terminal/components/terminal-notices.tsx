import {
  ScissorsLineDashed,
  ShieldAlert,
  TicketX,
  Unplug,
  Users,
  type LucideIcon,
} from "lucide-react";

import { Alert, AlertDescription, AlertTitle, Button, MonoId } from "@compozy/ui";

import { terminalErrorCopy, terminalGapCopy } from "../lib/terminal-copy";
import type { TerminalGapNotice } from "../stores/terminal-store";

const NOTICE_ICONS: Record<string, LucideIcon> = {
  ticket_expired: TicketX,
  ticket_invalid: TicketX,
  subscriber_limit_reached: Users,
  slow_consumer: Unplug,
  journal_unavailable: ShieldAlert,
};

export interface TerminalStreamNoticeProps {
  code: string;
  message: string;
  /** Present only where retrying is genuinely the remedy. */
  onReconnect?: () => void;
  /**
   * Opens the journal.
   *
   * Offered for `terminal_not_found` alone: the terminal is gone, so there is
   * nothing to retry, but everything it ran is still recorded. No other code
   * gets an invented way out.
   */
  onViewJournal?: () => void;
}

/**
 * A stream refusal, stated as a sentence with its machine code beneath.
 *
 * The code stays visible because it is what a person carries into a CLI or an
 * issue; it never becomes the headline.
 */
export function TerminalStreamNotice({
  code,
  message,
  onReconnect,
  onViewJournal,
}: TerminalStreamNoticeProps) {
  const copy = terminalErrorCopy(code, message);
  const Icon = NOTICE_ICONS[code];
  // A terminal that no longer exists cannot be reconnected to; its journal is
  // the only thing left to open.
  const gone = code === "terminal_not_found";
  // These failures can be repaired by minting a fresh attachment. A full
  // subscriber list cannot: retrying before someone leaves only repeats the
  // same refusal, even when the caller has a reconnect callback available.
  const retryable =
    code === "ticket_expired" || code === "ticket_invalid" || code === "slow_consumer";
  return (
    <Alert data-testid={`terminal-notice-${code}`} variant="warning">
      {Icon ? <Icon aria-hidden="true" className="size-4" /> : null}
      <AlertTitle>{copy.title}</AlertTitle>
      {copy.detail ? <AlertDescription>{copy.detail}</AlertDescription> : null}
      <MonoId size="sm" value={code} />
      {gone && onViewJournal ? (
        <div className="mt-2">
          <Button
            data-testid="terminal-notice-view-journal"
            onClick={onViewJournal}
            size="xs"
            type="button"
            variant="outline"
          >
            View journal
          </Button>
        </div>
      ) : retryable && onReconnect ? (
        <div className="mt-2">
          <Button onClick={onReconnect} size="xs" type="button" variant="outline">
            Reconnect
          </Button>
        </div>
      ) : null}
    </Alert>
  );
}

/**
 * The seam where output was skipped.
 *
 * A viewer that fell behind is shown the break and how much it covered, then a
 * clean current picture — never a splice that reads as continuous output.
 */
export function TerminalGapSeam({ gap }: { gap: TerminalGapNotice }) {
  return (
    <div
      className="my-1.5 flex items-center gap-2 font-mono text-micro whitespace-nowrap text-warning"
      data-testid="terminal-gap-seam"
      role="status"
    >
      <span aria-hidden="true" className="h-px flex-1 bg-line-strong" />
      <ScissorsLineDashed aria-hidden="true" className="size-3" />
      {terminalGapCopy(gap.droppedBytes)}
      <span aria-hidden="true" className="h-px flex-1 bg-line-strong" />
    </div>
  );
}

/**
 * The audit-blocked bar.
 *
 * The record is load-bearing: when the journal cannot write, new commands and
 * typing pause while output and watching continue untouched. Warning, not
 * danger — nothing was lost.
 */
export function TerminalAuditBlockedBar() {
  const copy = terminalErrorCopy("journal_unavailable", "");
  return (
    <Alert
      className="rounded-none"
      data-testid="terminal-audit-blocked"
      role="status"
      variant="warning"
    >
      <ShieldAlert aria-hidden="true" className="size-4" />
      <AlertTitle>{copy.title}</AlertTitle>
      <AlertDescription>{copy.detail}</AlertDescription>
      <MonoId size="sm" value="journal_unavailable" />
    </Alert>
  );
}
