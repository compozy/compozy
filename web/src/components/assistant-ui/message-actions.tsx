import { useAuiState } from "@assistant-ui/react";

import { cn } from "@/lib/utils";
import {
  formatMessageTimestamp,
  formatMessageTimestampFull,
} from "@/systems/session/lib/format-timestamp";
import { SessionRewindMessageAction } from "@/systems/session";
import { CopyIconButton } from "@compozy/ui";
import { deriveMessageActions } from "./message-actions.logic";
import { useSessionThreadReadOnly } from "./hooks/use-session-thread-read-only";

const ACTIONS_CLASS_NAME = "flex items-center gap-2 text-small-body text-muted tabular-nums";

export interface MessageActionsProps {
  /** `start` aligns the row under a flat assistant message; `end` under the right-aligned user bubble. */
  align: "start" | "end";
  copyLabel: string;
  testId: string;
}

export function MessageActions({ align, copyLabel, testId }: MessageActionsProps) {
  const message = useAuiState(
    state => state.message as { content?: unknown; status?: { type?: string } }
  );
  const { source, timestampMs, streaming, visible } = deriveMessageActions(message);
  const readOnly = useSessionThreadReadOnly();

  if (!visible) {
    return null;
  }

  const timestamp =
    timestampMs !== null ? (
      <time
        data-testid={`${testId}-timestamp`}
        dateTime={new Date(timestampMs).toISOString()}
        title={formatMessageTimestampFull(timestampMs)}
        className="text-subtle tabular-nums"
      >
        {formatMessageTimestamp(timestampMs)}
      </time>
    ) : null;

  const copy = (
    <CopyIconButton
      value={source}
      copyLabel={copyLabel}
      copiedLabel="Copied"
      copiedToastLabel="Message copied"
      copyFailedToastLabel="Couldn't copy message"
      disabled={streaming && source.length === 0}
      className="text-muted hover:text-fg"
      data-testid={`${testId}-copy`}
    />
  );
  return (
    <div
      data-testid={testId}
      className={cn(ACTIONS_CLASS_NAME, align === "end" ? "justify-end" : "justify-start")}
    >
      {align === "end" ? (
        <>
          {timestamp}
          {copy}
          {readOnly ? null : <SessionRewindMessageAction />}
        </>
      ) : (
        <>
          {copy}
          {timestamp}
        </>
      )}
    </div>
  );
}
