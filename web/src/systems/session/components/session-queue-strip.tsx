import { Trash2 } from "lucide-react";
import { AnimatePresence, m, useReducedMotionConfig } from "motion/react";
import { useState, type KeyboardEvent, type ReactNode } from "react";
import { toast } from "sonner";

import { cn } from "@/lib/utils";
import { Button, Spinner } from "@compozy/ui";

import type { QueuedPrompt, QueuedPromptEditOutcome } from "../lib/queued-prompt";
import { splitQuotedPrompt } from "../lib/session-terminal-quote-prompt";
import type { UnconfirmedSend } from "../lib/session-unconfirmed-send";
import { SessionQueueEditingRow } from "./session-queue-strip-editor";
import { SessionQueueEntryRow, SessionUnconfirmedRow } from "./session-queue-strip-row";

const EMPTY_UNCONFIRMED: UnconfirmedSend[] = [];

/** A row leaves in one 180ms fade (`--duration-base` / `--ease-out`); nothing else animates. */
const ROW_EXIT_TRANSITION = { duration: 0.18, ease: [0.22, 1, 0.36, 1] as const };

export interface SessionQueueStripProps {
  /** Durable entries from the daemon list, in dispatch order. */
  prompts: QueuedPrompt[];
  /** Client-local sends whose acknowledgment was lost (never in the daemon list). */
  unconfirmedSends?: UnconfirmedSend[];
  /** The daemon's queue cap (queue summary, or the last refusal); the header reads "full" at cap. */
  queueCap?: number | null;
  onSteer: (prompt: QueuedPrompt) => void;
  onRemove: (id: string) => void;
  /**
   * Saves an in-row edit as one atomic replacement; absent when the surface
   * cannot offer editing. Resolves with how the edit ended, never rejects.
   */
  onSaveEdit?: (prompt: QueuedPrompt, text: string) => Promise<QueuedPromptEditOutcome>;
  /** Explicit clear-all; absent when the surface cannot offer it. Resolves once the daemon answered. */
  onClear?: () => Promise<unknown>;
  onRetryUnconfirmed?: (id: string) => void;
  onDiscardUnconfirmed?: (id: string) => void;
  /** Row verbs and Retry are suspended while a send or queue mutation is in flight. */
  disabled?: boolean;
  className?: string;
}

/**
 * The strip is in exactly one of these: idle, editing one row, asking to clear
 * everything, or clearing. Editing and clearing are mutually exclusive, so the
 * row verbs are absent whenever the strip is not idle.
 */
type StripMode =
  | { kind: "idle" }
  | { kind: "editing"; id: string; text: string; saving: boolean }
  | { kind: "confirming-clear" }
  | { kind: "clearing" };

const IDLE: StripMode = { kind: "idle" };

function SessionQueueHeader({
  count,
  full,
  onClearRequested,
}: {
  count: number;
  full: boolean;
  onClearRequested: (() => void) | null;
}) {
  return (
    <div
      data-testid="composer-queue-header"
      data-full={full ? "true" : undefined}
      className="flex h-7 items-center gap-2 border-b border-line-soft pr-1.5 pl-3 text-eyebrow text-subtle"
    >
      <span className="font-mono text-muted tabular-nums" data-testid="composer-queue-count">
        {count}
      </span>
      queued
      {full ? (
        <>
          <span aria-hidden="true" className="text-faint">
            ·
          </span>
          <span data-testid="composer-queue-full">full</span>
        </>
      ) : null}
      <span className="flex-1" />
      {onClearRequested ? (
        <Button
          type="button"
          variant="ghost"
          size="xs"
          onClick={onClearRequested}
          data-testid="composer-queue-clear"
          className="text-muted hover:text-fg-strong"
        >
          Clear all
        </Button>
      ) : null}
    </div>
  );
}

/**
 * The confirmation takes the header's place — no dialog. Clear all wears the
 * destructive tint, Keep is the quiet way out, Escape is Keep.
 */
function SessionQueueClearConfirm({
  count,
  clearing,
  onConfirm,
  onKeep,
}: {
  count: number;
  clearing: boolean;
  onConfirm: () => void;
  onKeep: () => void;
}) {
  const handleKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key === "Escape" && !clearing) {
      event.preventDefault();
      event.stopPropagation();
      onKeep();
    }
  };
  return (
    <div
      role="group"
      aria-label="Clear the queue"
      data-testid="composer-queue-clear-confirm"
      onKeyDown={handleKeyDown}
      className="flex h-8 items-center gap-2 border-b border-line-soft bg-canvas-tint pr-1.5 pl-3 text-eyebrow text-fg"
    >
      <Trash2 aria-hidden="true" className="size-3 shrink-0 text-subtle" />
      <span className="min-w-0 flex-1 truncate">
        Remove all {count} queued {count === 1 ? "follow-up" : "follow-ups"}?
      </span>
      <Button
        type="button"
        variant="destructive"
        size="xs"
        onClick={onConfirm}
        disabled={clearing}
        aria-busy={clearing ? "true" : undefined}
        data-testid="composer-queue-clear-confirm-button"
      >
        {clearing ? <Spinner className="size-3" /> : null}
        Clear all
      </Button>
      <Button
        type="button"
        variant="ghost"
        size="xs"
        onClick={onKeep}
        disabled={clearing}
        autoFocus
        data-testid="composer-queue-clear-keep"
      >
        Keep
      </Button>
    </div>
  );
}

/** Each row leaves with a short fade; reduced motion removes it at once. */
function SessionQueueRowExit({
  children,
  reducedMotion,
}: {
  children: ReactNode;
  reducedMotion: boolean;
}) {
  return (
    <m.div
      className="shrink-0"
      layout={false}
      initial={false}
      exit={{ opacity: 0 }}
      transition={reducedMotion ? { duration: 0 } : ROW_EXIT_TRANSITION}
    >
      {children}
    </m.div>
  );
}

/**
 * The queue as a place the operator manages (S2): a strip fused to the top of
 * the composer, one row per parked follow-up with position, owner attribution,
 * a one-line preview, and the verbs the runtime will honor. Every row is a
 * durable entry the daemon accepted, except the client-local unconfirmed rows.
 * Quiet by design: no accent except Retry, no fills — bookkeeping, not an event.
 */
export function SessionQueueStrip({
  prompts,
  unconfirmedSends = EMPTY_UNCONFIRMED,
  queueCap = null,
  onSteer,
  onRemove,
  onSaveEdit,
  onClear,
  onRetryUnconfirmed,
  onDiscardUnconfirmed,
  disabled = false,
  className,
}: SessionQueueStripProps) {
  const [mode, setMode] = useState<StripMode>(IDLE);
  const reducedMotion = useReducedMotionConfig() === true;
  // Nothing durable to show and nothing local to resolve: the strip is absent, not empty.
  if (prompts.length === 0 && unconfirmedSends.length === 0) {
    return null;
  }
  // An edit whose row left the list (dispatched, removed elsewhere) is over.
  const editing =
    mode.kind === "editing" && prompts.some(prompt => prompt.id === mode.id) ? mode : null;
  const clearPhase =
    mode.kind === "confirming-clear" || mode.kind === "clearing" ? mode.kind : null;
  const idle = editing === null && clearPhase === null;
  const clearable = idle && prompts.length > 0 && onClear !== undefined;
  const full = queueCap !== null && prompts.length >= queueCap;
  const rowsSuspended = disabled || !idle;

  const handleEditRequested = (prompt: QueuedPrompt) => {
    if (!idle) return;
    // The editor holds the annotation only; the terminal-context envelope
    // (when any) rides back unchanged on save.
    setMode({
      id: prompt.id,
      kind: "editing",
      saving: false,
      text: splitQuotedPrompt(prompt.text).annotation,
    });
  };

  const handleSaveEdit = () => {
    if (!editing || !onSaveEdit || editing.saving) return;
    const prompt = prompts.find(candidate => candidate.id === editing.id);
    if (!prompt) return;
    setMode({ ...editing, saving: true });
    onSaveEdit(prompt, editing.text).then(outcome => {
      // The editor stays open only when the request never landed.
      setMode(outcome === "failed" ? { ...editing, saving: false } : IDLE);
    });
  };

  const handleConfirmClear = () => {
    if (!onClear) return;
    setMode({ kind: "clearing" });
    onClear().then(
      () => setMode(IDLE),
      error => {
        console.error("Failed to clear the queue", error);
        toast.error("Couldn't clear the queue.");
        setMode(IDLE);
      }
    );
  };

  return (
    <div
      data-testid="composer-queued-prompts"
      data-clear-phase={clearPhase ?? undefined}
      className={cn(
        "flex flex-col rounded-t-lg border border-b-0 border-line bg-elevated",
        // Squared under a docked decision panel so the stack reads as one shape.
        "group-has-[[data-slot=dock]]/composer:rounded-none",
        className
      )}
    >
      {clearPhase === null ? (
        <SessionQueueHeader
          count={prompts.length}
          full={full}
          onClearRequested={clearable ? () => setMode({ kind: "confirming-clear" }) : null}
        />
      ) : (
        <SessionQueueClearConfirm
          count={prompts.length}
          clearing={clearPhase === "clearing"}
          onConfirm={handleConfirmClear}
          onKeep={() => setMode(IDLE)}
        />
      )}
      {/* Four rows tall at most; the header count tells the truth, the body scrolls. */}
      <div className={cn("flex flex-col overflow-y-auto", editing ? "max-h-48" : "max-h-33")}>
        <AnimatePresence initial={false}>
          {prompts.map((prompt, index) =>
            editing?.id === prompt.id ? (
              <SessionQueueRowExit key={prompt.id} reducedMotion={reducedMotion}>
                <SessionQueueEditingRow
                  prompt={prompt}
                  first={index === 0}
                  text={editing.text}
                  saving={editing.saving}
                  onTextChange={text => setMode({ ...editing, text })}
                  onSave={handleSaveEdit}
                  onCancel={() => setMode(IDLE)}
                />
              </SessionQueueRowExit>
            ) : (
              <SessionQueueRowExit key={prompt.id} reducedMotion={reducedMotion}>
                <SessionQueueEntryRow
                  prompt={prompt}
                  first={index === 0}
                  disabled={rowsSuspended}
                  actionsHidden={!idle}
                  onSteer={onSteer}
                  onEdit={onSaveEdit ? handleEditRequested : undefined}
                  onRemove={onRemove}
                />
              </SessionQueueRowExit>
            )
          )}
          {unconfirmedSends.map((send, index) => (
            <SessionQueueRowExit key={send.id} reducedMotion={reducedMotion}>
              <SessionUnconfirmedRow
                send={send}
                first={prompts.length === 0 && index === 0}
                disabled={rowsSuspended}
                onRetry={onRetryUnconfirmed}
                onDiscard={onDiscardUnconfirmed}
              />
            </SessionQueueRowExit>
          ))}
        </AnimatePresence>
      </div>
    </div>
  );
}
