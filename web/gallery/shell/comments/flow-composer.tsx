/**
 * New comment on a flow, inside the comments panel: about the selected step, or the whole flow
 * when no step is selected. Same body, paste and image rules as the prototype composer.
 */
import { useState } from "react";

import { Button, Kbd, KbdGroup, Textarea } from "@compozy/ui";

import { AttachmentInput } from "#gallery/shell/comments/attachments";
import { maxBodyLength, type NodeTarget } from "#gallery/shell/comments/model";
import { useAttachments } from "#gallery/shell/comments/use-attachments";

export function FlowComposer({
  slug,
  target,
  onSubmit,
}: {
  slug: string;
  target: NodeTarget;
  onSubmit: (body: string, attachments: string[]) => Promise<void>;
}) {
  const uploads = useAttachments(slug);
  const [body, setBody] = useState("");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const canSubmit = body.trim() !== "" && !pending && uploads.ready;

  const submit = async () => {
    if (!canSubmit) return;
    setPending(true);
    setError(null);
    try {
      await onSubmit(body, uploads.ids);
      uploads.committed();
      setBody("");
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason));
    }
    setPending(false);
  };

  return (
    <form
      className="flex flex-col gap-2 rounded-lg bg-surface-2 p-3"
      data-testid="flow-comment-composer"
      onSubmit={event => {
        event.preventDefault();
        void submit();
      }}
    >
      <p className="truncate text-small-body text-muted">
        {target.node ? (
          <>
            Comment on step <span className="text-fg">{target.label}</span>
          </>
        ) : (
          "Comment on the whole flow (select a step to comment on it)"
        )}
      </p>
      <Textarea
        aria-label="New comment"
        className="min-h-16"
        maxLength={maxBodyLength}
        onChange={event => setBody(event.target.value)}
        onKeyDown={event => {
          if (event.key === "Enter" && (event.metaKey || event.ctrlKey)) {
            event.preventDefault();
            void submit();
          }
        }}
        onPaste={event => {
          if (event.clipboardData.files.length === 0) return;
          event.preventDefault();
          uploads.add([...event.clipboardData.files]);
        }}
        placeholder="What should change in this flow?"
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
        <Button data-testid="flow-comment-submit" disabled={!canSubmit} size="sm" type="submit">
          Comment
        </Button>
      </div>
    </form>
  );
}
