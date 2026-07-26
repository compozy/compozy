import { ComposerPrimitive } from "@assistant-ui/react";
import { FilePenLine, ListPlus, Scissors, SendHorizontal, Square, X } from "lucide-react";
import type { KeyboardEvent } from "react";
import { toast } from "sonner";

import { cn } from "@/lib/utils";
import { Button } from "@agh/ui";
import { SessionComposerQueuedPrompts, type QueuedPrompt } from "./session-composer-queued-prompts";
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
}

function describeComposerActionError(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message.trim().length > 0) {
    return error.message;
  }
  return fallback;
}

/**
 * The session prompt composer. Idle: an accent Send disc submits to the runtime.
 * While a turn runs the phase changes coherently — Enter queues the draft (with a
 * visible hint), the primary disc becomes a danger Stop, and Queue/Interrupt stay
 * available. Queued follow-ups render fused onto the composer top with
 * steer/edit/remove actions (steer lives on the queue strip, not as a composer CTA).
 */
export function SessionComposer({
  composerState,
  contentInset,
  canPrompt,
  onCancelPrompt,
  onQueuePrompt,
  onInterruptPrompt,
  isBusyInputPending = false,
  isSessionRunning = false,
  allowBusyInput = true,
  queuedPrompts = EMPTY_QUEUED_PROMPTS,
  onRemoveQueuedPrompt,
  onSteerQueuedPrompt,
  inactivePlaceholder = "Session is not active",
}: SessionComposerProps & { composerState: SessionComposerState }) {
  const { clearComposer, setComposerInputElement, setComposerText, composerText, isRunning } =
    composerState;
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
    <div className="border-t border-line bg-canvas-soft" data-testid="composer-shell">
      <ThreadContentRail
        inset={contentInset ?? SESSION_THREAD_CONTENT_INSET_DEFAULT}
        className="py-3"
      >
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
            "flex flex-col gap-2 border border-line-strong bg-elevated px-3 pt-2.5 pb-2",
            "transition-colors focus-within:border-accent",
            showQueuedStrip ? "rounded-b-xl border-t-0" : "rounded-xl"
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
            onKeyDown={handleInputKeyDown}
            className={cn(
              "min-h-6 w-full resize-none border-none bg-transparent p-0 text-small-body leading-relaxed",
              "text-fg placeholder:text-subtle",
              "outline-none focus-visible:border-transparent focus-visible:ring-0",
              "dark:bg-transparent"
            )}
          />
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="flex min-w-0 items-center gap-2">
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
              {runtimeRunning && canQueueFromInput ? (
                <span data-testid="composer-enter-hint" className="text-form-label text-subtle">
                  <kbd className="font-mono not-italic text-subtle">Enter</kbd> to queue
                </span>
              ) : null}
            </div>

            <div className="flex flex-wrap items-center justify-end gap-2">
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
                      "inline-flex size-button-icon-lg items-center justify-center rounded-full",
                      "bg-danger text-canvas transition-opacity",
                      "hover:opacity-90 focus-visible:outline-none focus-visible:shadow-focus-ring"
                    )}
                  >
                    <Square className="size-3.5 fill-current" />
                  </button>
                </>
              ) : (
                <ComposerPrimitive.Send
                  aria-label="Send message"
                  disabled={!canPrompt}
                  className={cn(
                    "inline-flex size-button-icon-lg items-center justify-center rounded-full",
                    "bg-accent text-accent-ink transition-colors",
                    "hover:bg-accent-hover disabled:cursor-not-allowed disabled:opacity-50"
                  )}
                  data-testid="composer-send-button"
                >
                  <SendHorizontal className="size-3.5" />
                </ComposerPrimitive.Send>
              )}
            </div>
          </div>
        </ComposerPrimitive.Root>
      </ThreadContentRail>
    </div>
  );
}
