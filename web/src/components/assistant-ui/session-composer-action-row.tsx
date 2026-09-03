import { CornerDownRight, ListPlus, Scissors, Square } from "lucide-react";
import type { ReactNode } from "react";

import { cn } from "@/lib/utils";
import type { SessionPromptCapability } from "@/systems/session/lib/session-prompt-capability";
import { Button, Kbd } from "@compozy/ui";

import { SessionAttachButton } from "./session-attach-button";
import { SessionComposerSendButton } from "./session-composer-send-button";
import type { SessionBusyInputHandler } from "./hooks/use-session-busy-input-actions";

interface SessionComposerActionRowProps {
  actionState: SessionComposerActionState;
  composerAttachmentCount: number;
  hasStagedQuote?: boolean;
  sessionId: string;
  handleInterruptAction: () => void;
  handleQueueAction: () => void;
  handleSteerAction: () => void;
  onCancelPrompt: () => void;
  onInterruptPrompt?: SessionBusyInputHandler;
  onQueuePrompt?: SessionBusyInputHandler;
  onSteerPrompt?: SessionBusyInputHandler;
  promptEmbeddedContextCapability: SessionPromptCapability;
  promptImageCapability: SessionPromptCapability;
  runtimeControl?: ReactNode;
  environmentControl?: ReactNode;
}

type SessionComposerActionState = {
  prompt: "enabled" | "disabled";
  enterHint: "send" | "queue";
  controls:
    | { kind: "send" }
    | {
        kind: "busy";
        submission: "enabled" | "disabled";
        fence: "available" | "unavailable";
      };
};

export function SessionComposerActionRow({
  actionState,
  composerAttachmentCount,
  hasStagedQuote = false,
  sessionId,
  handleInterruptAction,
  handleQueueAction,
  handleSteerAction,
  onCancelPrompt,
  onInterruptPrompt,
  onQueuePrompt,
  onSteerPrompt,
  promptEmbeddedContextCapability,
  promptImageCapability,
  runtimeControl,
  environmentControl,
}: SessionComposerActionRowProps) {
  const canPrompt = actionState.prompt === "enabled";
  const busyControls = actionState.controls.kind === "busy" ? actionState.controls : null;
  const canSubmitBusyInput = busyControls?.submission === "enabled";
  const busyInputFenceAvailable = busyControls?.fence === "available";

  return (
    <div className="flex min-h-7 flex-wrap items-center gap-2">
      {runtimeControl || environmentControl ? (
        <div className="flex min-w-0 items-center gap-2">
          {runtimeControl}
          {environmentControl}
        </div>
      ) : null}
      {canPrompt ? <SessionAttachButton /> : null}
      {canPrompt ? (
        <span
          data-testid="composer-enter-hint"
          className="inline-flex items-center gap-1 text-micro text-faint"
        >
          <Kbd>⏎</Kbd>
          {actionState.enterHint}
        </span>
      ) : null}
      <span className="flex-1" />
      {busyControls ? (
        <>
          {onQueuePrompt ? (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={handleQueueAction}
              disabled={!canSubmitBusyInput}
              data-testid="composer-queue-button"
            >
              <ListPlus className="size-3" />
              Queue
            </Button>
          ) : null}
          {onSteerPrompt ? (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={handleSteerAction}
              disabled={
                !canSubmitBusyInput || composerAttachmentCount > 0 || !busyInputFenceAvailable
              }
              data-testid="composer-steer-button"
            >
              <CornerDownRight className="size-3" />
              Steer
            </Button>
          ) : null}
          {onInterruptPrompt ? (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={handleInterruptAction}
              disabled={!canSubmitBusyInput || !busyInputFenceAvailable}
              data-testid="composer-interrupt-button"
            >
              <Scissors className="size-3" />
              Interrupt
            </Button>
          ) : null}
          <button
            type="button"
            onClick={onCancelPrompt}
            aria-label="Stop generation"
            data-testid="composer-stop-button"
            className={cn(
              "inline-flex size-7 items-center justify-center rounded-full border border-line",
              "text-muted transition-colors duration-base ease-out",
              "hover:border-transparent hover:bg-danger-tint hover:text-danger",
              "focus-visible:shadow-focus-ring focus-visible:outline-none"
            )}
          >
            <Square className="size-3 fill-current" />
          </button>
        </>
      ) : (
        <SessionComposerSendButton
          canPrompt={canPrompt}
          hasStagedQuote={hasStagedQuote}
          sessionId={sessionId}
          promptEmbeddedContextCapability={promptEmbeddedContextCapability}
          promptImageCapability={promptImageCapability}
        />
      )}
    </div>
  );
}
