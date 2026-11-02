import { CornerDownRight, ListPlus, Scissors, Square } from "@compozy/ui/icons";
import type { MouseEvent, ReactNode } from "react";

import { primaryShortcutModifier } from "@/systems/os/lib/window-manager-shortcuts";
import type { SessionSteerDelivery } from "@/systems/session";
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
import type { SessionBusyInputHandler } from "./hooks/use-session-busy-input-actions";
import type { SessionComposerEnterHint } from "./hooks/use-session-composer-controller";

interface SessionComposerActionRowProps {
  actionState: SessionComposerActionState;
  /** `false` leaves out the attach button for hosts that carry text only. */
  attachments?: boolean;
  busyInputSteerDelivery: SessionSteerDelivery | null;
  hasStagedQuote?: boolean;
  sessionId: string;
  /** Present while the live stream is down: Send answers with the guard note instead of a request. */
  handleDisconnectedSend?: () => void;
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
  contextControl?: ReactNode;
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

/** What a steer would do on this agent, answered by the session resource before any send. */
function steerCapabilityTitle(delivery: SessionSteerDelivery | null): string | undefined {
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

/**
 * What Enter does right now, and the one-shot opposite the modifier applies,
 * for assistive tech only. The Queue and Steer buttons already name the busy
 * verbs on screen, so the row carries no visible shortcut badges (#826).
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

/**
 * Steer, with what it would do on this agent in a tooltip. The trigger is a
 * wrapper span so the reason stays reachable by hover when the button itself
 * is disabled (a disabled button receives no pointer events).
 */
function SessionComposerSteerButton({
  capability,
  delivery,
  disabled,
  onClick,
}: {
  capability: string | undefined;
  delivery: SessionSteerDelivery | null;
  disabled: boolean;
  onClick: () => void;
}) {
  const button = (
    <InputGroupButton
      size="tool"
      onClick={onClick}
      disabled={disabled}
      aria-description={capability}
      data-steer-delivery={delivery ?? undefined}
      data-testid="composer-steer-button"
    >
      <CornerDownRight aria-hidden="true" />
      Steer
    </InputGroupButton>
  );
  if (!capability) return button;
  return (
    <Tooltip>
      <TooltipTrigger render={<span className="inline-flex" data-testid="composer-steer-hint" />}>
        {button}
      </TooltipTrigger>
      <TooltipContent>{capability}</TooltipContent>
    </Tooltip>
  );
}

/**
 * The busy-turn cluster: queue, steer, and interrupt as the session offers them,
 * then the stop control — one group on the same 4 px rhythm as the tool group,
 * held at the row end like the send disc when a narrow row wraps it, and
 * wrapping inside itself so a 220 px window keeps every action reachable.
 */
function SessionComposerBusyActions({
  busyInputSteerDelivery,
  canSubmitBusyInput,
  handleInterruptAction,
  handleQueueAction,
  handleSteerAction,
  onCancelPrompt,
  onInterruptPrompt,
  onQueuePrompt,
  onSteerPrompt,
  stopping,
}: Pick<
  SessionComposerActionRowProps,
  | "busyInputSteerDelivery"
  | "handleInterruptAction"
  | "handleQueueAction"
  | "handleSteerAction"
  | "onCancelPrompt"
  | "onInterruptPrompt"
  | "onQueuePrompt"
  | "onSteerPrompt"
> & { canSubmitBusyInput: boolean; stopping: boolean }) {
  return (
    <div className="ml-auto flex flex-wrap items-center justify-end gap-1">
      {onQueuePrompt ? (
        <InputGroupButton
          size="tool"
          onClick={handleQueueAction}
          disabled={!canSubmitBusyInput}
          data-testid="composer-queue-button"
        >
          <ListPlus aria-hidden="true" />
          Queue
        </InputGroupButton>
      ) : null}
      {onSteerPrompt ? (
        <SessionComposerSteerButton
          capability={steerCapabilityTitle(busyInputSteerDelivery)}
          delivery={busyInputSteerDelivery}
          disabled={!canSubmitBusyInput}
          onClick={handleSteerAction}
        />
      ) : null}
      {onInterruptPrompt ? (
        <InputGroupButton
          size="tool"
          onClick={handleInterruptAction}
          disabled={!canSubmitBusyInput}
          data-testid="composer-interrupt-button"
        >
          <Scissors aria-hidden="true" />
          Interrupt
        </InputGroupButton>
      ) : null}
      <SessionComposerStopControl onCancelPrompt={onCancelPrompt} stopping={stopping} />
    </div>
  );
}

export function SessionComposerActionRow({
  actionState,
  attachments = true,
  busyInputSteerDelivery,
  hasStagedQuote = false,
  sessionId,
  handleDisconnectedSend,
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
  contextControl,
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
        </div>
      ) : null}
      {canPrompt ? <SessionComposerEnterHintLabel enterHint={actionState.enterHint} /> : null}
      <span className="flex-1" />
      {busyControls ? (
        <SessionComposerBusyActions
          busyInputSteerDelivery={busyInputSteerDelivery}
          canSubmitBusyInput={busyControls.submission === "enabled"}
          handleInterruptAction={handleInterruptAction}
          handleQueueAction={handleQueueAction}
          handleSteerAction={handleSteerAction}
          onCancelPrompt={onCancelPrompt}
          onInterruptPrompt={onInterruptPrompt}
          onQueuePrompt={onQueuePrompt}
          onSteerPrompt={onSteerPrompt}
          stopping={busyControls.stopping}
        />
      ) : (
        <SessionComposerSendButton
          canPrompt={canPrompt}
          hasStagedQuote={hasStagedQuote}
          onDisconnectedSend={handleDisconnectedSend}
          sessionId={sessionId}
          promptEmbeddedContextCapability={promptEmbeddedContextCapability}
          promptImageCapability={promptImageCapability}
        />
      )}
    </div>
  );
}
