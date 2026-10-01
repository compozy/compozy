import { ComposerPrimitive, useAui, useAuiState } from "@assistant-ui/react";
import { ArrowUp } from "lucide-react";

import { InputGroupButton } from "@compozy/ui";
import type { SessionPromptCapability } from "@/systems/session/lib/session-prompt-capability";

import {
  composeQuotedPrompt,
  discardSessionTerminalQuote,
  peekSessionTerminalQuote,
} from "@/systems/session";

import { sessionComposerSendBlocker } from "./hooks/use-session-composer-send-gate";
import { sessionAttachmentTileState } from "./session-attachment-tile-model";

// The session surface's one deliberate stroke override: the send arrow (like the
// transcript's small check / × receipt marks) draws at 2 so it holds its weight
// inside the filled disc; every other glyph takes the global 1.75.
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
      <InputGroupButton
        aria-label="Send message"
        data-testid="composer-send-button"
        data-transport="disconnected"
        disabled={disabled}
        onClick={onDisconnectedSend}
        size="send"
        title={blocker ?? undefined}
      >
        <ArrowUp strokeWidth={2} />
      </InputGroupButton>
    );
  }

  if (quoteOnly) {
    return (
      <InputGroupButton
        aria-label="Send message"
        data-testid="composer-send-button"
        disabled={disabled}
        onClick={() => {
          aui.thread.append(composeQuotedPrompt("", peekSessionTerminalQuote(sessionId)));
          discardSessionTerminalQuote(sessionId);
        }}
        size="send"
        title={blocker ?? undefined}
      >
        <ArrowUp strokeWidth={2} />
      </InputGroupButton>
    );
  }

  return (
    <ComposerPrimitive.Send
      aria-label="Send message"
      disabled={disabled}
      title={blocker ?? undefined}
      data-testid="composer-send-button"
      render={<InputGroupButton size="send" />}
    >
      <ArrowUp strokeWidth={2} />
    </ComposerPrimitive.Send>
  );
}
