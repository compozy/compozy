import { useAuiState } from "@assistant-ui/react";

import { cn } from "@/lib/utils";
import {
  formatMessageTimestamp,
  formatMessageTimestampFull,
} from "@/systems/session/lib/format-timestamp";
import { Button, CopyIconButton } from "@agh/ui";
import { useSessionComposerPrefill } from "./hooks/use-session-composer-prefill";
import { deriveMessageActions } from "./message-actions.logic";

const ACTIONS_CLASS_NAME = "flex items-center gap-2 text-small-body text-muted tabular-nums";

// Non-Goal actions reveal on hover/focus. A Goal action stays visible so its
// pointer target exists before hover and on devices without hover support.
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
  goalPrefill?: boolean;
}

export function MessageActions({
  align,
  copyLabel,
  testId,
  goalPrefill = false,
}: MessageActionsProps) {
  const setComposerText = useSessionComposerPrefill();
  const message = useAuiState(
    state => state.message as { content?: unknown; status?: { type?: string } }
  );
  const { source, timestampMs, visible, goalEligible } = deriveMessageActions(message);
  const prefillGoal =
    goalPrefill && goalEligible && setComposerText
      ? () => setComposerText(`/goal ${source}`)
      : null;

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
  const useAsGoal = prefillGoal ? (
    <Button
      type="button"
      variant="ghost"
      size="xs"
      className="px-1 text-muted hover:text-fg"
      data-testid={`${testId}-goal-prefill`}
      onClick={prefillGoal}
    >
      Use as Goal
    </Button>
  ) : null;

  return (
    <div
      data-testid={testId}
      className={cn(
        prefillGoal ? ACTIONS_CLASS_NAME : REVEAL_CLASS_NAME,
        align === "end" ? "justify-end" : "justify-start"
      )}
    >
      {align === "end" ? (
        <>
          {timestamp}
          {copy}
        </>
      ) : (
        <>
          {copy}
          {useAsGoal}
          {timestamp}
        </>
      )}
    </div>
  );
}
