import { CornerDownRight, ListPlus, Pencil, Trash2 } from "lucide-react";

import { cn } from "@/lib/utils";
import type { QueuedPrompt } from "@/systems/session";
import { Button } from "@compozy/ui";
import { queuedPromptPreview } from "./session-composer-queued-prompts.logic";

interface SessionComposerQueuedPromptsProps {
  prompts: QueuedPrompt[];
  onSteer: (prompt: QueuedPrompt) => void;
  onEdit: (prompt: QueuedPrompt) => void;
  onRemove: (id: string) => void;
  disabled?: boolean;
}

/**
 * Queued follow-up rows fused onto the top of the composer (`.qstrip`/`.qrow`):
 * quiet 32px rows — queue glyph · single-line preview · Steer (inject into the
 * running turn) · edit (back into the field) · remove. No accent, no fills —
 * the queue is bookkeeping, not an event. Truthful: every row is a real durable
 * queue entry the daemon accepted, never a fabricated placeholder.
 */
export function SessionComposerQueuedPrompts({
  prompts,
  onSteer,
  onEdit,
  onRemove,
  disabled = false,
}: SessionComposerQueuedPromptsProps) {
  if (prompts.length === 0) {
    return null;
  }

  return (
    <div
      data-testid="composer-queued-prompts"
      className={cn(
        "flex flex-col rounded-t-lg border border-b-0 border-line bg-elevated",
        // Squared under a docked decision panel so the stack reads as one shape.
        "group-has-[[data-slot=dock]]/composer:rounded-none"
      )}
    >
      {prompts.map((prompt, index) => {
        const mutable = prompt.status === undefined || prompt.status === "queued";
        return (
          <div
            key={prompt.id}
            data-testid="composer-queued-prompt-row"
            className={cn(
              "flex min-h-8 min-w-0 items-center gap-2 py-transcript-meta-gap pr-transcript-inline-gap pl-3",
              index > 0 && "border-t border-line-soft"
            )}
          >
            <ListPlus aria-hidden="true" className="size-3 shrink-0 text-faint" />
            <span
              className="min-w-0 flex-1 truncate text-transcript-body text-muted"
              title={prompt.text}
            >
              {queuedPromptPreview(prompt.text)}
            </span>
            <div className="flex shrink-0 items-center gap-px">
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={() => onSteer(prompt)}
                disabled={disabled || !mutable}
                data-testid="composer-queued-steer"
                className="text-muted hover:text-fg-strong"
              >
                <CornerDownRight aria-hidden="true" className="size-3" />
                Steer
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="icon-xs"
                onClick={() => onEdit(prompt)}
                disabled={disabled || !mutable}
                data-testid="composer-queued-edit"
                aria-label="Edit queued prompt"
                className="text-faint hover:text-fg"
              >
                <Pencil aria-hidden="true" className="size-3" />
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="icon-xs"
                onClick={() => onRemove(prompt.id)}
                disabled={disabled || !mutable}
                data-testid="composer-queued-remove"
                aria-label="Remove queued prompt"
                className="text-faint hover:text-fg"
              >
                <Trash2 aria-hidden="true" className="size-3" />
              </Button>
            </div>
          </div>
        );
      })}
    </div>
  );
}
