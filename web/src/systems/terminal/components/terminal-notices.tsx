import {
  ScissorsLineDashed,
  ShieldAlert,
  TicketX,
  Unplug,
  Users,
  type LucideIcon,
} from "lucide-react";

import {
  Alert,
  AlertActions,
  AlertDescription,
  AlertMeta,
  AlertTitle,
  Button,
  MonoId,
} from "@compozy/ui";

import { terminalErrorCopy, terminalGapCopy } from "../lib/terminal-copy";
import type { TerminalGapNotice } from "../stores/terminal-store";

const NOTICE_ICONS: Record<string, LucideIcon> = {
  ticket_expired: TicketX,
  ticket_invalid: TicketX,
  subscriber_limit_reached: Users,
  slow_consumer: Unplug,
  journal_unavailable: ShieldAlert,
};

/** The board's notices carry no headline tier — a bold lead-in over body text. */
const NOTICE_TITLE_CLASS = "text-form-input font-semibold";

export interface TerminalStreamNoticeProps {
  code: string;
  message: string;
  /** Present only where retrying is genuinely the remedy. */
  onReconnect?: () => void;
  /**
   * Opens the journal.
   *
   * Offered when the terminal itself is gone (`terminal_not_found`,
   * `terminal_expired`): there is nothing to retry, but everything it ran is
   * still recorded. No other code gets an invented way out.
   */
  onViewJournal?: () => void;
}

/**
 * A stream refusal, stated as a sentence with its machine code beneath.
 *
 * The code stays visible because it is what a person carries into a CLI or an
 * issue; it never becomes the headline. Pinned inside the surface, the notice
 * reads as a bar rather than a floating card.
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
  const gone = code === "terminal_not_found" || code === "terminal_expired";
  // These failures can be repaired by minting a fresh attachment. A full
  // subscriber list cannot: retrying before someone leaves only repeats the
  // same refusal, even when the caller has a reconnect callback available.
  const retryable =
    code === "ticket_expired" || code === "ticket_invalid" || code === "slow_consumer";
  const detail =
    code === "subscriber_limit_reached" && message.trim() !== "" ? message : copy.detail;
  // Warning is reserved for the audit-blocked pause. Other refusals are
  // information: the signal grammar must not paint every stream error amber.
  const variant = code === "journal_unavailable" ? "warning" : "default";
  return (
    // A refusal is urgent — the primitive's assertive `role="alert"` stands.
    <Alert className="rounded-none" data-testid={`terminal-notice-${code}`} variant={variant}>
      {Icon ? <Icon aria-hidden="true" /> : null}
      <AlertTitle className={NOTICE_TITLE_CLASS}>{copy.title}</AlertTitle>
      {detail ? <AlertDescription>{detail}</AlertDescription> : null}
      <AlertMeta>
        <MonoId size="sm" value={code} />
      </AlertMeta>
      {gone && onViewJournal ? (
        <AlertActions>
          <Button
            data-testid="terminal-notice-view-journal"
            onClick={onViewJournal}
            size="xs"
            type="button"
            variant="outline"
          >
            View journal
          </Button>
        </AlertActions>
      ) : retryable && onReconnect ? (
        <AlertActions>
          <Button onClick={onReconnect} size="xs" type="button" variant="outline">
            Reconnect
          </Button>
        </AlertActions>
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
