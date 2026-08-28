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

export function SessionComposerSendButton({
  canPrompt,
  hasStagedQuote = false,
  sessionId,
  promptEmbeddedContextCapability,
  promptImageCapability,
}: {
  canPrompt: boolean;
  hasStagedQuote?: boolean;
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

  if (quoteOnly) {
    return (
      <button
        aria-label="Send message"
        className={cn(
          "inline-flex size-7 items-center justify-center rounded-full",
          "bg-accent text-accent-ink shadow-highlight transition-colors duration-base ease-out",
          "hover:bg-accent-hover disabled:cursor-not-allowed disabled:bg-btn-default-fill disabled:text-faint disabled:opacity-100 disabled:shadow-none",
          "focus-visible:shadow-focus-ring focus-visible:outline-none"
        )}
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
