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

const REVEAL_CLASS_NAME = cn(
  ACTIONS_CLASS_NAME,
  "opacity-0 pointer-events-none transition-opacity duration-slow motion-reduce:transition-none",
  "group-hover/message:opacity-100 group-hover/message:pointer-events-auto",
  "focus-within:opacity-100 focus-within:pointer-events-auto"
);

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
  const { source, timestampMs, visible } = deriveMessageActions(message);
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
      className="text-muted hover:text-fg"
      data-testid={`${testId}-copy`}
    />
  );
  return (
    <div
      data-testid={testId}
      className={cn(REVEAL_CLASS_NAME, align === "end" ? "justify-end" : "justify-start")}
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
