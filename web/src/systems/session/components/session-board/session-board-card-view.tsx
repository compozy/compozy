import {
  ArrowRightLeft,
  CornerDownRight,
  GitBranch,
  GitFork,
  GitMerge,
  GitPullRequest,
  GitPullRequestClosed,
  GitPullRequestDraft,
  ListTree,
  type LucideIcon,
  PencilLine,
} from "lucide-react";
import type { ComponentProps, MouseEvent } from "react";

import {
  formatAbsoluteTime,
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
  sessionBoardCardLineage,
  sessionBoardCardMeta,
  sessionBoardCardRollup,
  sessionBoardCardStatus,
  sessionBoardCardUnread,
  sessionBoardCardWaiting,
  type SessionBoardCardLineage,
  type SessionBoardCardMeta,
  type SessionBoardCardRollup,
  type SessionBoardCardStatus,
  type SessionBoardPrShape,
} from "../../lib/session-board-card";
import type { SessionBoardPayload } from "../../lib/session-board-contract";
import { getSessionDisplayTitle, UNTITLED_SESSION_TITLE } from "../../lib/session-display-title";
import { sessionLaterDetails, sessionLaterSpoken } from "../../lib/session-later";
import { SessionLaterDetails } from "../session-later-details";
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

/** A child nests under its parent; a fork or continuation names its source with the origin pill's glyph. */
const LINEAGE_ICON: Record<SessionBoardCardLineage["kind"], LucideIcon> = {
  child: CornerDownRight,
  fork: GitFork,
  continue: ArrowRightLeft,
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
  /** The composer holds unsent text for this session (#1052). */
  draft?: boolean;
  onOpen: (session: SessionBoardPayload) => void;
  /**
   * The board selection (`SessionBoardCard`): the card can be selected, so the footer
   * keeps a checkbox's place where the provider mark sits; `mode` while any card is.
   */
  selection?: { selected: boolean; mode: boolean };
  /** Replaces the plain open: a selection gesture, or `onOpen` when the click is not one. */
  onActivate?: (event: MouseEvent<HTMLButtonElement>) => void;
}

/** The card's accessible name: the title, then every fact its rows show, in reading order. */
function sessionBoardCardLabel({
  session,
  lineage,
  title,
  status,
  waiting,
  rollup,
  draft,
}: {
  session: SessionBoardPayload;
  lineage: SessionBoardCardLineage | null;
  title: string;
  status: SessionBoardCardStatus | null;
  waiting: ReturnType<typeof sessionBoardCardWaiting>;
  rollup: SessionBoardCardRollup | null;
  draft: boolean;
}): string {
  return [
    title,
    status?.label,
    waiting?.word,
    lineage ? `${lineage.verb} ${lineage.title}` : null,
    rollup ? `${rollup.total} child ${rollup.total === 1 ? "session" : "sessions"}` : null,
    rollup?.needsYou ? "A child needs you" : null,
    draft ? "Unsent draft" : null,
    sessionLaterSpoken(sessionLaterDetails(session), formatAbsoluteTime),
    formatRelativeTime(session.updated_at),
  ]
    .filter(Boolean)
    .join(", ");
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
  draft = false,
  onOpen,
  selection,
  onActivate,
  ...intentHandlers
}: SessionBoardCardViewProps) {
  const selected = selection?.selected === true;
  const title = getSessionDisplayTitle(session);
  const untitled = title === UNTITLED_SESSION_TITLE;
  const status = sessionBoardCardStatus(session);
  const meta = sessionBoardCardMeta(session, workspaceName);
  const waiting = sessionBoardCardWaiting(session);
  const rollup = sessionBoardCardRollup(session);
  const lineage = sessionBoardCardLineage(session);
  const label = sessionBoardCardLabel({ session, lineage, title, status, waiting, rollup, draft });
  return (
    <button
      aria-current={current ? "true" : undefined}
      aria-label={label}
      // While a selection is open, activating the card toggles its membership,
      // so the card reads as a toggle button; otherwise it only opens.
      aria-pressed={selection?.mode ? selected : undefined}
      className={cn(
        SESSION_BOARD_CARD_FRAME,
        "cursor-pointer text-left transition-colors duration-base ease-out outline-none hover:bg-surface-2 focus-visible:shadow-focus-ring",
        current && "bg-selected shadow-inset-strong",
        archived && "bg-transparent shadow-hairline-inset",
        selected && "bg-selected shadow-inset-strong hover:bg-selected"
      )}
      data-archived={archived ? "true" : undefined}
      data-agent-name={session.agent_name}
      data-board-column={session.board_column}
      data-selected={selected ? "true" : undefined}
      data-session-id={session.id}
      data-testid={`session-board-card-${session.id}`}
      data-workspace-id={session.workspace_id ?? ""}
      onClick={event => (onActivate ? onActivate(event) : onOpen(session))}
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
      <SessionLaterDetails session={session} />
      <SessionBoardCardFooter
        draft={draft}
        lineage={lineage}
        rollup={rollup}
        runtime={runtime}
        selection={selection}
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

/** `↳ Parent` for a child; `⑂ Source` for a fork, which is top-level and only names its origin. */
function SessionBoardCardLineageMark({
  lineage,
  sessionId,
}: {
  lineage: SessionBoardCardLineage;
  sessionId: string;
}) {
  const Icon = LINEAGE_ICON[lineage.kind];
  return (
    <span
      className="flex min-w-0 flex-auto items-center gap-1 overflow-hidden"
      data-lineage={lineage.kind}
      data-testid={`session-board-card-parent-${sessionId}`}
      title={`${lineage.verb} ${lineage.title}`}
    >
      <Icon aria-hidden="true" className="size-3 shrink-0" />
      <span className="min-w-0 truncate">{lineage.title}</span>
    </span>
  );
}

function SessionBoardCardFooter({
  session,
  lineage,
  runtime,
  rollup,
  waiting,
  selection,
  draft,
}: {
  session: SessionBoardPayload;
  lineage: SessionBoardCardLineage | null;
  draft: boolean;
  runtime: SessionRuntimeLabel;
  rollup: SessionBoardCardRollup | null;
  waiting: ReturnType<typeof sessionBoardCardWaiting>;
  selection: SessionBoardCardViewProps["selection"];
}) {
  const mark = <SessionListProviderMark runtime={runtime} sessionId={session.id} />;
  return (
    <span className={SESSION_BOARD_CARD_FOOTER_ROW}>
      <span className="flex max-w-28 min-w-0 shrink-0 items-center gap-1.25 overflow-hidden">
        {selection ? (
          // The checkbox's place (`SESSION_BOARD_CARD_CHECKBOX`): the mark gives way to it on
          // hover or focus, and while a selection is open, as a list row's badge does.
          <span
            className={cn(
              "grid size-4 shrink-0 place-items-center transition-opacity duration-fast group-focus-within/session-card:opacity-0 group-hover/session-card:opacity-0 motion-reduce:transition-none",
              selection.mode && "opacity-0"
            )}
            data-testid={`session-board-card-mark-slot-${session.id}`}
          >
            {mark}
          </span>
        ) : (
          mark
        )}
        <span className="min-w-0 truncate font-medium text-muted">{session.agent_name}</span>
      </span>
      {rollup ? <SessionBoardCardRollupMark rollup={rollup} sessionId={session.id} /> : null}
      {lineage ? <SessionBoardCardLineageMark lineage={lineage} sessionId={session.id} /> : null}
      {waiting ? (
        <span
          className="flex min-w-0 items-center gap-1 overflow-hidden whitespace-nowrap text-muted"
          data-testid={`session-board-card-waiting-${session.id}`}
        >
          <StateGlyph size="sm" state={waiting.glyph} />
          {waiting.word}
        </span>
      ) : null}
      {draft ? (
        <span
          className="flex shrink-0 items-center gap-1 text-muted"
          data-testid={`session-board-card-draft-${session.id}`}
        >
          <PencilLine aria-hidden="true" className="size-3 text-subtle" />
          Draft
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
