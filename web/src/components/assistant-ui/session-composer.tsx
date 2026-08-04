import { ComposerPrimitive } from "@assistant-ui/react";
import { ArrowUp, CornerDownRight, FilePenLine, ListPlus, Scissors, Square, X } from "lucide-react";
import type { ReactNode } from "react";

import { cn } from "@/lib/utils";
import type { QueuedPrompt } from "@/systems/session";
import { Button } from "@compozy/ui";
import { SessionComposerQueuedPrompts } from "./session-composer-queued-prompts";
import {
  useSessionBusyInputActions,
  type SessionBusyInputHandler,
} from "./hooks/use-session-busy-input-actions";
import type { SessionComposerState } from "./hooks/use-session-composer-state";
import {
  SESSION_THREAD_CONTENT_INSET_DEFAULT,
  ThreadContentRail,
  type SessionThreadContentInset,
} from "./session-thread-content-rail";

export type { SessionBusyInputHandler } from "./hooks/use-session-busy-input-actions";

const EMPTY_QUEUED_PROMPTS: QueuedPrompt[] = [];

export interface SessionComposerProps {
  canPrompt: boolean;
  onCancelPrompt: () => void;
  onQueuePrompt?: SessionBusyInputHandler;
  onInterruptPrompt?: SessionBusyInputHandler;
  onSteerPrompt?: SessionBusyInputHandler;
  isBusyInputPending?: boolean;
  isSessionRunning?: boolean;
  allowBusyInput?: boolean;
  busyInputFenceAvailable?: boolean;
  queuedPrompts?: QueuedPrompt[];
  onRemoveQueuedPrompt?: (id: string) => void;
  onReplaceQueuedPrompt?: (prompt: QueuedPrompt, message: string) => Promise<unknown>;
  onSteerQueuedPrompt?: (prompt: QueuedPrompt) => void;
  contentInset?: SessionThreadContentInset;
  inactivePlaceholder?: string;
  /** Pending decision panel fused onto the composer top (permission/clarification dock). */
  decisionDock?: ReactNode;
  /** Runtime picker that applies to the next submitted prompt. */
  runtimeControl?: ReactNode;
}

/**
 * The session prompt composer. Idle: an accent Send disc submits to the runtime.
 * While a turn runs the phase changes coherently — Enter queues the draft (with a
 * visible hint), the primary disc becomes a danger Stop, and Queue/Interrupt stay
 * available. A direct Steer action replaces the fenced active turn with the current draft;
 * queued follow-ups keep their separate steer/edit/remove actions.
 */
export function SessionComposer({
  composerState,
  contentInset,
  canPrompt,
  onCancelPrompt,
  onQueuePrompt,
  onInterruptPrompt,
  onSteerPrompt,
  isBusyInputPending = false,
  isSessionRunning = false,
  allowBusyInput = true,
  busyInputFenceAvailable = true,
  queuedPrompts = EMPTY_QUEUED_PROMPTS,
  onRemoveQueuedPrompt,
  onReplaceQueuedPrompt,
  onSteerQueuedPrompt,
  inactivePlaceholder = "Session is not active",
  decisionDock,
  runtimeControl,
}: SessionComposerProps & { composerState: SessionComposerState }) {
  const {
    clearComposer,
    persistComposerText,
    setComposerInputElement,
    setComposerText,
    composerText,
    isRunning,
  } = composerState;
  const trimmedComposerText = composerText.trim();
  const goalCommandReady =
    trimmedComposerText === "/goal" || trimmedComposerText.startsWith("/goal ");
  const runtimeRunning = isRunning || isSessionRunning;
  const canSubmitBusyInput =
    runtimeRunning &&
    canPrompt &&
    allowBusyInput &&
    trimmedComposerText.length > 0 &&
    !isBusyInputPending;
  const showBusyControls = runtimeRunning || isBusyInputPending;
  const canQueueFromInput = allowBusyInput && Boolean(onQueuePrompt);
  const hasQueuedPrompts = queuedPrompts.length > 0;
  const showQueuedStrip = hasQueuedPrompts && Boolean(onRemoveQueuedPrompt && onSteerQueuedPrompt);
  const {
    handleComposerChange,
    handleEditQueuedPrompt,
    handleInputKeyDown,
    handleInterruptAction,
    handleQueueAction,
    handleRemoveQueuedPrompt,
    handleSteerAction,
  } = useSessionBusyInputActions({
    canQueueFromInput,
    canSubmitBusyInput,
    clearComposer,
    persistComposerText,
    onInterruptPrompt,
    onQueuePrompt,
    onRemoveQueuedPrompt,
    onReplaceQueuedPrompt,
    onSteerPrompt,
    queuedPrompts,
    runtimeRunning,
    setComposerText,
    trimmedComposerText,
  });

  return (
    // The composer zone continues the canvas — no top border; the transcript's
    // scroll edge does the separation.
    <div className="bg-canvas" data-testid="composer-shell">
      <ThreadContentRail
        inset={contentInset ?? SESSION_THREAD_CONTENT_INSET_DEFAULT}
        className="pt-1.5 pb-4"
      >
        <div className="group/composer flex min-w-0 flex-col">
          {decisionDock}
          {showQueuedStrip ? (
            <SessionComposerQueuedPrompts
              prompts={queuedPrompts}
              onSteer={onSteerQueuedPrompt!}
              onEdit={handleEditQueuedPrompt}
              onRemove={handleRemoveQueuedPrompt}
              disabled={isBusyInputPending}
              editDisabled={!onReplaceQueuedPrompt}
              steerDisabled={!busyInputFenceAvailable}
            />
          ) : null}
          <ComposerPrimitive.Root
            className={cn(
              "flex flex-col gap-[7px] rounded-lg border border-line bg-elevated shadow-highlight",
              "pt-[11px] pr-2.5 pb-2 pl-3.5",
              "transition-colors duration-base ease-out",
              "hover:border-line-strong focus-within:border-accent-dim",
              "group-has-[[data-slot=dock]]/composer:rounded-t-none",
              showQueuedStrip ? "rounded-t-none" : null
            )}
          >
            <ComposerPrimitive.Input
              ref={setComposerInputElement}
              aria-label="Session prompt"
              data-testid="composer-textarea"
              disabled={!canPrompt}
              placeholder={canPrompt ? "Send a message…" : inactivePlaceholder}
              rows={1}
              maxRows={12}
              submitMode="enter"
              onChange={handleComposerChange}
              onKeyDown={handleInputKeyDown}
              className={cn(
                "min-h-6 w-full resize-none border-none bg-transparent p-0 text-small-body leading-relaxed",
                "text-fg placeholder:text-subtle",
                "outline-none focus-visible:border-transparent focus-visible:ring-0",
                "dark:bg-transparent"
              )}
            />
            <div className="flex min-h-7 flex-wrap items-center gap-2">
              {runtimeControl ? (
                <div className="flex min-w-0 items-center">{runtimeControl}</div>
              ) : null}
              {goalCommandReady ? (
                <div
                  role="status"
                  aria-live="polite"
                  className="flex min-w-0 items-center gap-1.5 text-form-label text-info"
                >
                  <FilePenLine className="size-3.5 shrink-0" aria-hidden="true" />
                  <span>Goal command draft</span>
                  <Button
                    type="button"
                    variant="ghost"
                    size="xs"
                    aria-label="Discard Goal command"
                    onClick={clearComposer}
                    className="size-6 p-0 text-muted hover:text-fg"
                  >
                    <X className="size-3" aria-hidden="true" />
                  </Button>
                </div>
              ) : null}
              {canPrompt ? (
                <span
                  data-testid="composer-enter-hint"
                  className="inline-flex items-center gap-[5px] text-[10.5px] text-faint"
                >
                  <kbd className="rounded-xs border border-line bg-canvas-soft px-1 py-px font-mono text-[9px] not-italic text-subtle">
                    ⏎
                  </kbd>
                  {runtimeRunning && canQueueFromInput ? "queue" : "send"}
                </span>
              ) : null}
              <span className="flex-1" />

              {showBusyControls ? (
                <>
                  {allowBusyInput && onQueuePrompt ? (
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
                  {allowBusyInput && onSteerPrompt ? (
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      onClick={handleSteerAction}
                      disabled={!canSubmitBusyInput || !busyInputFenceAvailable}
                      data-testid="composer-steer-button"
                    >
                      <CornerDownRight className="size-3" />
                      Steer
                    </Button>
                  ) : null}
                  {allowBusyInput && onInterruptPrompt ? (
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
                      "inline-flex size-7 items-center justify-center rounded-full",
                      "border border-line text-muted transition-colors duration-base ease-out",
                      "hover:border-transparent hover:bg-danger-tint hover:text-danger",
                      "focus-visible:shadow-focus-ring focus-visible:outline-none"
                    )}
                  >
                    <Square className="size-3 fill-current" />
                  </button>
                </>
              ) : (
                <ComposerPrimitive.Send
                  aria-label="Send message"
                  disabled={!canPrompt}
                  className={cn(
                    "inline-flex size-7 items-center justify-center rounded-full",
                    "bg-accent text-accent-ink shadow-highlight transition-colors duration-base ease-out",
                    "hover:bg-accent-hover disabled:cursor-not-allowed disabled:bg-btn-default-fill disabled:text-faint disabled:opacity-100 disabled:shadow-none"
                  )}
                  data-testid="composer-send-button"
                >
                  <ArrowUp className="size-3.5" />
                </ComposerPrimitive.Send>
              )}
            </div>
          </ComposerPrimitive.Root>
        </div>
      </ThreadContentRail>
    </div>
  );
}
