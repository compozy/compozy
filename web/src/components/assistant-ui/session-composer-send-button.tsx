import { ComposerPrimitive, useAui, useAuiState } from "@assistant-ui/react";
import { ArrowUp } from "lucide-react";

import { cn } from "@/lib/utils";
import type { SessionPromptCapability } from "@/systems/session/lib/session-prompt-capability";

import {
  composeQuotedPrompt,
  discardSessionTerminalQuote,
  peekSessionTerminalQuote,
} from "@/systems/session";

import { sessionComposerSendBlocker } from "./hooks/use-session-composer-send-gate";
import { sessionAttachmentTileState } from "./session-attachment-tile-model";

const SEND_BUTTON_CLASS = cn(
  "inline-flex size-7 items-center justify-center rounded-full",
  "bg-accent text-accent-ink shadow-highlight transition-colors duration-base ease-out",
  "hover:bg-accent-hover disabled:cursor-not-allowed disabled:bg-btn-default-fill disabled:text-faint disabled:opacity-100 disabled:shadow-none",
  "focus-visible:shadow-focus-ring focus-visible:outline-none"
);

export function SessionComposerSendButton({
  canPrompt,
  hasStagedQuote = false,
  onDisconnectedSend,
  sessionId,
  promptEmbeddedContextCapability,
  promptImageCapability,
}: {
  canPrompt: boolean;
  hasStagedQuote?: boolean;
  /** While the live stream is down the press answers with the guard note; nothing leaves. */
  onDisconnectedSend?: () => void;
  sessionId: string;
  promptEmbeddedContextCapability: SessionPromptCapability;
  promptImageCapability: SessionPromptCapability;
}) {
  const aui = useAui();
  const attachments = useAuiState(state => state.composer.attachments);
  const text = useAuiState(state => state.composer.text);
  const blocker = sessionComposerSendBlocker({
    attachments,
    promptEmbeddedContextCapability,
    promptImageCapability,
  });
  const hasReadyAttachment = attachments.some(
    attachment => sessionAttachmentTileState(attachment) === "ready"
  );
  const quoteOnly = hasStagedQuote && text.trim().length === 0 && !hasReadyAttachment;
  const disabled =
    !canPrompt ||
    Boolean(blocker) ||
    (text.trim().length === 0 && !hasReadyAttachment && !hasStagedQuote);

  if (onDisconnectedSend) {
    // The send button stays a send button (US-018.AC-3): the guard answers the press.
    return (
      <button
        aria-label="Send message"
        className={SEND_BUTTON_CLASS}
        data-testid="composer-send-button"
        data-transport="disconnected"
        disabled={disabled}
        onClick={onDisconnectedSend}
        title={blocker ?? undefined}
        type="button"
      >
        <ArrowUp className="size-3.5" />
      </button>
    );
  }

  if (quoteOnly) {
    return (
      <button
        aria-label="Send message"
        className={SEND_BUTTON_CLASS}
        data-testid="composer-send-button"
        disabled={disabled}
        onClick={() => {
          aui.thread.append(composeQuotedPrompt("", peekSessionTerminalQuote(sessionId)));
          discardSessionTerminalQuote(sessionId);
        }}
        title={blocker ?? undefined}
        type="button"
      >
        <ArrowUp className="size-3.5" />
      </button>
    );
  }

  return (
    <ComposerPrimitive.Send
      aria-label="Send message"
      disabled={disabled}
      title={blocker ?? undefined}
      className={cn(
        "inline-flex size-7 items-center justify-center rounded-full",
        "bg-accent text-accent-ink shadow-highlight transition-colors duration-base ease-out",
        "hover:bg-accent-hover disabled:cursor-not-allowed disabled:bg-btn-default-fill disabled:text-faint disabled:opacity-100 disabled:shadow-none",
        "focus-visible:shadow-focus-ring focus-visible:outline-none"
      )}
      data-testid="composer-send-button"
    >
      <ArrowUp className="size-3.5" />
    </ComposerPrimitive.Send>
  );
}
