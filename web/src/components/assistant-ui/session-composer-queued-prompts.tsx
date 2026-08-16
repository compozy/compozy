import { CornerDownRight, ListPlus, Pencil, Trash2 } from "lucide-react";

import { cn } from "@/lib/utils";
import type { QueuedPrompt, QueuedPromptAttachmentSummary } from "@/systems/session";
import { Button } from "@compozy/ui";
import { queuedPromptPreview } from "./session-composer-queued-prompts.logic";

function queuedAttachmentSuffix(summary: QueuedPromptAttachmentSummary): string {
  const parts: string[] = [];
  if (summary.imageCount > 0) {
    parts.push(`${summary.imageCount} ${summary.imageCount === 1 ? "image" : "images"}`);
  }
  if (summary.fileCount > 0) {
    parts.push(`${summary.fileCount} ${summary.fileCount === 1 ? "file" : "files"}`);
  }
  return parts.length > 0 ? `· ${parts.join(" · ")}` : "";
}

function SessionQueuedAttachmentWell({ summary }: { summary: QueuedPromptAttachmentSummary }) {
  const suffix = queuedAttachmentSuffix(summary);
  return (
    <>
      <span
        data-testid="composer-queued-attachment-well"
        className="grid size-5 shrink-0 place-items-center overflow-hidden rounded-xs border border-line bg-canvas-soft"
      >
        {summary.preview?.kind === "image" ? (
          <img src={summary.preview.url} alt="" className="size-full object-cover" />
        ) : (
          <span className="font-mono text-micro font-semibold leading-none text-subtle">
            {summary.preview?.kind === "file" ? summary.preview.mark : "FILE"}
          </span>
        )}
      </span>
      {suffix ? <span className="shrink-0 font-mono text-micro text-faint">{suffix}</span> : null}
    </>
  );
}

interface SessionComposerQueuedPromptsProps {
  prompts: QueuedPrompt[];
  onSteer: (prompt: QueuedPrompt) => void;
  onEdit: (prompt: QueuedPrompt) => void;
  onRemove: (id: string) => void;
  disabled?: boolean;
  editDisabled?: boolean;
  steerDisabled?: boolean;
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
  editDisabled = false,
  steerDisabled = false,
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
            {prompt.attachments ? (
              <SessionQueuedAttachmentWell summary={prompt.attachments} />
            ) : null}
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
                disabled={disabled || steerDisabled || !mutable || Boolean(prompt.attachments)}
                data-testid="composer-queued-steer"
                title={
                  prompt.attachments ? "Queued prompts with files cannot be steered" : undefined
                }
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
                disabled={disabled || editDisabled || !mutable}
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
