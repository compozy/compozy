/**
 * One comment in the panel: number, state, view and target, its thread, and, when selected, the
 * reply form with resolve/reopen and delete (with confirmation).
 */
import { CheckIcon, RotateCcwIcon, Trash2Icon } from "lucide-react";
import { useEffect, useRef, useState } from "react";

import { Button, Textarea, cn } from "@compozy/ui";

import { commentsApi } from "#gallery/shell/comments/api";
import { AttachmentInput } from "#gallery/shell/comments/attachments";
import { Message } from "#gallery/shell/comments/message";
import { commentNumber, targetSummary, type ReviewComment } from "#gallery/shell/comments/model";
import { useAttachments } from "#gallery/shell/comments/use-attachments";

function CommentActions({ comment }: { comment: ReviewComment }) {
  const uploads = useAttachments(comment.slug);
  const [reply, setReply] = useState("");
  const [confirming, setConfirming] = useState(false);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // Only a sent reply clears the draft; resolve, reopen and delete keep it.
  const run = (action: () => Promise<unknown>) => {
    setPending(true);
    setError(null);
    action()
      .catch((reason: unknown) =>
        setError(reason instanceof Error ? reason.message : String(reason))
      )
      .finally(() => setPending(false));
  };
  const open = comment.status === "open";

  return (
    <form
      className="flex flex-col gap-2"
      onSubmit={event => {
        event.preventDefault();
        if (!reply.trim() || !uploads.ready) return;
        run(() =>
          commentsApi.reply(comment.slug, comment.id, reply, uploads.ids).then(() => {
            uploads.committed();
            setReply("");
          })
        );
      }}
    >
      <Textarea
        aria-label="Reply"
        className="min-h-14"
        maxLength={10_000}
        onChange={event => setReply(event.target.value)}
        onPaste={event => {
          if (event.clipboardData.files.length === 0) return;
          event.preventDefault();
          uploads.add([...event.clipboardData.files]);
        }}
        placeholder="Reply…"
        value={reply}
      />
      <AttachmentInput disabled={pending} uploads={uploads} />
      {error ? (
        <p className="text-meta text-danger" role="alert">
          {error}
        </p>
      ) : null}
      <div className="flex flex-wrap items-center gap-1.5">
        <Button disabled={!reply.trim() || pending || !uploads.ready} size="sm" type="submit">
          Reply
        </Button>
        <Button
          data-testid={open ? "comment-resolve" : "comment-reopen"}
          disabled={pending}
          onClick={() =>
            run(() => commentsApi.setStatus(comment.slug, comment.id, open ? "resolved" : "open"))
          }
          size="sm"
          type="button"
          variant="secondary"
        >
          {open ? <CheckIcon aria-hidden="true" /> : <RotateCcwIcon aria-hidden="true" />}
          {open ? "Resolve" : "Reopen"}
        </Button>
        {confirming ? (
          <span className="ml-auto flex items-center gap-1">
            <Button
              disabled={pending}
              onClick={() => run(() => commentsApi.remove(comment.slug, comment.id))}
              size="sm"
              type="button"
              variant="destructive"
            >
              Delete
            </Button>
            <Button onClick={() => setConfirming(false)} size="sm" type="button" variant="ghost">
              Keep
            </Button>
          </span>
        ) : (
          <Button
            aria-label="Delete comment"
            className="ml-auto"
            onClick={() => setConfirming(true)}
            size="icon-sm"
            type="button"
            variant="quiet"
          >
            <Trash2Icon aria-hidden="true" />
          </Button>
        )}
      </div>
    </form>
  );
}

type CardProps = {
  comment: ReviewComment;
  stateLabel: string;
  selected: boolean;
  onSelect: () => void;
};

export function CommentCard({ comment, stateLabel, selected, onSelect }: CardProps) {
  const ref = useRef<HTMLLIElement>(null);
  const { context, target } = comment;

  useEffect(() => {
    if (selected) ref.current?.scrollIntoView({ block: "nearest", behavior: "smooth" });
  }, [selected]);

  return (
    <li
      className={cn(
        "flex flex-col gap-2 rounded-lg p-3 transition-colors",
        selected ? "bg-selected" : "bg-surface-2 hover:bg-selected"
      )}
      data-comment-id={comment.id}
      data-testid="comment-card"
      ref={ref}
    >
      <button
        aria-expanded={selected}
        className="flex min-w-0 flex-col gap-1 rounded-xs text-left outline-none focus-visible:shadow-focus-ring"
        onClick={onSelect}
        type="button"
      >
        <span className="flex items-center gap-2">
          <span
            className={cn(
              "grid size-5 shrink-0 place-items-center rounded-pill font-mono text-mono-id tabular-nums",
              comment.status === "open" ? "bg-accent text-accent-ink" : "bg-canvas text-muted"
            )}
          >
            {commentNumber(comment)}
          </span>
          <span className="min-w-0 flex-1 truncate text-meta text-muted">{stateLabel}</span>
          <span className="shrink-0 font-mono text-mono-id tracking-mono-id text-subtle">
            {context.theme ?? "dark"}
            {target.kind === "node" ? "" : ` · ${context.viewport.width}`}
          </span>
        </span>
        {target.kind === "node" ? null : (
          <span className="truncate text-meta text-muted">{targetSummary(target)}</span>
        )}
        {target.kind !== "node" && target.source ? (
          <span
            className="truncate font-mono text-mono-id tracking-mono-id text-subtle"
            title={target.source}
          >
            {target.source}
          </span>
        ) : null}
      </button>
      <ul className="flex flex-col gap-2 border-t border-line-soft pt-2">
        {comment.thread.map(message => (
          <li key={message.id}>
            <Message commentId={comment.id} message={message} slug={comment.slug} />
          </li>
        ))}
      </ul>
      {selected ? <CommentActions comment={comment} /> : null}
    </li>
  );
}
