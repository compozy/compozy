import { CornerDownRight, ListPlus, LoaderCircle, Scissors, Square } from "lucide-react";
import type { MouseEvent, ReactNode } from "react";

import { cn } from "@/lib/utils";
import { primaryShortcutModifier } from "@/systems/os";
import type { SessionSteerDelivery } from "@/systems/session";
import type { SessionPromptCapability } from "@/systems/session/lib/session-prompt-capability";
import { Button, Kbd } from "@compozy/ui";

import { SessionAttachButton } from "./session-attach-button";
import { SessionComposerSendButton } from "./session-composer-send-button";
import type { SessionBusyInputHandler } from "./hooks/use-session-busy-input-actions";
import type { SessionComposerEnterHint } from "./hooks/use-session-composer-controller";

interface SessionComposerActionRowProps {
  actionState: SessionComposerActionState;
  busyInputSteerDelivery: SessionSteerDelivery | null;
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
  enterHint: SessionComposerEnterHint;
  controls:
    | { kind: "send" }
    | {
        kind: "busy";
        /** A stop is landing: the primary control reads "Stopping…" and takes no activation. */
        stopping: boolean;
        submission: "enabled" | "disabled";
      };
};

/**
 * The primary control while a turn runs: a Stop disc, or the same element
 * widened into a quiet "Stopping…" pill from the first activation until the
 * daemon confirms the stop (US-009.AC-1). The pill is `aria-disabled`, not
 * `disabled`, so it keeps focus and announces its state; it has no handler, so
 * a second press lands on nothing. The disc itself drops the second click of a
 * double-click (`event.detail > 1`), which also covers a double-click on Send
 * whose second click lands here (US-009.EC-1).
 */
function SessionComposerStopControl({
  onCancelPrompt,
  stopping,
}: {
  onCancelPrompt: () => void;
  stopping: boolean;
}) {
  if (stopping) {
    return (
      <button
        type="button"
        aria-busy="true"
        aria-disabled="true"
        aria-live="polite"
        data-state="stopping"
        data-testid="composer-stop-button"
        className={cn(
          "inline-flex h-7 min-w-7 cursor-default items-center gap-1.5 rounded-full",
          "border border-line-soft bg-input-fill pr-2.5 pl-2 text-form font-medium text-subtle",
          "transition-colors duration-base ease-out",
          "focus-visible:shadow-focus-ring focus-visible:outline-none"
        )}
      >
        <LoaderCircle aria-hidden="true" className="size-3 animate-spin" />
        Stopping…
      </button>
    );
  }
  const handleClick = (event: MouseEvent<HTMLButtonElement>) => {
    if (event.detail > 1) return;
    onCancelPrompt();
  };
  return (
    <button
      type="button"
      onClick={handleClick}
      aria-label="Stop generation"
      data-state="stop"
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
  );
}

/** What a steer would do on this agent, answered by the session resource before any send. */
function steerCapabilityTitle(
  delivery: SessionSteerDelivery | null,
  attachmentCount: number
): string | undefined {
  if (attachmentCount > 0) {
    return "Steer can't carry files on this agent — queue it instead";
  }
  switch (delivery) {
    case "injected":
      return "Delivered into the live turn";
    case "pending_injection":
      return "Delivered when the current tool finishes";
    case "interrupt_fallback":
      return "Interrupts the turn and runs this instead";
    default:
      return undefined;
  }
}

function modifierKeyLabel(): string {
  const platform = typeof navigator === "undefined" ? "" : navigator.platform;
  return primaryShortcutModifier(platform) === "meta" ? "⌘⏎" : "Ctrl⏎";
}

export function SessionComposerActionRow({
  actionState,
  busyInputSteerDelivery,
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
  const { enterHint } = actionState;

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
          data-enter={enterHint.enter}
          data-modifier={enterHint.modifier ?? undefined}
          className="inline-flex items-center gap-1 text-micro text-faint"
        >
          <Kbd>⏎</Kbd>
          {enterHint.enter}
          {enterHint.modifier ? (
            <>
              <span aria-hidden="true">·</span>
              <Kbd>{modifierKeyLabel()}</Kbd>
              {enterHint.modifier}
            </>
          ) : null}
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
              disabled={!canSubmitBusyInput || composerAttachmentCount > 0}
              title={steerCapabilityTitle(busyInputSteerDelivery, composerAttachmentCount)}
              data-steer-delivery={busyInputSteerDelivery ?? undefined}
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
              disabled={!canSubmitBusyInput}
              data-testid="composer-interrupt-button"
            >
              <Scissors className="size-3" />
              Interrupt
            </Button>
          ) : null}
          <SessionComposerStopControl
            onCancelPrompt={onCancelPrompt}
            stopping={busyControls.stopping}
          />
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
