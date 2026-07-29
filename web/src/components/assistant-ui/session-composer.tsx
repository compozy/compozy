import { ComposerPrimitive } from "@assistant-ui/react";
import { ArrowUp, CornerDownRight, FilePenLine, ListPlus, Scissors, Square, X } from "lucide-react";
import type { KeyboardEvent, ReactNode } from "react";
import { toast } from "sonner";

import { cn } from "@/lib/utils";
import type { QueuedPrompt } from "@/systems/session";
import { Button } from "@compozy/ui";
import { SessionComposerQueuedPrompts } from "./session-composer-queued-prompts";
import type { SessionComposerState } from "./hooks/use-session-composer-state";
import {
  SESSION_THREAD_CONTENT_INSET_DEFAULT,
  ThreadContentRail,
  type SessionThreadContentInset,
} from "./session-thread-content-rail";

export type SessionBusyInputHandler = (message: string) => void | Promise<void>;

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
  queuedPrompts?: QueuedPrompt[];
  onRemoveQueuedPrompt?: (id: string) => void;
  onSteerQueuedPrompt?: (prompt: QueuedPrompt) => void;
  contentInset?: SessionThreadContentInset;
  inactivePlaceholder?: string;
  /** Pending decision panel fused onto the composer top (permission/clarification dock). */
  decisionDock?: ReactNode;
}

function describeComposerActionError(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message.trim().length > 0) {
    return error.message;
  }
  return fallback;
}

function isAbortError(error: unknown): boolean {
  return (
    typeof error === "object" && error !== null && "name" in error && error.name === "AbortError"
  );
}

/**
 * The session prompt composer. Idle: an accent Send disc submits to the runtime.
 * While a turn runs the phase changes coherently — Enter queues the draft (with a
 * visible hint), the primary disc becomes a danger Stop, and Queue/Interrupt stay
 * available. A direct Steer action injects the current draft into the live turn;
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
  queuedPrompts = EMPTY_QUEUED_PROMPTS,
  onRemoveQueuedPrompt,
  onSteerQueuedPrompt,
  inactivePlaceholder = "Session is not active",
  decisionDock,
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

  const handleBusyInputAction = (
    handler: SessionBusyInputHandler | undefined,
    failureMessage: string
  ) => {
    if (!handler || !canSubmitBusyInput) {
      return;
    }

    void Promise.resolve(handler(trimmedComposerText))
      .then(clearComposer)
      .catch(error => {
        if (isAbortError(error)) return;
        toast.error(describeComposerActionError(error, failureMessage));
      });
  };

  // While a turn runs, Enter has ONE defined meaning: queue the draft (matching the
  // primary visible affordance + the "Enter to queue" hint). Intercepting here with
  // preventDefault also suppresses assistant-ui's own Enter-submit, because
  // ComposerPrimitive.Input wires `onKeyDown` through
  // composeEventHandlers(onKeyDown, handleKeyPress): our handler runs first and its
  // preventDefault short-circuits the internal submit.
  const handleInputKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (
      runtimeRunning &&
      canQueueFromInput &&
      event.key === "Enter" &&
      !event.shiftKey &&
      !event.ctrlKey &&
      !event.metaKey &&
      !event.nativeEvent.isComposing
    ) {
      event.preventDefault();
      handleBusyInputAction(onQueuePrompt, "Couldn't queue prompt.");
    }
  };

  const handleEditQueuedPrompt = (prompt: QueuedPrompt) => {
    if (trimmedComposerText.length > 0 && trimmedComposerText !== prompt.text.trim()) {
      toast.warning("Send or clear the current draft before editing a queued prompt.");
      return;
    }
    setComposerText(prompt.text);
    onRemoveQueuedPrompt?.(prompt.id);
  };

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
              onRemove={onRemoveQueuedPrompt!}
              disabled={isBusyInputPending}
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
              onChange={event => persistComposerText(event.currentTarget.value)}
              onKeyDown={handleInputKeyDown}
              className={cn(
                "min-h-6 w-full resize-none border-none bg-transparent p-0 text-small-body leading-relaxed",
                "text-fg placeholder:text-subtle",
                "outline-none focus-visible:border-transparent focus-visible:ring-0",
                "dark:bg-transparent"
              )}
            />
            <div className="flex min-h-7 flex-wrap items-center gap-2">
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
                      onClick={() => handleBusyInputAction(onQueuePrompt, "Couldn't queue prompt.")}
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
                      onClick={() => handleBusyInputAction(onSteerPrompt, "Couldn't steer prompt.")}
                      disabled={!canSubmitBusyInput}
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
                      onClick={() =>
                        handleBusyInputAction(onInterruptPrompt, "Couldn't interrupt prompt.")
                      }
                      disabled={!canSubmitBusyInput}
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
