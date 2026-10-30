/** One message of a thread; editing stays local until Save succeeds. */
import { useState } from "react";

import { Button, Textarea, Time } from "@compozy/ui";

import { commentsApi } from "#gallery/shell/comments/api";
import { AttachmentImages, AttachmentInput } from "#gallery/shell/comments/attachments";
import { maxAttachments, type CommentMessage } from "#gallery/shell/comments/model";
import { useAttachments } from "#gallery/shell/comments/use-attachments";

const authors = { reviewer: "Reviewer", agent: "Agent" };

type MessageProps = { slug: string; commentId: string; message: CommentMessage };

function MessageEditor({
  slug,
  commentId,
  message,
  onDone,
}: MessageProps & { onDone: () => void }) {
  const uploads = useAttachments(slug);
  // The draft starts from the saved body until the reviewer types.
  const [draft, setDraft] = useState<string | null>(null);
  const body = draft ?? message.body;
  const [removed, setRemoved] = useState<readonly string[]>([]);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const removedIds = new Set(removed);
  const images = message.attachments.filter(image => !removedIds.has(image.id));
  const capacity = maxAttachments - images.length;

  const cancel = () => {
    uploads.cancel();
    onDone();
  };
  const save = () => {
    if (!uploads.ready || !body.trim()) return;
    setPending(true);
    setError(null);
    commentsApi
      .edit(slug, commentId, message.id, {
        body,
        addAttachments: uploads.ids,
        removeAttachments: [...removed],
      })
      .then(
        () => {
          uploads.committed();
          onDone();
        },
        (reason: unknown) => {
          setError(reason instanceof Error ? reason.message : String(reason));
          setPending(false);
        }
      );
  };

  return (
    <form
      className="flex flex-col gap-2"
      onKeyDown={event => {
        if (event.key === "Escape" && !pending) {
          event.preventDefault();
          cancel();
        }
      }}
      onSubmit={event => {
        event.preventDefault();
        save();
      }}
    >
      <Textarea
        aria-label="Edit message"
        disabled={pending}
        maxLength={10_000}
        onChange={event => setDraft(event.target.value)}
        onPaste={event => {
          if (event.clipboardData.files.length === 0) return;
          event.preventDefault();
          uploads.add([...event.clipboardData.files], capacity);
        }}
        value={body}
      />
      {images.length > 0 ? (
        <ul className="flex flex-wrap gap-1.5">
          {images.map(image => (
            <li key={image.id}>
              <Button
                disabled={pending}
                onClick={() => setRemoved(previous => [...previous, image.id])}
                size="xs"
                type="button"
                variant="ghost"
              >
                Remove {image.name}
              </Button>
            </li>
          ))}
        </ul>
      ) : null}
      <AttachmentInput capacity={capacity} disabled={pending} uploads={uploads} />
      {error ? (
        <p className="text-meta text-danger" role="alert">
          Could not save: {error}
        </p>
      ) : null}
      <div className="flex gap-1.5">
        <Button disabled={pending || !body.trim() || !uploads.ready} size="sm" type="submit">
          Save
        </Button>
        <Button disabled={pending} onClick={cancel} size="sm" type="button" variant="ghost">
          Cancel
        </Button>
      </div>
    </form>
  );
}

export function Message(props: MessageProps) {
  const { message } = props;
  const [editing, setEditing] = useState(false);
  return (
    <div className="flex flex-col gap-1" data-testid="comment-message">
      <span className="flex items-center gap-1.5 text-meta text-subtle">
        <span className="font-medium text-fg">{authors[message.author]}</span>
        <Time iso={message.at} mode="relative" />
        {message.editedAt ? <span>· edited</span> : null}
        {editing ? null : (
          <Button
            className="ml-auto"
            onClick={() => setEditing(true)}
            size="xs"
            type="button"
            variant="quiet"
          >
            Edit
          </Button>
        )}
      </span>
      {editing ? (
        <MessageEditor {...props} onDone={() => setEditing(false)} />
      ) : (
        <>
          <p className="text-small-body whitespace-pre-wrap text-fg">{message.body}</p>
          <AttachmentImages images={message.attachments} />
        </>
      )}
    </div>
  );
}
