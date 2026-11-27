import { useContext, type HTMLAttributes, type ReactElement, type ReactNode } from "react";

import type { ComputerId } from "@/lib/computer-scope";
import { cn } from "@/lib/utils";
import { useAgentIdentityView } from "@/systems/agent/hooks/use-agent-identity-view";
import type { ProfileOwner } from "@/systems/profiles/lib/profile-scope";

import { useKeyboardFocusToken } from "../../hooks/use-keyboard-focus-token";
import type { useSessionEntryPreload } from "../../hooks/use-session-entry-preload";
import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import type { SessionDragHandlers, SessionRowDropEdge } from "../../hooks/use-session-list-dnd";
import type { SessionRuntimeLabel } from "../../hooks/use-session-runtime-label";
import type { SessionRowSelection } from "../../hooks/use-session-selection";
import { sessionBadgeSignal } from "../../lib/session-badge";
import { getSessionDisplayTitle } from "../../lib/session-display-title";
import {
  lookupSessionComputer,
  SessionListComputerContext,
  type SessionRowComputerInfo,
} from "../../lib/session-list-computer";
import { SessionListBadgeContext } from "../../lib/session-list-row-badge";
import type { SessionRowLayoutMenu } from "../../lib/session-menu-tree";
import { sessionWaitingOn } from "../../lib/session-pull-request";
import { rowMeta } from "../../lib/session-row-meta";
import { sessionComputerOf, sessionRowKey } from "../../lib/session-row-ref";
import type { SessionPayload } from "../../types";
import { SessionLaterDetails } from "../session-later-details";
import { SessionListAgentMark } from "./session-list-agent-mark";
import { SessionListProviderMark } from "./session-list-provider-mark";
import { SessionRowDragFrame } from "./session-list-row-drag-frame";
import {
  SESSION_ROW_FRAME,
  SESSION_ROW_LATER_FRAME,
  SESSION_ROW_MARK_TOOLTIP_DELAY,
} from "./session-list-row-frame";
import {
  activateSessionRow,
  SessionRowBadgeMark,
  SessionRowCheckbox,
  SessionRowEndSlot,
  SessionRowMetaLine,
  SessionRowTarget,
  SessionRowTrailing,
} from "./session-list-row-parts";

export interface SessionListRowProps {
  session: SessionPayload;
  /** The computer owning this row; omitted defaults to the session's recorded provenance or local. */
  computerId?: ComputerId;
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
  trailing?: ReactNode;
  selection?: SessionRowSelection;
  /** Folder moves and manual-order verbs for a root row in the workspace list. */
  layoutMenu?: SessionRowLayoutMenu;
  /** Drag source/target wiring for a root row in the workspace list. */
  drag?: SessionDragHandlers;
  /** The pending drop lands on this edge of the row. */
  dropEdge?: SessionRowDropEdge | null;
  /** This row is the one being dragged. */
  dragging?: boolean;
  /** A child session or subagent below runs; defaults to the row's own live subagents (#1926). */
  descendantsRunning?: boolean;
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

function rowDataComputerId(
  computer: SessionRowComputerInfo | undefined,
  effectiveComputerId: ComputerId | undefined
): string | undefined {
  if (computer?.id) return computer.id;
  if (effectiveComputerId && effectiveComputerId !== "local") return effectiveComputerId;
  return undefined;
}

function runtimeModelLabel(runtime: SessionRuntimeLabel): string {
  return runtime.model || (runtime.next ? runtime.label : "");
}

function runtimeNextModelLabel(runtime: SessionRuntimeLabel): string {
  return runtime.next ? runtime.next.model || runtime.next.label : "";
}

/**
 * Two-line session row. Line 1 belongs to the title: only the end slot shares it,
 * holding the compact time at rest and the row actions on hover or focus. Line 2
 * carries where and who runs it (project, owner, the agent's face, the provider
 * mark) and one phrase: the model, or the needs-you words that replace it, then the
 * trailing cluster. The title button's `::after` stretches over the row so the whole
 * row navigates, while the checkbox, branch toggle, and actions sit above it as siblings.
 */
export function SessionListRowView(props: SessionListRowViewProps) {
  const { session, showActions = true, wrapRow, selection, computerId } = props;
  const effectiveComputerId = computerId ?? sessionComputerOf(session);
  const rowKey = sessionRowKey({ computerId: effectiveComputerId, sessionId: session.id });
  const selected = selection?.selectedIds.has(rowKey) ?? false;
  const computer = lookupSessionComputer(
    useContext(SessionListComputerContext),
    session.id,
    effectiveComputerId
  );
  const offline = computer?.offline === true;
  const canAct = showActions && !offline;
  const row = (
    <SessionRowDragFrame
      className={cn(
        SESSION_ROW_FRAME,
        "group/session-row transition-colors hover:bg-surface-2 has-[[data-slot=session-row-target]:focus-visible]:shadow-focus-ring",
        (selected || props.current) && "bg-selected",
        offline && "opacity-70"
      )}
      data-computer-id={rowDataComputerId(computer, effectiveComputerId)}
      data-computer-offline={offline ? "true" : undefined}
      data-slot="session-list-row"
      drag={offline ? undefined : props.drag}
      dropEdge={props.dropEdge ?? null}
      dragging={props.dragging ?? false}
      dropTestId={`${props.testIdPrefix}-drop-${props.dropEdge}-${session.id}`}
      onContextMenu={() => {
        if (selection?.mode && !selected) selection.selectOnly(rowKey);
      }}
    >
      <SessionListRowBody
        {...props}
        computer={computer}
        rowKey={rowKey}
        selected={selected}
        showActions={canAct}
      />
    </SessionRowDragFrame>
  );
  if (!canAct || !wrapRow) return row;
  return wrapRow(row);
}

interface SessionListRowBodyProps extends SessionListRowViewProps {
  rowKey: string;
  selected: boolean;
  computer: SessionRowComputerInfo | undefined;
}

function SessionListRowBody({
  session,
  rowKey,
  selected,
  computer,
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
  descendantsRunning,
}: SessionListRowBodyProps) {
  const title = getSessionDisplayTitle(session);
  const signal = sessionBadgeSignal(session.badge, sessionWaitingOn(session));
  const { keyboardFocus, focusHandlers } = useKeyboardFocusToken();
  const actionsVisible = showActions && !selection?.mode;
  const agent = useAgentIdentityView(session.agent_name, {
    workspace: session.workspace_id,
    profile: session.profile_name,
  });
  const meta = rowMeta({
    session,
    agentLabel: session.agent_name ? agent.label : "",
    runtime,
    signal,
    orphanedSubagent,
  });
  const badgeOf = useContext(SessionListBadgeContext);
  const badge = inheritsProject ? undefined : badgeOf?.(session);
  const activate = (event: React.MouseEvent<HTMLElement>) =>
    activateSessionRow(event, rowKey, selection, onSelect);
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
        badgeMark={
          <SessionRowBadgeMark
            session={session}
            selection={selection}
            descendantsRunning={descendantsRunning}
          />
        }
        testId={`${testIdPrefix}-session-${session.id}`}
        onClick={activate}
        focusHandlers={focusHandlers}
        preloadHandlers={preloadHandlers}
      />
      {selection ? (
        <SessionRowCheckbox
          title={title}
          sessionId={rowKey}
          selected={selected}
          selection={selection}
        />
      ) : null}
      <SessionRowEndSlot session={session} actions={actionsVisible ? (rowActions ?? null) : null} />
      <SessionRowMetaLine
        session={session}
        model={runtimeModelLabel(runtime)}
        nextModel={runtimeNextModelLabel(runtime)}
        badge={badge}
        computer={computer}
        owner={owner}
        meta={meta}
        mark={
          <>
            {session.agent_name ? (
              <SessionListAgentMark
                appearance={agent.appearance}
                label={agent.label}
                onClick={activate}
                sessionId={session.id}
              />
            ) : null}
            <SessionListProviderMark
              sessionId={session.id}
              runtime={runtime}
              delay={SESSION_ROW_MARK_TOOLTIP_DELAY}
              keyboardFocus={keyboardFocus}
              onClick={activate}
            />
          </>
        }
        trailing={<SessionRowTrailing>{trailing}</SessionRowTrailing>}
      />
      <SessionLaterDetails className={SESSION_ROW_LATER_FRAME} session={session} />
    </>
  );
}
