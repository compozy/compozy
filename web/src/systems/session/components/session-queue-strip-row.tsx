import { CornerDownRight, ListPlus, Pencil, RotateCcw, Trash2, X } from "lucide-react";
import type { ReactNode } from "react";

import { cn } from "@/lib/utils";
import { Button, MonoId, OwnerAvatar, Spinner } from "@compozy/ui";

import {
  isQueuedPromptMutable,
  type QueuedPrompt,
  type QueuedPromptAttachmentSummary,
  type QueuedPromptOwner,
} from "../lib/queued-prompt";
import { queuedPromptPreview } from "../lib/queued-prompt-preview";
import type { UnconfirmedSend } from "../lib/session-unconfirmed-send";

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

/** Who parked the row when it was not the operator: avatar tier by kind, then the name. */
function SessionQueuedOwner({ owner }: { owner: QueuedPromptOwner }) {
  const label = owner.id ?? owner.kind;
  const tier = owner.kind === "agent" ? "agent" : owner.kind === "human" ? "human" : "system";
  return (
    <span
      data-testid="composer-queued-owner"
      data-owner-kind={owner.kind}
      className="inline-flex shrink-0 items-center gap-1.5 text-micro text-muted"
    >
      <OwnerAvatar ownerKind={tier} ownerId={label} name={label} size="sm" />
      {label}
    </span>
  );
}

function SessionQueuedPreview({ text }: { text: string }) {
  const preview = queuedPromptPreview(text);
  return (
    <span
      data-testid="composer-queued-preview"
      data-preview-kind={preview.kind}
      className="min-w-0 flex-1 truncate text-transcript-body text-muted"
      title={text}
    >
      {preview.kind === "code" ? (
        <span className="mr-1.5 rounded-xxs bg-badge-fill px-1 font-mono text-mono-id text-subtle">
          code
        </span>
      ) : null}
      {preview.text}
    </span>
  );
}

function SessionQueuedState({
  children,
  spinning = false,
  testId,
}: {
  children: ReactNode;
  spinning?: boolean;
  testId: string;
}) {
  return (
    <span
      data-testid={testId}
      className="inline-flex shrink-0 items-center gap-1.5 text-micro text-subtle tabular-nums"
    >
      {spinning ? <Spinner className="size-3" /> : null}
      {children}
    </span>
  );
}

function SessionQueueRowFrame({
  children,
  className,
  first,
  ...props
}: {
  children: ReactNode;
  className?: string;
  first: boolean;
} & Record<`data-${string}`, string | undefined>) {
  return (
    <div
      data-testid="composer-queued-prompt-row"
      className={cn(
        "flex min-h-8 min-w-0 items-center gap-2 py-transcript-meta-gap pr-transcript-inline-gap pl-3",
        !first && "border-t border-line-soft",
        className
      )}
      {...props}
    >
      {children}
    </div>
  );
}

function SessionQueuePosition({ children }: { children: ReactNode }) {
  return (
    <span
      data-testid="composer-queued-position"
      className="min-w-3.5 shrink-0 font-mono text-mono-id text-faint tabular-nums"
    >
      {children}
    </span>
  );
}

export interface SessionQueueEntryRowProps {
  prompt: QueuedPrompt;
  first: boolean;
  /** Row verbs are suspended (a mutation or send is in flight). */
  disabled: boolean;
  /** The verbs are absent while another row is being edited or the queue is being cleared. */
  actionsHidden: boolean;
  onSteer: (prompt: QueuedPrompt) => void;
  /** Absent when the surface cannot offer editing: the verb is not rendered. */
  onEdit?: (prompt: QueuedPrompt) => void;
  onRemove: (id: string) => void;
}

/**
 * One durable queue entry: position · glyph · owner (other actors only) ·
 * preview · state or verbs. Verbs exist only for the operator's own `queued`
 * rows — a dispatching row and another actor's row show state, never disabled
 * controls the runtime would not honor.
 */
export function SessionQueueEntryRow({
  prompt,
  first,
  disabled,
  actionsHidden,
  onSteer,
  onEdit,
  onRemove,
}: SessionQueueEntryRowProps) {
  const dispatching = prompt.status === "dispatching";
  const mutable = isQueuedPromptMutable(prompt) && prompt.owner === null;
  return (
    <SessionQueueRowFrame
      first={first}
      data-status={prompt.status}
      data-owner-kind={prompt.owner?.kind}
    >
      <SessionQueuePosition>#{prompt.position}</SessionQueuePosition>
      <ListPlus aria-hidden="true" className="size-3 shrink-0 text-faint" />
      {prompt.owner ? <SessionQueuedOwner owner={prompt.owner} /> : null}
      {prompt.attachments ? <SessionQueuedAttachmentWell summary={prompt.attachments} /> : null}
      <SessionQueuedPreview text={prompt.text} />
      {dispatching ? (
        <SessionQueuedState spinning testId="composer-queued-state">
          Sending…
        </SessionQueuedState>
      ) : null}
      {mutable && !dispatching && !actionsHidden ? (
        <div className="flex shrink-0 items-center gap-px">
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={() => onSteer(prompt)}
            disabled={disabled || Boolean(prompt.attachments)}
            data-testid="composer-queued-steer"
            title={
              prompt.attachments
                ? "Queued messages with files can't be steered on this agent"
                : undefined
            }
            className="text-muted hover:text-fg-strong"
          >
            <CornerDownRight aria-hidden="true" className="size-3" />
            Steer
          </Button>
          {onEdit ? (
            <Button
              type="button"
              variant="ghost"
              size="icon-xs"
              onClick={() => onEdit(prompt)}
              disabled={disabled}
              data-testid="composer-queued-edit"
              aria-label="Edit queued message"
              className="text-faint hover:text-fg"
            >
              <Pencil aria-hidden="true" className="size-3" />
            </Button>
          ) : null}
          <Button
            type="button"
            variant="ghost"
            size="icon-xs"
            onClick={() => onRemove(prompt.id)}
            disabled={disabled}
            data-testid="composer-queued-remove"
            aria-label="Remove from queue"
            className="text-faint hover:text-fg"
          >
            <Trash2 aria-hidden="true" className="size-3" />
          </Button>
        </div>
      ) : null}
    </SessionQueueRowFrame>
  );
}

export interface SessionUnconfirmedRowProps {
  send: UnconfirmedSend;
  first: boolean;
  disabled: boolean;
  onRetry?: (id: string) => void;
  onDiscard?: (id: string) => void;
}

/**
 * The only client-local row: a send whose acknowledgment was lost. No position
 * (it has none yet), the retained message id beside "Not confirmed", and Retry
 * — the one action-colored control — replaying the same identity. Discard drops
 * the local row only; the daemon is never asked to forget anything.
 */
export function SessionUnconfirmedRow({
  send,
  first,
  disabled,
  onRetry,
  onDiscard,
}: SessionUnconfirmedRowProps) {
  const retrying = send.phase === "retrying";
  return (
    <SessionQueueRowFrame first={first} data-local="unconfirmed" data-phase={send.phase}>
      <SessionQueuePosition>—</SessionQueuePosition>
      <ListPlus aria-hidden="true" className="size-3 shrink-0 text-faint" />
      <SessionQueuedPreview text={send.text} />
      {retrying ? (
        <SessionQueuedState spinning testId="composer-queued-state">
          Retrying…
        </SessionQueuedState>
      ) : (
        <SessionQueuedState testId="composer-queued-state">
          Not confirmed
          <MonoId value={send.identity.messageId} className="text-faint" />
        </SessionQueuedState>
      )}
      {retrying ? null : (
        <div className="flex shrink-0 items-center gap-px">
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={() => onRetry?.(send.id)}
            disabled={disabled || !onRetry}
            data-testid="composer-queued-retry"
            className="text-accent-strong hover:bg-accent-tint hover:text-accent-strong"
          >
            <RotateCcw aria-hidden="true" className="size-3" />
            Retry
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="icon-xs"
            onClick={() => onDiscard?.(send.id)}
            disabled={disabled || !onDiscard}
            data-testid="composer-queued-discard"
            aria-label="Discard"
            className="text-faint hover:text-fg"
          >
            <X aria-hidden="true" className="size-3" />
          </Button>
        </div>
      )}
    </SessionQueueRowFrame>
  );
}
