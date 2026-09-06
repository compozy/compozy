import { ListPlus } from "lucide-react";
import { useLayoutEffect, useRef, type KeyboardEvent } from "react";

import { cn } from "@/lib/utils";
import { Button, Spinner, Textarea } from "@compozy/ui";

import type { QueuedPrompt } from "../lib/queued-prompt";

export interface SessionQueueEditingRowProps {
  prompt: QueuedPrompt;
  first: boolean;
  /** The annotation being edited — never the terminal-context envelope. */
  text: string;
  saving: boolean;
  onTextChange: (text: string) => void;
  onSave: () => void;
  onCancel: () => void;
}

/**
 * Edit happens in the row (S2 §07): the preview becomes a field, Save and
 * Cancel replace the verbs, and the durable entry stays exactly where it is
 * until the daemon accepts one atomic replacement. Escape cancels; the
 * platform modifier + Enter saves, so a multi-line edit keeps plain Enter.
 */
export function SessionQueueEditingRow({
  prompt,
  first,
  text,
  saving,
  onTextChange,
  onSave,
  onCancel,
}: SessionQueueEditingRowProps) {
  const rowRef = useRef<HTMLDivElement>(null);
  useLayoutEffect(() => {
    rowRef.current?.scrollIntoView?.({ block: "nearest" });
  }, []);
  const canSave = !saving && text.trim().length > 0;
  const handleKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key === "Escape" && !saving) {
      event.preventDefault();
      onCancel();
      return;
    }
    if (event.key === "Enter" && (event.metaKey || event.ctrlKey) && canSave) {
      event.preventDefault();
      onSave();
    }
  };
  return (
    <div
      ref={rowRef}
      data-testid="composer-queued-prompt-row"
      data-status={prompt.status}
      data-editing="true"
      className={cn(
        "flex min-w-0 flex-col gap-1.5 py-2 pr-2 pl-3",
        !first && "border-t border-line-soft"
      )}
    >
      <div className="flex items-center gap-2">
        <span
          data-testid="composer-queued-position"
          className="min-w-3.5 shrink-0 font-mono text-mono-id text-faint tabular-nums"
        >
          #{prompt.position}
        </span>
        <ListPlus aria-hidden="true" className="size-3 shrink-0 text-faint" />
        <span data-testid="composer-queued-state" className="text-micro text-subtle">
          {saving ? "Saving…" : "Editing"}
        </span>
      </div>
      <Textarea
        aria-label="Queued message"
        autoFocus
        data-testid="composer-queued-editor"
        disabled={saving}
        onChange={event => onTextChange(event.target.value)}
        onKeyDown={handleKeyDown}
        rows={2}
        value={text}
        className="resize-none px-2.5 py-1.5 leading-normal"
      />
      <div className="flex items-center justify-end gap-1.5">
        <Button
          type="button"
          variant="ghost"
          size="xs"
          onClick={onCancel}
          disabled={saving}
          data-testid="composer-queued-edit-cancel"
        >
          Cancel
        </Button>
        <Button
          type="button"
          size="xs"
          onClick={onSave}
          disabled={!canSave}
          aria-busy={saving ? "true" : undefined}
          data-testid="composer-queued-edit-save"
        >
          {saving ? <Spinner className="size-3" /> : null}
          Save
        </Button>
      </div>
    </div>
  );
}
