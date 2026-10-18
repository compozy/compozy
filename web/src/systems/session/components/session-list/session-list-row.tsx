import { useRef, useState } from "react";

import { Checkbox, Time } from "@compozy/ui";
import type { SessionRowSelection } from "../../hooks/use-session-selection";

import { cn } from "@/lib/utils";

import { getSessionDisplayTitle } from "../../lib/session-display-title";
import { sessionBadgeSignal } from "../../lib/session-badge";
import { sessionBadgeWordClass } from "../../lib/session-badge-classes";
import { sessionAwaitsSubagents, sessionSubagentCounts } from "../../lib/session-subagent-summary";
import { maskedAttentionNote } from "../../lib/session-pending-interactions";
import { sessionAttentionReason } from "../../lib/session-attention-reason";
import { SessionBadgeMark } from "../session-badge-mark";
import type { SessionPayload } from "../../types";
import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import { SessionRowActions } from "../session-row-actions";
import { SessionRowContextMenu, type SessionRowLayoutMenu } from "../session-row-context-menu";
import type { SessionDragHandlers, SessionDropPosition } from "../../hooks/use-session-list-dnd";
import { SUBAGENT_OF_DELETED_SESSION } from "../subagents/subagent-format";
import { SessionListProviderMark } from "./session-list-provider-mark";
import { SessionListSubagentChip } from "./session-list-subagent-chip";
import { ProfileOwnerTag, type ProfileOwner } from "@/systems/profiles";

export interface SessionListRowProps {
  session: SessionPayload;
  /**
   * The row's profile, supplied only while the aggregate is on. A scoped list
   * already answers "whose work is this", so it stays tag-free (US-011.AC-1).
   */
  owner?: ProfileOwner;
  current?: boolean;
  onSelect: () => void;
  sessionActions: SessionLifecycleActionHandlers;
  showActions?: boolean;
  testIdPrefix: string;
  /** A subagent session whose ancestry was deleted: it leads its thread, labelled. */
  orphanedSubagent?: boolean;
  /** Trailing controls rendered beside the row actions (e.g. a thread toggle). */
  trailing?: React.ReactNode;
  selection?: SessionRowSelection;
  /** Folder moves and manual-order verbs for a root row in the workspace list. */
  layoutMenu?: SessionRowLayoutMenu;
  /** Drag source/target wiring for a root row in the workspace list. */
  drag?: SessionDragHandlers;
  /** The pending drop lands on this edge of the row. */
  dropEdge?: SessionDropPosition | null;
  /** This row is the one being dragged. */
  dragging?: boolean;
}

/** Share the navigation row with selection gestures without nesting interactive controls. */
export function SessionListRow(props: SessionListRowProps) {
  const { session, sessionActions, showActions = true, layoutMenu } = props;
  const row = (
    <SessionRowDragFrame
      drag={props.drag}
      dropEdge={props.dropEdge ?? null}
      dragging={props.dragging ?? false}
      dropTestId={`${props.testIdPrefix}-drop-${props.dropEdge}-${session.id}`}
    >
      <SessionListRowBody {...props} />
    </SessionRowDragFrame>
  );
  if (!showActions) return row;
  return (
    <SessionRowContextMenu
      session={session}
      actions={sessionActions}
      layout={layoutMenu}
      render={row}
    />
  );
}

function SessionListRowBody({
  session,
  owner,
  current = false,
  onSelect,
  sessionActions,
  showActions = true,
  testIdPrefix,
  trailing,
  selection,
  orphanedSubagent = false,
}: SessionListRowProps) {
  const selected = selection?.selectedIds.has(session.id) ?? false;
  const title = getSessionDisplayTitle(session);
  const subagentCounts = sessionSubagentCounts(session);
  // Each keyboard focus gets its own id; pointer focus reveals nothing.
  const focusCount = useRef(0);
  const [keyboardFocus, setKeyboardFocus] = useState<number | null>(null);
  return (
    <>
      <button
        type="button"
        className={cn(
          "relative grid min-w-0 grid-cols-[12px_minmax(0,1fr)_auto] items-start gap-x-2 rounded-md px-2 py-1.5 text-left transition-colors hover:bg-surface-2 focus-visible:shadow-focus-ring focus-visible:outline-none",
          (selected || current) && "bg-selected"
        )}
        data-status={session.badge}
        data-testid={`${testIdPrefix}-session-${session.id}`}
        aria-current={current ? "true" : undefined}
        // While a selection is open, activating the row toggles its membership,
        // so the row reads as a toggle button; otherwise it only navigates.
        aria-pressed={selection?.mode ? selected : undefined}
        data-selected={selected || undefined}
        onClick={event => activateSessionRow(event, session.id, selection, onSelect)}
        onFocus={event => {
          focusCount.current += 1;
          setKeyboardFocus(
            event.currentTarget.matches(":focus-visible") ? focusCount.current : null
          );
        }}
        onBlur={() => setKeyboardFocus(null)}
      >
        <SessionBadgeMark
          badge={session.badge}
          delegated={sessionAwaitsSubagents(session)}
          className={cn(
            "mt-1",
            selection &&
              "transition-opacity duration-fast motion-reduce:transition-none group-hover/session-row:opacity-0 group-focus-within/session-row:opacity-0",
            selection?.mode && "opacity-0"
          )}
        />
        <SessionListRowDetails
          session={session}
          current={current}
          orphanedSubagent={orphanedSubagent}
          keyboardFocus={keyboardFocus}
        />
        <span className="col-start-3 row-start-1 mt-0.5 flex items-center gap-1.5">
          {owner ? <ProfileOwnerTag compact owner={owner} /> : null}
          <Time iso={session.updated_at} className="text-eyebrow text-subtle" />
        </span>
      </button>
      {selection ? (
        <SessionRowCheckbox
          title={title}
          sessionId={session.id}
          selected={selected}
          selection={selection}
        />
      ) : null}
      <SessionRowTrailing
        session={session}
        subagentCounts={subagentCounts}
        trailing={trailing}
        actions={showActions && !selection?.mode ? sessionActions : null}
        onSelect={onSelect}
      />
    </>
  );
}

function activateSessionRow(
  event: React.MouseEvent<HTMLButtonElement>,
  sessionId: string,
  selection: SessionRowSelection | undefined,
  onSelect: () => void
) {
  if (selection && event.shiftKey) selection.toggleRange(sessionId);
  else if (selection && (selection.mode || event.metaKey || event.ctrlKey))
    selection.toggle(sessionId);
  else onSelect();
}

function SessionRowCheckbox({
  title,
  sessionId,
  selected,
  selection,
}: {
  title: string;
  sessionId: string;
  selected: boolean;
  selection: SessionRowSelection;
}) {
  return (
    <Checkbox
      aria-label={`Select ${title}`}
      checked={selected}
      className={cn(
        "absolute top-2 left-1 z-10 transition-opacity duration-fast motion-reduce:transition-none after:inset-0",
        selection.mode
          ? "opacity-100"
          : "pointer-events-none opacity-0 group-hover/session-row:pointer-events-auto group-hover/session-row:opacity-100 group-focus-within/session-row:pointer-events-auto group-focus-within/session-row:opacity-100"
      )}
      onCheckedChange={(_checked, details) => {
        if ("shiftKey" in details.event && details.event.shiftKey) selection.toggleRange(sessionId);
        else selection.toggle(sessionId);
      }}
    />
  );
}

function SessionRowTrailing({
  session,
  subagentCounts,
  trailing,
  actions,
  onSelect,
}: {
  session: SessionPayload;
  subagentCounts: ReturnType<typeof sessionSubagentCounts>;
  trailing: React.ReactNode;
  actions: SessionListRowProps["sessionActions"] | null;
  onSelect: () => void;
}) {
  return (
    <div className="flex items-center gap-0.5 pt-1">
      {subagentCounts !== null ? (
        <SessionListSubagentChip session={session} counts={subagentCounts} onSelect={onSelect} />
      ) : null}
      {trailing}
      {actions !== null ? <SessionRowActions session={session} actions={actions} /> : null}
    </div>
  );
}

/** The drag source/target frame; it draws the drop line on the pending edge. */
function SessionRowDragFrame({
  drag,
  dropEdge,
  dragging,
  dropTestId,
  children,
  className,
  ...rest
}: React.ComponentProps<"div"> & {
  drag: SessionDragHandlers | undefined;
  dropEdge: SessionDropPosition | null;
  dragging: boolean;
  dropTestId: string;
}) {
  return (
    <div
      {...rest}
      {...drag}
      data-dragging={dragging || undefined}
      className={cn(
        "group/session-row relative grid grid-cols-[minmax(0,1fr)_auto] items-start gap-1",
        dragging && "opacity-50",
        className
      )}
    >
      {dropEdge !== null ? <SessionRowDropLine edge={dropEdge} testId={dropTestId} /> : null}
      {children}
    </div>
  );
}

/** The drop indicator on the edge where a dragged session will land. */
function SessionRowDropLine({ edge, testId }: { edge: SessionDropPosition; testId: string }) {
  return (
    <span
      aria-hidden="true"
      data-testid={testId}
      className={cn(
        "pointer-events-none absolute inset-x-1 z-10 h-0.5 rounded-full bg-accent",
        edge === "before" ? "-top-px" : "-bottom-px"
      )}
    />
  );
}

/**
 * Keep attention and lifecycle words visible when the selection checkbox covers the badge.
 * Two grid items: the title sits beside the time, and the sub line runs under the time
 * column so the provider mark costs it no attention word.
 */
function SessionListRowDetails({
  session,
  current,
  orphanedSubagent,
  keyboardFocus,
}: {
  session: SessionPayload;
  current: boolean;
  orphanedSubagent: boolean;
  keyboardFocus: number | null;
}) {
  const signal = sessionBadgeSignal(session.badge);
  const maskedNote = maskedAttentionNote(session, signal.label);
  const attentionReason = sessionAttentionReason(session, signal.label);
  return (
    <>
      <span
        className={cn(
          "col-start-2 row-start-1 block min-w-0 truncate text-body",
          // A needs-you row keeps its pull even when the window is not focused.
          signal.attention === "needs-you" ? "font-medium text-fg" : "text-fg",
          current && "font-medium"
        )}
      >
        {getSessionDisplayTitle(session)}
      </span>
      <span className="col-span-2 col-start-2 row-start-2 flex min-w-0 items-center gap-1 text-eyebrow text-subtle">
        <SessionListProviderMark session={session} keyboardFocus={keyboardFocus} />
        <span className="min-w-0 truncate">
          <span className="font-medium text-muted">{session.agent_name}</span>
          {/* The mark carries the state; the word speaks up whenever the glyph is shared. */}
          {signal.speaks ? (
            <>
              <span aria-hidden="true"> · </span>
              <span className={sessionBadgeWordClass(session.badge)}>{signal.displayLabel}</span>
            </>
          ) : null}
          {attentionReason !== null ? (
            <>
              <span aria-hidden="true"> · </span>
              <span data-testid="session-attention-reason" title={attentionReason.detail}>
                {attentionReason.summary}
              </span>
            </>
          ) : null}
          {maskedNote !== null ? (
            <>
              <span aria-hidden="true"> · </span>
              {maskedNote}
            </>
          ) : null}
          {orphanedSubagent ? (
            <>
              <span aria-hidden="true"> · </span>
              {SUBAGENT_OF_DELETED_SESSION}
            </>
          ) : null}
          {session.archived_at !== null ? (
            <>
              <span aria-hidden="true"> · </span>
              Archived
            </>
          ) : null}
        </span>
      </span>
    </>
  );
}
