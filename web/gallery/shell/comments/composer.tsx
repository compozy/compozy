/** The new-comment form next to the picked element or region, over the canvas. */
import { useState } from "react";

import { Button, Kbd, KbdGroup, Textarea } from "@compozy/ui";

import type { Size } from "#gallery/shell/comments/dom";
import type { ElementTarget } from "#gallery/shell/comments/model";
import { AttachmentInput } from "#gallery/shell/comments/attachments";
import { useAttachments } from "#gallery/shell/comments/use-attachments";

const panelWidth = 304;
/** Room kept below the form's top edge, so it never runs off the canvas. */
const panelHeight = 260;

type ComposerProps = {
  slug: string;
  target: ElementTarget;
  scale: number;
  bounds: Size;
  onCancel: () => void;
  onSubmit: (body: string, attachments: string[]) => Promise<void>;
};

export function CommentComposer({
  slug,
  target,
  scale,
  bounds,
  onCancel,
  onSubmit,
}: ComposerProps) {
  const uploads = useAttachments(slug);
  const [body, setBody] = useState("");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const right = (target.rect.x + target.rect.width) * scale + 12;
  const left =
    right + panelWidth <= bounds.width
      ? right
      : Math.max(8, target.rect.x * scale - panelWidth - 12);
  const top = Math.min(
    Math.max(8, target.rect.y * scale),
    Math.max(8, bounds.height - panelHeight)
  );
  const canSubmit = body.trim() !== "" && !pending && uploads.ready;

  const submit = async () => {
    if (!canSubmit) return;
    setPending(true);
    setError(null);
    try {
      await onSubmit(body, uploads.ids);
      uploads.committed();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason));
      setPending(false);
    }
  };

  return (
    <form
      className="pointer-events-auto absolute flex cursor-auto flex-col gap-2.5 rounded-lg bg-popover p-3 text-fg shadow-pop"
      data-testid="comment-composer"
      onPointerDown={event => event.stopPropagation()}
      onSubmit={event => {
        event.preventDefault();
        void submit();
      }}
      style={{ left, top, width: panelWidth }}
    >
      <div className="flex min-w-0 flex-col gap-0.5">
        <p className="text-small-body font-medium">
          {target.kind === "region" ? "Comment on this area" : `Comment on this <${target.tag}>`}
        </p>
        {target.text ? <p className="truncate text-meta text-muted">“{target.text}”</p> : null}
        <p
          className="truncate font-mono text-mono-id tracking-mono-id text-subtle"
          title={target.source ?? target.selector}
        >
          {target.source ?? target.selector}
        </p>
      </div>
      <Textarea
        aria-label="Comment"
        autoFocus
        className="min-h-20"
        maxLength={10_000}
        onChange={event => setBody(event.target.value)}
        onKeyDown={event => {
          if (event.key === "Enter" && (event.metaKey || event.ctrlKey)) {
            event.preventDefault();
            void submit();
          }
          if (event.key === "Escape") {
            event.stopPropagation();
            onCancel();
          }
        }}
        onPaste={event => {
          if (event.clipboardData.files.length === 0) return;
          event.preventDefault();
          uploads.add([...event.clipboardData.files]);
        }}
        placeholder="What should change here?"
        value={body}
      />
      <AttachmentInput disabled={pending} uploads={uploads} />
      {error ? (
        <p className="text-meta text-danger" role="alert">
          Could not save the comment: {error}
        </p>
      ) : null}
      <div className="flex items-center gap-2">
        <KbdGroup className="mr-auto">
          <Kbd>⌘</Kbd>
          <Kbd>↵</Kbd>
        </KbdGroup>
        <Button onClick={onCancel} size="sm" type="button" variant="ghost">
          Cancel
        </Button>
        <Button data-testid="comment-submit" disabled={!canSubmit} size="sm" type="submit">
          Comment
        </Button>
      </div>
    </form>
  );
}
