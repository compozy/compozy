import { Folder } from "lucide-react";
import type { ComponentProps, MouseEvent, ReactNode } from "react";

import { Button, Pill, Skeleton, cn } from "@compozy/ui";

import { MessageClampToggle } from "@/components/assistant-ui/message-clamp-toggle";
import {
  MESSAGE_CLAMPED_CLASS,
  useMessageClamp,
} from "@/components/assistant-ui/use-message-clamp";
import {
  formatMessageTimestamp,
  formatMessageTimestampFull,
} from "@/systems/session/lib/format-timestamp";

import {
  DELETED_SESSION_LABEL,
  SESSION_MESSAGE_FRAME_CLASS,
  type SessionMessageOpen,
  type SessionMessageParty,
} from "./session-message-party";

/**
 * "From {title}" / "Reply from {title}" / "Sent to {title}": the title is a
 * link to the session (Enter opens it, ⌘/Ctrl opens a new window); a deleted
 * session reads plainly with no link. A party in another workspace adds its
 * folder and name after the link.
 */
export function SessionMessagePartyLabel({
  verb,
  party,
  onOpen,
}: {
  verb: string;
  party: SessionMessageParty;
  onOpen?: SessionMessageOpen;
}) {
  const handleOpen = (event: MouseEvent<HTMLButtonElement>) => {
    onOpen?.(party, { newWindow: event.metaKey || event.ctrlKey });
  };
  return (
    <>
      <span
        className="inline-flex min-w-0 items-baseline gap-1 text-transcript-meta whitespace-nowrap text-subtle"
        data-slot="session-message-party"
      >
        <span className="shrink-0">{verb}</span>
        {party.pending ? (
          <Skeleton
            aria-hidden="true"
            className="inline-block h-3 w-20 self-center rounded-xs"
            data-testid="session-message-party-pending"
          />
        ) : party.title === null ? (
          <span className="font-medium text-muted" data-testid="session-message-party-deleted">
            {DELETED_SESSION_LABEL}
          </span>
        ) : onOpen ? (
          <Button
            type="button"
            variant="link"
            size="xs"
            onClick={handleOpen}
            className="h-auto min-w-0 truncate px-0 text-transcript-meta font-medium text-fg-strong"
            data-testid="session-message-party-link"
          >
            {party.title}
          </Button>
        ) : (
          <span className="min-w-0 truncate font-medium text-fg-strong">{party.title}</span>
        )}
      </span>
      {party.workspaceName ? (
        <span
          className="inline-flex shrink-0 items-center gap-0.5 text-transcript-caption whitespace-nowrap text-subtle"
          data-testid="session-message-party-workspace"
        >
          <Folder aria-hidden="true" className="size-2.75 text-faint" />
          {party.workspaceName}
        </span>
      ) : null}
    </>
  );
}

/** Neutral header chip: the mode (Steered · Interrupted · Superseded) or "Reply requested". */
export function SessionMessageChip({
  icon,
  children,
  subtle = false,
}: {
  icon: ReactNode;
  children: ReactNode;
  /** Superseded reads one ink step down. */
  subtle?: boolean;
}) {
  return (
    <Pill
      tone="neutral"
      size="xs"
      data-testid="session-message-chip"
      className={cn("[&>svg]:size-2.75 [&>svg]:text-subtle", subtle ? "text-subtle" : "text-muted")}
    >
      {icon}
      {children}
    </Pill>
  );
}

/** Header time, pushed right: `HH:MM` with the full timestamp on hover. */
export function SessionMessageTime({ timestampMs }: { timestampMs: number | null }) {
  if (timestampMs === null) return null;
  return (
    <time
      dateTime={new Date(timestampMs).toISOString()}
      title={formatMessageTimestampFull(timestampMs)}
      className="ml-auto shrink-0 pl-2 font-mono text-transcript-caption text-faint tabular-nums"
      data-testid="session-message-time"
    >
      {formatMessageTimestamp(timestampMs)}
    </time>
  );
}

export type SessionMessageBodyTone = "default" | "quiet" | "danger";

const BODY_TONE: Record<SessionMessageBodyTone, string> = {
  default: "text-fg",
  quiet: "text-subtle",
  danger: "text-danger",
};

export interface SessionMessageFrameProps extends Omit<ComponentProps<"article">, "children"> {
  /** 24px avatar column: the other session's provider mark. */
  avatar: ReactNode;
  /** Header content after the avatar: party label and chips. */
  header: ReactNode;
  timestampMs: number | null;
  /** Superseded: the body drops one ink step, never removed. */
  subdued?: boolean;
  bodyTone?: SessionMessageBodyTone;
  /** Below the body: attachments, the truncation note. */
  footer?: ReactNode;
  children: ReactNode;
}

/**
 * The shared frame of the session message card (S1) and the reply card (S2):
 * avatar · header (party, chips, time) · body with the operator bubble's
 * reading type and clamp. Agent-authored, so it sits on the left.
 */
export function SessionMessageFrame({
  avatar,
  header,
  timestampMs,
  subdued = false,
  bodyTone = "default",
  footer,
  children,
  className,
  ...props
}: SessionMessageFrameProps) {
  const { contentRef, clampable, clamped, expanded, toggle } = useMessageClamp();
  return (
    <article
      data-slot="session-message"
      data-subdued={subdued || undefined}
      className={cn(
        SESSION_MESSAGE_FRAME_CLASS,
        "grid grid-cols-[24px_minmax(0,1fr)] gap-x-2.5 gap-y-1 py-2.25 pr-3 pl-2.25",
        className
      )}
      {...props}
    >
      <span className="row-span-2 self-start">{avatar}</span>
      <div className="flex min-h-6 min-w-0 items-center gap-1.5">
        {header}
        <SessionMessageTime timestampMs={timestampMs} />
      </div>
      <div
        ref={contentRef}
        data-testid="session-message-body"
        data-clamped={clamped || undefined}
        className={cn(
          "col-start-2 min-w-0 text-transcript-message leading-relaxed [overflow-wrap:anywhere]",
          // The markdown inside sets its own ink: a superseded body lowers all of it.
          subdued ? "text-subtle [&_*]:text-subtle" : BODY_TONE[bodyTone],
          clamped ? MESSAGE_CLAMPED_CLASS : null
        )}
      >
        {children}
      </div>
      <MessageClampToggle
        clampable={clampable}
        expanded={expanded}
        onToggle={toggle}
        testId="session-message-clamp-toggle"
        className="col-start-2 -ml-1 justify-self-start"
      />
      {footer ? <div className="col-start-2 flex min-w-0 flex-col gap-1">{footer}</div> : null}
    </article>
  );
}
