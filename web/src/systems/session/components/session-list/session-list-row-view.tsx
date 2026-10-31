import { useContext } from "react";

import { Checkbox, Time, formatAbsoluteTime } from "@compozy/ui";
import type { SessionRowSelection } from "../../hooks/use-session-selection";

import { cn } from "@/lib/utils";

import { getSessionDisplayTitle } from "../../lib/session-display-title";
import { sessionBadgeSignal } from "../../lib/session-badge";
import { sessionWaitingOn } from "../../lib/session-pull-request";
import type { SessionRuntimeLabel } from "../../hooks/use-session-runtime-label";
import { SessionBadgeMark } from "../session-badge-mark";
import type { SessionPayload } from "../../types";
import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import type { HTMLAttributes, ReactElement, ReactNode } from "react";
import type { SessionRowLayoutMenu } from "../session-row-context-menu";
import type {
  SessionDragHandlers,
  SessionDropPosition,
  SessionRowDropEdge,
} from "../../hooks/use-session-list-dnd";
import { SessionListProviderMark } from "./session-list-provider-mark";
import {
  SESSION_ROW_END_SLOT_FRAME,
  SESSION_ROW_FRAME,
  SESSION_ROW_LATER_FRAME,
  SESSION_ROW_META_FRAME,
  SESSION_ROW_TARGET_FRAME,
  SESSION_ROW_TRAILING_FRAME,
  SESSION_ROW_WHO_FRAME,
} from "./session-list-row-frame";
import { SessionListRowWho } from "./session-list-row-meta";
import { SessionUnrestrictedBadge } from "../session-unrestricted-badge";
import { SessionLaterDetails } from "../session-later-details";
import { sessionIsUnrestricted } from "../../lib/session-unrestricted";
import { rowMeta, type RowMeta } from "../../lib/session-row-meta";
import { useKeyboardFocusToken } from "../../hooks/use-keyboard-focus-token";
import type { useSessionEntryPreload } from "../../hooks/use-session-entry-preload";
import { ProfileOwnerTag } from "@/systems/profiles/components/profile-owner-tag";
import type { ProfileOwner } from "@/systems/profiles/lib/profile-scope";
import { WorkspaceBadge } from "@/systems/workspace/components/workspace-badge";
import type { WorkspaceBadgeIdentity } from "@/systems/workspace/lib/workspace-badge-identity";
import { SessionListBadgeContext } from "../../lib/session-list-row-badge";
import {
  sessionCheckboxGesture,
  sessionSelectionGesture,
} from "../../lib/session-selection-gesture";

export interface SessionListRowProps {
  session: SessionPayload;
  /**
   * The row's profile, supplied only while the aggregate is on and only where it
   * adds information: a scoped list already answers "whose work is this", and a
   * child in its root's profile repeats it (US-011.AC-1).
   */
  owner?: ProfileOwner;
  /** A child in its root's project repeats nothing: the project badge is omitted. */
  inheritsProject?: boolean;
  current?: boolean;
  onSelect: () => void;
  sessionActions: SessionLifecycleActionHandlers;
  showActions?: boolean;
  testIdPrefix: string;
  /** A subagent session whose ancestry was deleted: it leads its thread, labelled. */
  orphanedSubagent?: boolean;
  /** Controls rendered in the meta line's trailing cluster (the branch toggle). */
  trailing?: React.ReactNode;
  selection?: SessionRowSelection;
  /** Folder moves and manual-order verbs for a root row in the workspace list. */
  layoutMenu?: SessionRowLayoutMenu;
  /** Drag source/target wiring for a root row in the workspace list. */
  drag?: SessionDragHandlers;
  /** The pending drop lands on this edge of the row. */
  dropEdge?: SessionRowDropEdge | null;
  /** This row is the one being dragged. */
  dragging?: boolean;
}

export interface SessionListRowViewProps extends SessionListRowProps {
  /** The live runtime label the provider mark and meta line read. */
  runtime: SessionRuntimeLabel;
  /** Session-entry intent preload on the row target. */
  preloadHandlers: ReturnType<typeof useSessionEntryPreload>;
  /** The row actions (`…` menu), revealed in line 1's end slot on hover or focus. */
  rowActions?: ReactNode;
  /** Wraps the row in its right-click menu. */
  wrapRow?: (row: ReactElement<HTMLAttributes<HTMLElement>>) => ReactNode;
}

/**
 * Two-line session row. Line 1 belongs to the title: only the end slot shares it,
 * holding the compact time at rest and the row actions on hover or focus. Line 2
 * carries who runs it (owner, provider, agent), attention words, and the trailing
 * cluster. The title button's `::after` stretches over the row so the whole row
 * navigates, while the checkbox, branch toggle, and actions sit above it as
 * siblings — no interactive control nests inside another.
 * `SessionListRow` binds the live runtime label and entry preload to this view.
 */
export function SessionListRowView(props: SessionListRowViewProps) {
  const { session, showActions = true, wrapRow, selection } = props;
  const selected = selection?.selectedIds.has(session.id) ?? false;
  const row = (
    <SessionRowDragFrame
      className={cn(
        SESSION_ROW_FRAME,
        "group/session-row transition-colors hover:bg-surface-2 has-[[data-slot=session-row-target]:focus-visible]:shadow-focus-ring",
        (selected || props.current) && "bg-selected"
      )}
      data-slot="session-list-row"
      drag={props.drag}
      dropEdge={props.dropEdge ?? null}
      dragging={props.dragging ?? false}
      dropTestId={`${props.testIdPrefix}-drop-${props.dropEdge}-${session.id}`}
      onContextMenu={() => {
        // Finder's rule: a context click outside the selection makes that row the selection.
        if (selection?.mode && !selected) selection.selectOnly(session.id);
      }}
    >
      <SessionListRowBody {...props} />
    </SessionRowDragFrame>
  );
  if (!showActions || !wrapRow) return row;
  return wrapRow(row);
}

function SessionListRowBody({
  session,
  owner,
  inheritsProject = false,
  current = false,
  onSelect,
  showActions = true,
  testIdPrefix,
  trailing,
  selection,
  orphanedSubagent = false,
  runtime,
  preloadHandlers,
  rowActions,
}: SessionListRowViewProps) {
  const selected = selection?.selectedIds.has(session.id) ?? false;
  const title = getSessionDisplayTitle(session);
  const signal = sessionBadgeSignal(session.badge, sessionWaitingOn(session));
  const { keyboardFocus, focusHandlers } = useKeyboardFocusToken();
  const actionsVisible = showActions && !selection?.mode;
  const meta = rowMeta({ session, runtime, signal, orphanedSubagent });
  const badgeOf = useContext(SessionListBadgeContext);
  const badge = inheritsProject ? undefined : badgeOf?.(session);
  const hasTrailing = trailing !== undefined;
  return (
    <>
      <SessionRowTarget
        session={session}
        title={title}
        meta={meta}
        emphasized={signal.attention === "needs-you" || current}
        current={current}
        selected={selected}
        selection={selection}
        testId={`${testIdPrefix}-session-${session.id}`}
        onClick={event => activateSessionRow(event, session.id, selection, onSelect)}
        focusHandlers={focusHandlers}
        preloadHandlers={preloadHandlers}
      />
      {selection ? (
        <SessionRowCheckbox
          title={title}
          sessionId={session.id}
          selected={selected}
          selection={selection}
        />
      ) : null}
      <SessionRowEndSlot session={session} actions={actionsVisible ? (rowActions ?? null) : null} />
      {meta.visible || hasTrailing ? (
        <SessionRowMetaLine
          session={session}
          badge={badge}
          owner={owner}
          meta={meta}
          mark={
            <SessionListProviderMark
              sessionId={session.id}
              runtime={runtime}
              keyboardFocus={keyboardFocus}
              onClick={event => activateSessionRow(event, session.id, selection, onSelect)}
            />
          }
          trailing={hasTrailing ? <SessionRowTrailing>{trailing}</SessionRowTrailing> : null}
        />
      ) : null}
      <SessionLaterDetails className={SESSION_ROW_LATER_FRAME} session={session} />
    </>
  );
}

/** Line 1's title target; its `::after` stretches over the row so the whole row navigates. */
function SessionRowTarget({
  session,
  title,
  meta,
  emphasized,
  current,
  selected,
  selection,
  testId,
  onClick,
  focusHandlers,
  preloadHandlers,
}: {
  session: SessionPayload;
  title: string;
  meta: RowMeta;
  emphasized: boolean;
  current: boolean;
  selected: boolean;
  selection: SessionRowSelection | undefined;
  testId: string;
  onClick: (event: React.MouseEvent<HTMLElement>) => void;
  focusHandlers: ReturnType<typeof useKeyboardFocusToken>["focusHandlers"];
  preloadHandlers: ReturnType<typeof useSessionEntryPreload>;
}) {
  return (
    <button
      type="button"
      className={cn(
        SESSION_ROW_TARGET_FRAME,
        "outline-none after:absolute after:inset-0 after:rounded-md",
        emphasized && "font-medium"
      )}
      data-slot="session-row-target"
      // The end slot swaps the time for actions on hover, and the meta line sits
      // under the target, so the full timestamp and attention detail ride on the row.
      title={[formatAbsoluteTime(session.updated_at), meta.reason?.detail]
        .filter(Boolean)
        .join("\n")}
      data-status={session.badge}
      data-testid={testId}
      aria-current={current ? "true" : undefined}
      // While a selection is open, activating the row toggles its membership,
      // so the row reads as a toggle button; otherwise it only navigates.
      aria-pressed={selection?.mode ? selected : undefined}
      data-selected={selected || undefined}
      onClick={onClick}
      {...preloadHandlers}
      onFocus={event => {
        focusHandlers.onFocus(event);
        preloadHandlers.onFocus();
      }}
      onBlur={() => {
        focusHandlers.onBlur();
        preloadHandlers.onBlur();
      }}
    >
      <SessionBadgeMark
        badge={session.badge}
        waitingOn={sessionWaitingOn(session)}
        className={badgeMarkClass(selection)}
      />
      <span className="min-w-0 truncate">{title}</span>
      {/* The meta line is drawn outside the target; its words still name the row. */}
      <span className="sr-only">{meta.spoken}</span>
    </button>
  );
}

/**
 * Line 2: where and who (project badge, owner, provider, agent), its words, and
 * the trailing cluster. The project name gives way before the agent.
 */
function SessionRowMetaLine({
  session,
  badge,
  owner,
  meta,
  mark,
  trailing,
}: {
  session: SessionPayload;
  badge: WorkspaceBadgeIdentity | undefined;
  owner: ProfileOwner | undefined;
  meta: RowMeta;
  mark: React.ReactNode;
  trailing: React.ReactNode;
}) {
  return (
    <span className={SESSION_ROW_META_FRAME}>
      <span className={SESSION_ROW_WHO_FRAME}>
        {badge ? (
          <WorkspaceBadge
            identity={badge}
            className="max-w-40 shrink-[1000]"
            data-testid={`session-row-project-${session.id}`}
          />
        ) : null}
        {owner ? <ProfileOwnerTag compact size="xs" owner={owner} /> : null}
        {mark}
        <SessionListRowWho session={session} meta={meta} />
        {sessionIsUnrestricted(session) ? (
          <SessionUnrestrictedBadge data-testid={`session-row-unrestricted-${session.id}`} />
        ) : null}
      </span>
      {trailing}
    </span>
  );
}

/** Line 1's end slot: the compact time at rest, the row actions on hover or focus. */
function SessionRowEndSlot({ session, actions }: { session: SessionPayload; actions: ReactNode }) {
  return (
    <span className={SESSION_ROW_END_SLOT_FRAME}>
      <Time
        iso={session.updated_at}
        mode="compact"
        className={cn(
          "font-mono text-micro text-subtle transition-opacity duration-fast motion-reduce:transition-none",
          actions !== null &&
            "group-focus-within/session-row:opacity-0 group-hover/session-row:opacity-0 group-has-data-popup-open/session-row:opacity-0"
        )}
      />
      {actions !== null ? (
        <span className="relative z-10 -my-1 opacity-0 transition-opacity duration-fast group-focus-within/session-row:opacity-100 group-hover/session-row:opacity-100 group-has-data-popup-open/session-row:opacity-100 motion-reduce:transition-none">
          {actions}
        </span>
      ) : null}
    </span>
  );
}

/** The meta line's trailing cluster, lifted above the row target. */
function SessionRowTrailing({ children }: { children: React.ReactNode }) {
  return <span className={SESSION_ROW_TRAILING_FRAME}>{children}</span>;
}

/** With a selection available, the badge gives way to the checkbox on hover or focus. */
function badgeMarkClass(selection: SessionRowSelection | undefined): string | undefined {
  if (!selection) return undefined;
  return cn(
    "transition-opacity duration-fast motion-reduce:transition-none group-hover/session-row:opacity-0 group-focus-within/session-row:opacity-0",
    selection.mode && "opacity-0"
  );
}

function activateSessionRow(
  event: React.MouseEvent<HTMLElement>,
  sessionId: string,
  selection: SessionRowSelection | undefined,
  onSelect: () => void
) {
  const gesture = selection ? sessionSelectionGesture(event, selection.mode) : "open";
  if (gesture === "range") selection?.toggleRange(sessionId);
  else if (gesture === "toggle") selection?.toggle(sessionId);
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
        if (sessionCheckboxGesture(details.event) === "range") selection.toggleRange(sessionId);
        else selection.toggle(sessionId);
      }}
    />
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
  dropEdge: SessionRowDropEdge | null;
  dragging: boolean;
  dropTestId: string;
}) {
  return (
    <div
      {...rest}
      {...drag}
      data-dragging={dragging || undefined}
      data-drop={dropEdge ?? undefined}
      className={cn(
        dragging && "opacity-50",
        // Dropping onto the row nests the dragged session under it.
        dropEdge === "into" && "rounded-md bg-accent-tint ring-1 ring-accent ring-inset",
        className
      )}
    >
      {dropEdge === "before" || dropEdge === "after" ? (
        <SessionRowDropLine edge={dropEdge} testId={dropTestId} />
      ) : null}

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
