import { ComposerPrimitive, useAuiState } from "@assistant-ui/react";
import { ArrowUp } from "lucide-react";

import { cn } from "@/lib/utils";
import type { SessionPromptCapability } from "@/systems/session/lib/session-prompt-capability";

import { sessionComposerSendBlocker } from "./hooks/use-session-composer-send-gate";
import { sessionAttachmentTileState } from "./session-attachment-tile-model";

export function SessionComposerSendButton({
  canPrompt,
  promptEmbeddedContextCapability,
  promptImageCapability,
}: {
  canPrompt: boolean;
  promptEmbeddedContextCapability: SessionPromptCapability;
  promptImageCapability: SessionPromptCapability;
}) {
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
  const disabled =
    !canPrompt || Boolean(blocker) || (text.trim().length === 0 && !hasReadyAttachment);

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
