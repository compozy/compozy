import { History, WifiOff } from "lucide-react";

import { Button, Marker, MarkerMeta } from "@compozy/ui";

import { useSessionTransportState } from "../hooks/use-session-transcript-thread-messages";
import { formatMessageTimestamp } from "../lib/format-timestamp";
import { describeSessionHistoryReset } from "../lib/session-transport";

/**
 * Loaded, then lost (US-018.AC-2): retries ran out while the conversation was
 * on screen. What is shown is saved up to the last applied frame; the danger
 * glyph is earned — this is a real failure of the live view — and Try again
 * is the same recovery the sync-failed pane offers.
 */
export function SessionTransportFailureNotice() {
  const transport = useSessionTransportState();
  if (transport.phase !== "failed" || transport.failure === null) return null;
  const lostAt = transport.lastLiveAt === null ? "" : formatMessageTimestamp(transport.lastLiveAt);
  return (
    <Marker
      role="alert"
      data-testid="session-transport-failure"
      tone="danger"
      icon={<WifiOff strokeWidth={1.8} />}
    >
      <b>{lostAt ? `Live updates stopped at ${lostAt}` : "Live updates stopped"}</b> — couldn&apos;t
      reconnect after {transport.failure.attempts} tries. What you see is saved up to then.{" "}
      <Button
        type="button"
        variant="link"
        size="xs"
        className="h-auto px-0 text-muted underline underline-offset-2 hover:text-fg-strong"
        onClick={transport.retry}
        data-testid="session-transport-failure-retry"
      >
        Try again
      </Button>
    </Marker>
  );
}

/**
 * The daemon reset the view (US-017.AC-3 / EC-1): another generation while
 * the stream was down, a sequence reset, or retention retiring the cursor the
 * view held. Stated in one info line with the daemon's reason and generation
 * as meta, never a modal.
 */
export function SessionTransportHistoryResetNotice() {
  const transport = useSessionTransportState();
  const reset = transport.historyReset;
  if (reset === null) return null;
  const sentence = describeSessionHistoryReset(reset);
  return (
    <Marker
      role="status"
      data-testid="session-transport-history-reset"
      data-reset-reason={reset.reason ?? undefined}
      tone="info"
      icon={<History strokeWidth={1.8} />}
    >
      <b>{sentence.lead}</b>
      {sentence.rest}{" "}
      <MarkerMeta data-testid="session-transport-history-reset-generation">
        generation {reset.generation}
        {reset.reason ? ` · ${reset.reason}` : ""}
      </MarkerMeta>
    </Marker>
  );
}
