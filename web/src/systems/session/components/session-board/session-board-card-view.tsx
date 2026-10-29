import {
  CornerDownRight,
  GitBranch,
  GitMerge,
  GitPullRequest,
  GitPullRequestClosed,
  GitPullRequestDraft,
  ListTree,
  type LucideIcon,
} from "lucide-react";
import type { ComponentProps } from "react";

import {
  formatRelativeTime,
  Pill,
  StateGlyph,
  Time,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@compozy/ui";

import { cn } from "@/lib/utils";

import type { SessionRuntimeLabel } from "../../hooks/use-session-runtime-label";
import {
  sessionBoardCardMeta,
  sessionBoardCardRollup,
  sessionBoardCardStatus,
  sessionBoardCardUnread,
  sessionBoardCardWaiting,
  type SessionBoardCardMeta,
  type SessionBoardCardRollup,
  type SessionBoardCardStatus,
  type SessionBoardPrShape,
} from "../../lib/session-board-card";
import type { SessionBoardPayload } from "../../lib/session-board-contract";
import { getSessionDisplayTitle, UNTITLED_SESSION_TITLE } from "../../lib/session-display-title";
import { SessionListProviderMark } from "../session-list/session-list-provider-mark";
import {
  SESSION_BOARD_CARD_FOOTER_ROW,
  SESSION_BOARD_CARD_FRAME,
  SESSION_BOARD_CARD_META_ROW,
  SESSION_BOARD_CARD_TITLE_ROW,
  SESSION_BOARD_CARD_TITLE_TYPE,
} from "./sessions-board-frame";

const SHAPE_ICON: Record<SessionBoardPrShape | "branch", LucideIcon> = {
  open: GitPullRequest,
  draft: GitPullRequestDraft,
  merged: GitMerge,
  closed: GitPullRequestClosed,
  branch: GitBranch,
};

type CardIntentHandlers = Pick<
  ComponentProps<"button">,
  "onPointerEnter" | "onPointerLeave" | "onFocus" | "onBlur" | "onTouchStart"
>;

export interface SessionBoardCardViewProps extends CardIntentHandlers {
  session: SessionBoardPayload;
  /** All projects: the workspace name, shown in the meta row when the card has no PR. */
  workspaceName?: string | null;
  current?: boolean;
  archived?: boolean;
  /** The provider mark and model label shown beside the agent name. */
  runtime: SessionRuntimeLabel;
  onOpen: (session: SessionBoardPayload) => void;
}

/**
 * The presentational board card (`_uiux.md` S5): meta + one status, title, then
 * owner, lineage and time, as one button that opens the session. It takes the
 * runtime label and intent-preload handlers as props, so it renders without the
 * query layer; `SessionBoardCard` binds them for the live board.
 */
export function SessionBoardCardView({
  session,
  workspaceName = null,
  current = false,
  archived = false,
  runtime,
  onOpen,
  ...intentHandlers
}: SessionBoardCardViewProps) {
  const title = getSessionDisplayTitle(session);
  const untitled = title === UNTITLED_SESSION_TITLE;
  const status = sessionBoardCardStatus(session);
  const meta = sessionBoardCardMeta(session, workspaceName);
  const waiting = sessionBoardCardWaiting(session);
  const rollup = sessionBoardCardRollup(session);
  const parentTitle = session.parent_title?.trim();
  const label = [
    title,
    status?.label,
    waiting?.word,
    parentTitle ? `Child of ${parentTitle}` : null,
    rollup ? `${rollup.total} child ${rollup.total === 1 ? "session" : "sessions"}` : null,
    rollup?.needsYou ? "A child needs you" : null,
    formatRelativeTime(session.updated_at),
  ]
    .filter(Boolean)
    .join(", ");
  return (
    <button
      aria-current={current ? "true" : undefined}
      aria-label={label}
      className={cn(
        SESSION_BOARD_CARD_FRAME,
        "cursor-pointer text-left transition-colors duration-base ease-out outline-none hover:bg-surface-2 focus-visible:shadow-focus-ring",
        current && "bg-selected shadow-inset-strong",
        archived && "bg-transparent shadow-hairline-inset"
      )}
      data-archived={archived ? "true" : undefined}
      data-board-column={session.board_column}
      data-testid={`session-board-card-${session.id}`}
      onClick={() => onOpen(session)}
      {...intentHandlers}
      type="button"
    >
      <SessionBoardCardMetaRow meta={meta} sessionId={session.id} status={status} />
      <span className={SESSION_BOARD_CARD_TITLE_ROW}>
        {sessionBoardCardUnread(session) ? (
          <span
            aria-hidden="true"
            className="mt-1.5 size-1.5 shrink-0 rounded-full bg-fg-strong"
            data-testid={`session-board-card-unread-${session.id}`}
          />
        ) : null}
        <span
          className={cn(
            "line-clamp-2 min-w-0",
            SESSION_BOARD_CARD_TITLE_TYPE,
            untitled && "font-normal text-subtle",
            archived && "text-muted"
          )}
        >
          {title}
        </span>
      </span>
      <SessionBoardCardFooter
        rollup={rollup}
        runtime={runtime}
        session={session}
        waiting={waiting}
      />
    </button>
  );
}

function SessionBoardCardMetaRow({
  sessionId,
  meta,
  status,
}: {
  sessionId: string;
  meta: SessionBoardCardMeta;
  status: SessionBoardCardStatus | null;
}) {
  if (meta.shape === null && meta.workspaceName === null && status === null) return null;
  const Icon = meta.shape ? SHAPE_ICON[meta.shape] : null;
  return (
    <span className={SESSION_BOARD_CARD_META_ROW}>
      <span
        className="flex min-w-0 flex-1 items-center gap-1.25 font-mono text-mono-id tracking-mono-id whitespace-nowrap text-faint tabular-nums"
        data-testid={`session-board-card-meta-${sessionId}`}
      >
        {meta.workspaceName !== null ? (
          <span className="min-w-0 truncate font-sans text-eyebrow tracking-eyebrow text-muted">
            {meta.workspaceName}
          </span>
        ) : null}
        {Icon ? <Icon aria-hidden="true" className="size-3 shrink-0 text-subtle" /> : null}
        {meta.branch ? (
          <span className="min-w-0 truncate" title={meta.branch}>
            {meta.branch}
          </span>
        ) : null}
        {meta.number !== null ? (
          <>
            {meta.branch ? <span aria-hidden="true">·</span> : null}
            <span className="shrink-0 text-fg-2">#{meta.number}</span>
          </>
        ) : null}
      </span>
      {status ? (
        <Pill
          className="ml-auto shrink-0"
          data-testid={`session-board-card-status-${sessionId}`}
          form="plain"
          tone={status.tone}
        >
          <Pill.Dot />
          {status.label}
        </Pill>
      ) : null}
    </span>
  );
}

function SessionBoardCardFooter({
  session,
  runtime,
  rollup,
  waiting,
}: {
  session: SessionBoardPayload;
  runtime: SessionRuntimeLabel;
  rollup: SessionBoardCardRollup | null;
  waiting: ReturnType<typeof sessionBoardCardWaiting>;
}) {
  const parentTitle = session.parent_title?.trim();
  return (
    <span className={SESSION_BOARD_CARD_FOOTER_ROW}>
      <span className="flex max-w-28 min-w-0 shrink-0 items-center gap-1.25 overflow-hidden">
        <SessionListProviderMark runtime={runtime} sessionId={session.id} />
        <span className="min-w-0 truncate font-medium text-muted">{session.agent_name}</span>
      </span>
      {rollup ? <SessionBoardCardRollupMark rollup={rollup} sessionId={session.id} /> : null}
      {parentTitle ? (
        <span
          className="flex min-w-0 flex-auto items-center gap-1 overflow-hidden"
          data-testid={`session-board-card-parent-${session.id}`}
        >
          <CornerDownRight aria-hidden="true" className="size-3 shrink-0" />
          <span className="min-w-0 truncate">{parentTitle}</span>
        </span>
      ) : null}
      {waiting ? (
        <span
          className="flex min-w-0 items-center gap-1 overflow-hidden whitespace-nowrap text-muted"
          data-testid={`session-board-card-waiting-${session.id}`}
        >
          <StateGlyph size="sm" state={waiting.glyph} />
          {waiting.word}
        </span>
      ) : null}
      <Time
        className="ml-auto shrink-0 font-mono text-mono-id tracking-mono-id text-faint tabular-nums"
        iso={session.updated_at}
        mode="compact"
      />
    </span>
  );
}

function SessionBoardCardRollupMark({
  sessionId,
  rollup,
}: {
  sessionId: string;
  rollup: SessionBoardCardRollup;
}) {
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <span
            className="relative z-10 inline-flex shrink-0 items-center gap-1 text-muted tabular-nums"
            data-needs-you={rollup.needsYou ? "true" : undefined}
            data-testid={`session-board-card-children-${sessionId}`}
          />
        }
      >
        <ListTree aria-hidden="true" className="size-3 text-subtle" />
        {rollup.total}
        {rollup.needsYou ? <Pill.Dot className="ml-px" tone="accent" /> : null}
      </TooltipTrigger>
      <TooltipContent className="flex-col items-stretch gap-0">
        <div>
          {rollup.total} child {rollup.total === 1 ? "session" : "sessions"}
        </div>
        <div className="mt-1.5 grid gap-0.5">
          {rollup.breakdown.map(row => (
            <div className="flex justify-between gap-4 whitespace-nowrap" key={row.label}>
              <span className="font-normal text-subtle">{row.label}</span>
              <span className="tabular-nums">{row.count}</span>
            </div>
          ))}
        </div>
      </TooltipContent>
    </Tooltip>
  );
}
