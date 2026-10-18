import { Checkbox, Time } from "@compozy/ui";
import type { SessionRowSelection } from "../../hooks/use-session-selection";

import { cn } from "@/lib/utils";

import { getSessionDisplayTitle } from "../../lib/session-display-title";
import { sessionBadgeSignal } from "../../lib/session-badge";
import { sessionAwaitsSubagents, sessionSubagentCounts } from "../../lib/session-subagent-summary";
import { useSessionRuntimeLabel } from "../../hooks/use-session-runtime-label";
import { SessionBadgeMark } from "../session-badge-mark";
import type { SessionPayload } from "../../types";
import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import { SessionRowActions } from "../session-row-actions";
import { SessionRowContextMenu, type SessionRowLayoutMenu } from "../session-row-context-menu";
import type { SessionDragHandlers, SessionDropPosition } from "../../hooks/use-session-list-dnd";
import { SessionListProviderMark } from "./session-list-provider-mark";
import { SessionListRowWho } from "./session-list-row-meta";
import { rowMeta, type RowMeta } from "../../lib/session-row-meta";
import { useKeyboardFocusToken } from "../../hooks/use-keyboard-focus-token";
import { SessionListSubagentChip } from "./session-list-subagent-chip";
import { ProfileOwnerTag, type ProfileOwner } from "@/systems/profiles";

export interface SessionListRowProps {
  session: SessionPayload;
  /**
   * The row's profile, supplied only while the aggregate is on and only where it
   * adds information: a scoped list already answers "whose work is this", and a
   * child in its root's profile repeats it (US-011.AC-1).
   */
  owner?: ProfileOwner;
  /**
   * The thread root of a child row. A child running the same agent on the same
   * provider and model inherits the root's meta line and renders as a single line.
   */
  parent?: SessionPayload;
  current?: boolean;
  onSelect: () => void;
  sessionActions: SessionLifecycleActionHandlers;
  showActions?: boolean;
  testIdPrefix: string;
  /** A subagent session whose ancestry was deleted: it leads its thread, labelled. */
  orphanedSubagent?: boolean;
  /** Controls rendered in the meta line's trailing cluster (e.g. a thread toggle). */
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

/**
 * Two-line session row. Line 1 belongs to the title: only the end slot shares it,
 * holding the compact time at rest and the row actions on hover or focus. Line 2
 * carries who runs it (owner, provider, agent), attention words, and the trailing
 * cluster. The title button's `::after` stretches over the row so the whole row
 * navigates, while the checkbox, chip, toggle, and actions sit above it as
 * siblings — no interactive control nests inside another.
 */
export function SessionListRow(props: SessionListRowProps) {
  const { session, sessionActions, showActions = true, layoutMenu } = props;
  const selected = props.selection?.selectedIds.has(session.id) ?? false;
  const row = (
    <SessionRowDragFrame
      className={cn(
        "group/session-row relative grid grid-cols-[12px_minmax(0,1fr)_auto] items-center gap-x-2 gap-y-0.75 rounded-md px-2 py-1.5 transition-colors hover:bg-surface-2 has-[[data-slot=session-row-target]:focus-visible]:shadow-focus-ring",
        (selected || props.current) && "bg-selected"
      )}
      data-slot="session-list-row"
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
  parent,
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
  const runtime = useSessionRuntimeLabel(session.runtime?.effective);
  const signal = sessionBadgeSignal(session.badge);
  const { keyboardFocus, focusHandlers } = useKeyboardFocusToken();
  const actionsVisible = showActions && !selection?.mode;
  const meta = rowMeta({ session, parent, owner, runtime, signal, orphanedSubagent });
  const hasTrailing = subagentCounts !== null || trailing !== undefined;
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
      />
      {selection ? (
        <SessionRowCheckbox
          title={title}
          sessionId={session.id}
          selected={selected}
          selection={selection}
        />
      ) : null}
      <SessionRowEndSlot session={session} actions={actionsVisible ? sessionActions : null} />
      {meta.visible || hasTrailing ? (
        <SessionRowMetaLine
          session={session}
          owner={owner}
          meta={meta}
          mark={
            meta.inherits ? null : (
              <SessionListProviderMark
                sessionId={session.id}
                runtime={runtime}
                keyboardFocus={keyboardFocus}
                onClick={event => activateSessionRow(event, session.id, selection, onSelect)}
              />
            )
          }
          trailing={
            hasTrailing ? (
              <SessionRowTrailing
                session={session}
                subagentCounts={subagentCounts}
                trailing={trailing}
                onSelect={onSelect}
              />
            ) : null
          }
        />
      ) : null}
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
}) {
  return (
    <button
      type="button"
      className={cn(
        "col-span-2 col-start-1 row-start-1 grid min-w-0 grid-cols-[12px_minmax(0,1fr)] items-center gap-x-2 text-left text-body text-fg outline-none after:absolute after:inset-0 after:rounded-md",
        emphasized && "font-medium"
      )}
      data-slot="session-row-target"
      // The meta line sits under the target, so the attention detail rides on the row.
      title={meta.reason?.detail}
      data-status={session.badge}
      data-testid={testId}
      aria-current={current ? "true" : undefined}
      // While a selection is open, activating the row toggles its membership,
      // so the row reads as a toggle button; otherwise it only navigates.
      aria-pressed={selection?.mode ? selected : undefined}
      data-selected={selected || undefined}
      onClick={onClick}
      {...focusHandlers}
    >
      <SessionBadgeMark
        badge={session.badge}
        delegated={sessionAwaitsSubagents(session)}
        className={badgeMarkClass(selection)}
      />
      <span className="min-w-0 truncate">{title}</span>
      {/* The meta line is drawn outside the target; its words still name the row. */}
      <span className="sr-only">{meta.spoken}</span>
    </button>
  );
}

/** Line 2: who runs the row (owner, provider, agent), its words, and the trailing cluster. */
function SessionRowMetaLine({
  session,
  owner,
  meta,
  mark,
  trailing,
}: {
  session: SessionPayload;
  owner: ProfileOwner | undefined;
  meta: RowMeta;
  mark: React.ReactNode;
  trailing: React.ReactNode;
}) {
  return (
    <span className="col-span-2 col-start-2 row-start-2 flex min-w-0 items-center gap-2">
      <span className="flex min-w-0 flex-1 items-center gap-1.25 overflow-hidden text-eyebrow whitespace-nowrap text-subtle">
        {owner ? <ProfileOwnerTag compact size="xs" owner={owner} /> : null}
        {mark}
        <SessionListRowWho session={session} meta={meta} />
      </span>
      {trailing}
    </span>
  );
}

/** Line 1's end slot: the compact time at rest, the row actions on hover or focus. */
function SessionRowEndSlot({
  session,
  actions,
}: {
  session: SessionPayload;
  actions: SessionLifecycleActionHandlers | null;
}) {
  return (
    <span className="col-start-3 row-start-1 grid min-w-6 items-center justify-items-end *:col-start-1 *:row-start-1">
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
          <SessionRowActions session={session} actions={actions} />
        </span>
      ) : null}
    </span>
  );
}

/** The meta line's trailing cluster, lifted above the row target. */
function SessionRowTrailing({
  session,
  subagentCounts,
  trailing,
  onSelect,
}: {
  session: SessionPayload;
  subagentCounts: ReturnType<typeof sessionSubagentCounts>;
  trailing: React.ReactNode;
  onSelect: () => void;
}) {
  return (
    <span className="relative z-10 -mr-0.5 flex shrink-0 items-center gap-0.5">
      {subagentCounts !== null ? (
        <SessionListSubagentChip session={session} counts={subagentCounts} onSelect={onSelect} />
      ) : null}
      {trailing}
    </span>
  );
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
      className={cn(dragging && "opacity-50", className)}
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
