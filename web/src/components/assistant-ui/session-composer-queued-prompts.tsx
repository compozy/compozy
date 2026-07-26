import { CornerDownRight, ListPlus, Pencil, Trash2 } from "lucide-react";

import { Button } from "@agh/ui";
import { cn } from "@/lib/utils";
import { queuedPromptPreview, type QueuedPrompt } from "./session-composer-queued-prompts.logic";

export type { QueuedPrompt } from "./session-composer-queued-prompts.logic";

interface SessionComposerQueuedPromptsProps {
  prompts: QueuedPrompt[];
  onSteer: (prompt: QueuedPrompt) => void;
  onEdit: (prompt: QueuedPrompt) => void;
  onRemove: (id: string) => void;
  disabled?: boolean;
}

/**
 * Queued follow-up rows fused onto the top of the composer. Each row shows what
 * the operator staged and exposes steer (inject into the running turn), edit
 * (move back into the composer), and remove (cancel the queued entry). Truthful:
 * every row is a real durable queue entry the daemon accepted — never a fabricated
 * placeholder.
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
        "flex flex-col rounded-t-xl border border-b-0 border-line-strong bg-elevated",
        "text-small-body"
      )}
    >
      {prompts.map((prompt, index) => (
        <div
          key={prompt.id}
          data-testid="composer-queued-prompt-row"
          className={cn(
            "group/queued flex min-w-0 items-center gap-2 px-2.5 py-1.5",
            index > 0 && "border-t border-line"
          )}
        >
          <ListPlus aria-hidden="true" className="size-3.5 shrink-0 text-subtle" />
          <span className="min-w-0 flex-1 truncate text-muted" title={prompt.text}>
            {queuedPromptPreview(prompt.text)}
          </span>
          <div className="flex shrink-0 items-center gap-0.5">
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={() => onSteer(prompt)}
              disabled={disabled}
              data-testid="composer-queued-steer"
            >
              <CornerDownRight className="size-3" />
              Steer
            </Button>
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              onClick={() => onEdit(prompt)}
              disabled={disabled}
              data-testid="composer-queued-edit"
              aria-label="Edit queued prompt"
            >
              <Pencil className="size-3" />
            </Button>
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              onClick={() => onRemove(prompt.id)}
              disabled={disabled}
              data-testid="composer-queued-remove"
              aria-label="Remove queued prompt"
            >
              <Trash2 className="size-3" />
            </Button>
          </div>
        </div>
      ))}
    </div>
  );
}
