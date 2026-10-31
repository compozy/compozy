import { CornerDownRight, Folder, ListPlus, Pencil, RotateCcw, Trash2, X } from "@compozy/ui/icons";
import type { ReactNode } from "react";

import { cn } from "@/lib/utils";
import { Button, OwnerAvatar, Skeleton, Spinner } from "@compozy/ui";

import {
  isQueuedPromptMutable,
  type QueuedPrompt,
  type QueuedPromptAttachmentSummary,
  type QueuedPromptOwner,
} from "../lib/queued-prompt";
import { queuedPromptPreview } from "../lib/queued-prompt-preview";
import type { UnconfirmedSend } from "../lib/session-unconfirmed-send";
import { useSessionRuntimeRenderContext } from "../hooks/use-session-runtime-render-context";
import type { SessionMessageOrigin } from "../lib/session-message-payload";
import { SubagentAvatar } from "./subagents/subagent-avatar";
import { DELETED_SESSION_LABEL, sessionPartyMark } from "./session-messages/session-message-party";
import { useSessionLabel } from "./session-messages/use-session-label";

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
        className="grid size-5 shrink-0 place-items-center overflow-hidden rounded-xs border border-line bg-card"
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

function SessionQueuedPreview({ text, className }: { text: string; className?: string }) {
  const preview = queuedPromptPreview(text);
  return (
    <span
      data-testid="composer-queued-preview"
      data-preview-kind={preview.kind}
      className={cn("min-w-0 flex-1 truncate text-transcript-body text-muted", className)}
      title={text}
    >
      {preview.kind === "code" ? (
        <span className="mr-1.5 rounded-xxs bg-surface-2 px-1 font-mono text-mono-id text-subtle">
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
  ...props
}: {
  children: ReactNode;
  className?: string;
} & Record<`data-${string}`, string | undefined>) {
  return (
    <div
      data-testid="composer-queued-prompt-row"
      className={cn(
        "flex min-h-8 min-w-0 items-center gap-2 py-transcript-meta-gap pr-transcript-inline-gap pl-3.5",
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

function SessionQueuedRemove({
  label,
  disabled,
  onRemove,
}: {
  label: string;
  disabled: boolean;
  onRemove: () => void;
}) {
  return (
    <Button
      type="button"
      variant="ghost"
      size="icon-xs"
      onClick={onRemove}
      disabled={disabled}
      data-testid="composer-queued-remove"
      aria-label={label}
      className="text-faint hover:text-fg"
    >
      <Trash2 aria-hidden="true" className="size-3" />
    </Button>
  );
}

function SessionQueuedSteer({
  prompt,
  disabled,
  label,
  onSteer,
}: {
  prompt: QueuedPrompt;
  disabled: boolean;
  label?: string;
  onSteer: (prompt: QueuedPrompt) => void;
}) {
  return (
    <Button
      type="button"
      variant="ghost"
      size="sm"
      onClick={() => onSteer(prompt)}
      disabled={disabled}
      data-testid="composer-queued-steer"
      aria-label={label}
      className="text-muted hover:text-fg"
    >
      <CornerDownRight aria-hidden="true" className="size-3" />
      Steer
    </Button>
  );
}

type SessionLabel = ReturnType<typeof useSessionLabel>;

/** "From {title}" with the sender's provider mark and, across workspaces, its workspace (Gap 5). */
function SessionQueuedSender({ party, title }: { party: SessionLabel; title: string }) {
  return (
    <span
      data-testid="composer-queued-sender"
      aria-busy={party.pending || undefined}
      title={party.pending ? undefined : `From ${title}`}
      className="inline-flex max-w-[70%] min-w-0 items-center gap-1.5 text-micro text-muted"
    >
      <SubagentAvatar provider={sessionPartyMark(party)} size="sm" className="size-4.5" />
      <span className="shrink-0">From</span>
      {party.pending ? (
        <Skeleton aria-hidden="true" className="inline-block h-3 w-16 rounded-xs" />
      ) : (
        <span
          className={cn(
            "min-w-0 truncate",
            party.title === null ? "font-medium text-muted" : "font-medium text-fg-2"
          )}
        >
          {title}
        </span>
      )}
      {party.workspaceName ? (
        <span
          className="inline-flex min-w-0 items-center gap-0.5 text-subtle"
          data-testid="composer-queued-sender-workspace"
        >
          <Folder aria-hidden="true" className="size-2.75 shrink-0 text-faint" />
          <span className="truncate">{party.workspaceName}</span>
        </span>
      ) : null}
    </span>
  );
}

/** Steer and Remove for a message from another session, named after its sender once known. */
function SessionQueuedSessionMessageActions({
  prompt,
  party,
  title,
  disabled,
  onSteer,
  onRemove,
}: {
  prompt: QueuedPrompt;
  party: SessionLabel;
  title: string;
  disabled: boolean;
  onSteer: (prompt: QueuedPrompt) => void;
  onRemove: (id: string) => void;
}) {
  const suffix = party.pending ? "" : ` from ${title}`;
  return (
    <div className="flex shrink-0 items-center gap-px">
      <SessionQueuedSteer
        prompt={prompt}
        disabled={disabled}
        label={`Steer message${suffix}`}
        onSteer={onSteer}
      />
      <SessionQueuedRemove
        label={`Remove message${suffix}`}
        disabled={disabled}
        onRemove={() => onRemove(prompt.id)}
      />
    </div>
  );
}

/**
 * A message another session sent (S4, VC-05): the sender's provider mark and
 * "From {title}" in the owner slot, the title kept whole before the preview
 * gives way so rows from different senders stay apart. Its text is
 * agent-authored: the operator may steer it into the running turn (promotion
 * changes when, never what) or remove it; edit stays refused
 * (409 `input_agent_authored`).
 */
function SessionQueuedSessionMessageRow({
  prompt,
  sender,
  disabled,
  actionsHidden,
  onSteer,
  onRemove,
}: {
  prompt: QueuedPrompt;
  sender: SessionMessageOrigin;
  disabled: boolean;
  actionsHidden: boolean;
  onSteer: (prompt: QueuedPrompt) => void;
  onRemove: (id: string) => void;
}) {
  // The viewing session's workspace: a sender in another one is named with its workspace (Gap 5).
  const viewWorkspaceId = useSessionRuntimeRenderContext()?.workspaceId || sender.workspaceId;
  const party = useSessionLabel({
    sessionId: sender.sessionId,
    workspaceId: sender.workspaceId,
    currentWorkspaceId: viewWorkspaceId,
    agentName: sender.agentName,
    titleAtSend: sender.titleAtSend,
  });
  const dispatching = prompt.status === "dispatching";
  const title = party.title ?? DELETED_SESSION_LABEL;
  const actionable = isQueuedPromptMutable(prompt) && !dispatching && !actionsHidden;
  return (
    <SessionQueueRowFrame data-status={prompt.status} data-origin="session">
      <SessionQueuePosition>#{prompt.position}</SessionQueuePosition>
      <ListPlus aria-hidden="true" className="size-3 shrink-0 text-faint" />
      <SessionQueuedSender party={party} title={title} />
      {prompt.attachments ? <SessionQueuedAttachmentWell summary={prompt.attachments} /> : null}
      <SessionQueuedPreview text={prompt.text} className="min-w-16 basis-0" />
      {dispatching ? (
        <SessionQueuedState spinning testId="composer-queued-state">
          Sending…
        </SessionQueuedState>
      ) : null}
      {actionable ? (
        <SessionQueuedSessionMessageActions
          prompt={prompt}
          party={party}
          title={title}
          disabled={disabled}
          onSteer={onSteer}
          onRemove={onRemove}
        />
      ) : null}
    </SessionQueueRowFrame>
  );
}

export interface SessionQueueEntryRowProps {
  prompt: QueuedPrompt;
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
  disabled,
  actionsHidden,
  onSteer,
  onEdit,
  onRemove,
}: SessionQueueEntryRowProps) {
  if (prompt.sender) {
    return (
      <SessionQueuedSessionMessageRow
        prompt={prompt}
        sender={prompt.sender}
        disabled={disabled}
        actionsHidden={actionsHidden}
        onSteer={onSteer}
        onRemove={onRemove}
      />
    );
  }
  const dispatching = prompt.status === "dispatching";
  const mutable =
    isQueuedPromptMutable(prompt) && prompt.owner === null && !dispatching && !actionsHidden;
  return (
    <SessionQueueRowFrame data-status={prompt.status} data-owner-kind={prompt.owner?.kind}>
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
      {mutable ? (
        <div className="flex shrink-0 items-center gap-px">
          <SessionQueuedSteer prompt={prompt} disabled={disabled} onSteer={onSteer} />
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
          <SessionQueuedRemove
            label="Remove from queue"
            disabled={disabled}
            onRemove={() => onRemove(prompt.id)}
          />
        </div>
      ) : null}
    </SessionQueueRowFrame>
  );
}

export interface SessionUnconfirmedRowProps {
  send: UnconfirmedSend;
  disabled: boolean;
  onRetry?: (id: string) => void;
  onDiscard?: (id: string) => void;
}

/**
 * The only client-local row: a send whose acknowledgment was lost. No position
 * (it has none yet), the retained message id beside "Not confirmed", and Retry
 * replaying the same identity. Discard drops
 * the local row only; the daemon is never asked to forget anything.
 */
export function SessionUnconfirmedRow({
  send,
  disabled,
  onRetry,
  onDiscard,
}: SessionUnconfirmedRowProps) {
  const retrying = send.phase === "retrying";
  return (
    <SessionQueueRowFrame data-local="unconfirmed" data-phase={send.phase}>
      <SessionQueuePosition>—</SessionQueuePosition>
      <ListPlus aria-hidden="true" className="size-3 shrink-0 text-faint" />
      <SessionQueuedPreview text={send.text} />
      {retrying ? (
        <SessionQueuedState spinning testId="composer-queued-state">
          Retrying…
        </SessionQueuedState>
      ) : (
        <SessionQueuedState testId="composer-queued-state">
          <span title={send.identity.messageId}>Not confirmed — retry or discard</span>
        </SessionQueuedState>
      )}
      {retrying ? null : (
        <div className="flex shrink-0 items-center gap-px">
          <Button
            type="button"
            variant="secondary"
            size="sm"
            onClick={() => onRetry?.(send.id)}
            disabled={disabled || !onRetry}
            data-testid="composer-queued-retry"
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
