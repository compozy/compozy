import { Square } from "@compozy/ui/icons";
import type { MouseEvent, ReactNode } from "react";

import { primaryShortcutModifier } from "@/systems/os/lib/window-manager-shortcuts";
import type { SessionPromptCapability } from "@/systems/session/lib/session-prompt-capability";
import {
  InputGroupButton,
  Kbd,
  Spinner,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@compozy/ui";

import { SessionAttachButton } from "./session-attach-button";
import { SessionComposerSendButton } from "./session-composer-send-button";
import type { SessionComposerEnterHint } from "./hooks/use-session-composer-controller";

interface SessionComposerActionRowProps {
  actionState: SessionComposerActionState;
  /** `false` leaves out the attach button for hosts that carry text only. */
  attachments?: boolean;
  hasStagedQuote?: boolean;
  sessionId: string;
  /** Present while the live stream is down: Send answers with the guard note instead of a request. */
  handleDisconnectedSend?: () => void;
  /** The Send control's press when the composer's interceptor claims the text (`/side`). */
  handleInterceptedSend?: () => void;
  onCancelPrompt: () => void;
  promptEmbeddedContextCapability: SessionPromptCapability;
  promptImageCapability: SessionPromptCapability;
  runtimeControl?: ReactNode;
  environmentControl?: ReactNode;
  contextControl?: ReactNode;
  mcpControl?: ReactNode;
  subagentControl?: ReactNode;
  monitorControl?: ReactNode;
  processControl?: ReactNode;
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
      <InputGroupButton
        variant="secondary"
        size="tool"
        aria-busy="true"
        aria-disabled="true"
        aria-live="polite"
        data-state="stopping"
        data-testid="composer-stop-button"
        className="cursor-default text-subtle hover:bg-surface-2 hover:text-subtle hover:shadow-none"
      >
        <Spinner aria-hidden="true" className="size-3" />
        Stopping…
      </InputGroupButton>
    );
  }
  const handleClick = (event: MouseEvent<HTMLButtonElement>) => {
    if (event.detail > 1) return;
    onCancelPrompt();
  };
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <InputGroupButton
            variant="secondary"
            size="send"
            onClick={handleClick}
            aria-label="Stop generation"
            data-state="stop"
            data-testid="composer-stop-button"
            className="text-muted-foreground hover:bg-destructive-tint hover:text-destructive hover:shadow-none"
          />
        }
      >
        <Square aria-hidden="true" className="size-3 fill-current" />
      </TooltipTrigger>
      <TooltipContent>Stop</TooltipContent>
    </Tooltip>
  );
}

function modifierKeyLabel(): string {
  const platform = typeof navigator === "undefined" ? "" : navigator.platform;
  return primaryShortcutModifier(platform) === "meta" ? "⌘⏎" : "Ctrl⏎";
}

/**
 * What Enter does right now, and the one-shot opposite the modifier applies,
 * for assistive tech only. While a turn runs the row shows only Stop: Enter
 * and the modifier are how a follow-up queues or steers, and this hint names
 * them for screen readers (#826).
 */
function SessionComposerEnterHintLabel({ enterHint }: { enterHint: SessionComposerEnterHint }) {
  return (
    <span
      data-testid="composer-enter-hint"
      data-enter={enterHint.enter}
      data-modifier={enterHint.modifier ?? undefined}
      className="sr-only"
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
  );
}

export function SessionComposerActionRow({
  actionState,
  attachments = true,
  hasStagedQuote = false,
  sessionId,
  handleDisconnectedSend,
  handleInterceptedSend,
  onCancelPrompt,
  promptEmbeddedContextCapability,
  promptImageCapability,
  runtimeControl,
  environmentControl,
  contextControl,
  mcpControl,
  subagentControl,
  monitorControl,
  processControl,
}: SessionComposerActionRowProps) {
  const canPrompt = actionState.prompt === "enabled";
  const busyControls = actionState.controls.kind === "busy" ? actionState.controls : null;
  const attachControl = canPrompt && attachments ? <SessionAttachButton /> : null;

  // Every control is 28 px tall (`tool-icon` squares, `tool` pills, the send disc).
  // Controls inside a group sit 4 px apart; the tool and action groups 8 px apart.
  return (
    <div className="flex min-h-(--size-button-icon-default) flex-wrap items-center gap-2">
      {runtimeControl ||
      environmentControl ||
      contextControl ||
      mcpControl ||
      subagentControl ||
      monitorControl ||
      processControl ||
      attachControl ? (
        <div className="flex min-w-0 flex-wrap items-center gap-1">
          {runtimeControl}
          {environmentControl}
          {contextControl}
          {subagentControl}
          {monitorControl}
          {processControl}
          {attachControl}
          {mcpControl}
        </div>
      ) : null}
      {canPrompt ? <SessionComposerEnterHintLabel enterHint={actionState.enterHint} /> : null}
      <span className="flex-1" />
      {busyControls ? (
        // While a turn runs the row holds only Stop, where the send disc sits; Enter and the
        // modifier queue or steer a follow-up.
        <SessionComposerStopControl
          onCancelPrompt={onCancelPrompt}
          stopping={busyControls.stopping}
        />
      ) : (
        <SessionComposerSendButton
          canPrompt={canPrompt}
          hasStagedQuote={hasStagedQuote}
          onDisconnectedSend={handleDisconnectedSend}
          onInterceptedSend={handleInterceptedSend}
          sessionId={sessionId}
          promptEmbeddedContextCapability={promptEmbeddedContextCapability}
          promptImageCapability={promptImageCapability}
        />
      )}
    </div>
  );
}
