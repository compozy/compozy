import { CornerDownRight, ListPlus, Pencil, Trash2 } from "lucide-react";

import { cn } from "@/lib/utils";
import type { QueuedPrompt } from "@/systems/session";
import { queuedPromptPreview } from "./session-composer-queued-prompts.logic";

interface SessionComposerQueuedPromptsProps {
  prompts: QueuedPrompt[];
  onSteer: (prompt: QueuedPrompt) => void;
  onEdit: (prompt: QueuedPrompt) => void;
  onRemove: (id: string) => void;
  disabled?: boolean;
}

const ICON_BUTTON_CLASS = cn(
  "grid size-[22px] place-items-center rounded-sm text-faint",
  "transition-colors duration-fast ease-out hover:bg-hover hover:text-fg",
  "focus-visible:shadow-focus-ring focus-visible:outline-none",
  "disabled:pointer-events-none disabled:opacity-50"
);

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
      {prompts.map((prompt, index) => (
        <div
          key={prompt.id}
          data-testid="composer-queued-prompt-row"
          className={cn(
            "flex min-h-8 min-w-0 items-center gap-2 py-[3px] pr-[7px] pl-3",
            index > 0 && "border-t border-line-soft"
          )}
        >
          <ListPlus aria-hidden="true" className="size-3 shrink-0 text-faint" />
          <span className="min-w-0 flex-1 truncate text-[12px] text-muted" title={prompt.text}>
            {queuedPromptPreview(prompt.text)}
          </span>
          <div className="flex shrink-0 items-center gap-px">
            <button
              type="button"
              onClick={() => onSteer(prompt)}
              disabled={disabled}
              data-testid="composer-queued-steer"
              className={cn(
                "inline-flex h-[22px] items-center gap-[5px] rounded-sm px-2",
                "text-[11px] font-medium text-muted",
                "transition-colors duration-fast ease-out hover:bg-hover hover:text-fg-strong",
                "focus-visible:shadow-focus-ring focus-visible:outline-none",
                "disabled:pointer-events-none disabled:opacity-50"
              )}
            >
              <CornerDownRight aria-hidden="true" className="size-[11px]" />
              Steer
            </button>
            <button
              type="button"
              onClick={() => onEdit(prompt)}
              disabled={disabled}
              data-testid="composer-queued-edit"
              aria-label="Edit queued prompt"
              className={ICON_BUTTON_CLASS}
            >
              <Pencil aria-hidden="true" className="size-[11px]" />
            </button>
            <button
              type="button"
              onClick={() => onRemove(prompt.id)}
              disabled={disabled}
              data-testid="composer-queued-remove"
              aria-label="Remove queued prompt"
              className={ICON_BUTTON_CLASS}
            >
              <Trash2 aria-hidden="true" className="size-[11px]" />
            </button>
          </div>
        </div>
      ))}
    </div>
  );
}
