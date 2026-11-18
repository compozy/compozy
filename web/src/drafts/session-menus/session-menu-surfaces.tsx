/**
 * The two hosts of the grouped menu, built on the shipped list row, folder, board view and card
 * view. Only the menus are new: every row and card offers the same tree on right-click and `…`.
 */
import { Trash2, X } from "@compozy/ui/icons";
import { createContext, use, useMemo, useState, type ReactNode } from "react";

import { Button, Checkbox, TopbarOverflowIcon } from "@compozy/ui";

import { cn } from "@/lib/utils";
import type { SessionBoardCardProps } from "@/systems/session/components/session-board/session-board-card";
import { SessionBoardCardSlot } from "@/systems/session/components/session-board/session-board-card-slot";
import { SessionBoardCardView } from "@/systems/session/components/session-board/session-board-card-view";
import { SESSION_BOARD_CARD_CHECKBOX } from "@/systems/session/components/session-board/sessions-board-frame";
import { SessionsBoardView } from "@/systems/session/components/session-board/sessions-board-view";
import { SessionListFolder } from "@/systems/session/components/session-list/session-list-folder";
import { SessionListGroupToggle } from "@/systems/session/components/session-list/session-list-group-toggle";
import { SessionListRowView } from "@/systems/session/components/session-list/session-list-row-view";
import { SESSION_LIST_BAR_CLASS } from "@/systems/session/lib/session-list-bar-class";
import { SESSION_SIDEBAR_DEFAULT_WIDTH } from "@/systems/session/lib/session-sidebar-width";
import {
  boardRuntimeFixture,
  sessionsBoardViewFixture,
} from "@/systems/session/mocks/sessions-board-view-fixtures";
import {
  SESSION_ENTRY_PRELOAD_FIXTURE,
  sessionLifecycleActionsFixture,
} from "@/systems/session/mocks/session-row-fixtures";
import type { SessionPayload } from "@/systems/session/types";
import { WorkspaceBadge } from "@/systems/workspace/components/workspace-badge";

import {
  SELECTION_FACTS,
  LIST_FOLDER_IDS,
  LIST_ROOT_IDS,
  boardFacts,
  boardSession,
  listFacts,
} from "./session-menus-fixtures";
import { selectionMenuTree, sessionMenuTree } from "./session-menu-model";
import { SessionContextMenu, SessionDropdownMenu } from "./session-menu";

/** Which menu a gallery state shows open: one session's `…`, or the selection's More actions. */
export interface PinnedMenu {
  sessionId?: string;
  selection?: boolean;
  openSubs?: readonly string[];
}

const actions = sessionLifecycleActionsFixture();

function overflowTrigger(label: string) {
  return (
    <Button aria-label={label} size="icon-sm" type="button" variant="ghost">
      <TopbarOverflowIcon aria-hidden="true" />
    </Button>
  );
}

function selectionOf(selected: readonly string[]) {
  return {
    selectedIds: new Set(selected),
    mode: true,
    toggle: () => undefined,
    toggleRange: () => undefined,
    selectOnly: () => undefined,
  };
}

/** The selection bar (`SessionListSelectionBar`) with the grouped tree behind More actions. */
export function SelectionBar({
  sessions,
  pinned,
  className,
}: {
  sessions: readonly SessionPayload[];
  pinned: PinnedMenu;
  className?: string;
}) {
  // The bar names the count already: it renders the selection group's items without its label.
  const nodes = selectionMenuTree(sessions, SELECTION_FACTS).flatMap(node =>
    node.kind === "group" ? node.children : [node]
  );
  return (
    <div
      aria-label="Selected sessions"
      className={cn(SESSION_LIST_BAR_CLASS, className)}
      role="toolbar"
    >
      <Checkbox aria-label="Select all" className="ml-0.5" checked={false} indeterminate />
      <span className="ml-1.5 text-small-body font-medium whitespace-nowrap text-fg tabular-nums">
        {sessions.length} selected
      </span>
      <span className="min-w-0 flex-1" />
      <Button aria-label="Delete" className="shrink-0" size="icon-sm" variant="ghost">
        <Trash2 className="size-3.5" />
      </Button>
      <SessionDropdownMenu
        nodes={nodes}
        open={pinned.selection}
        openSubs={pinned.openSubs}
        trigger={overflowTrigger("More actions")}
      />
      <Button aria-label="Done" className="shrink-0" size="icon-sm" variant="ghost">
        <X className="size-3.5" />
      </Button>
    </div>
  );
}

function ListRow({
  session,
  folderId = null,
  pinned,
  selected,
}: {
  session: SessionPayload;
  folderId?: string | null;
  pinned: PinnedMenu;
  selected: readonly string[];
}) {
  const nodes = sessionMenuTree(session, listFacts(session.id, folderId));
  const selection = selected.length > 0 ? selectionOf(selected) : undefined;
  const inSelection = selected.includes(session.id) && selected.length > 1;
  const menu = inSelection ? selectionMenuTree(selected.map(boardSession), SELECTION_FACTS) : nodes;
  return (
    <SessionListRowView
      current={session.id === "s4" && selected.length === 0}
      onSelect={() => undefined}
      preloadHandlers={SESSION_ENTRY_PRELOAD_FIXTURE}
      rowActions={
        <SessionDropdownMenu
          nodes={nodes}
          open={pinned.sessionId === session.id}
          openSubs={pinned.openSubs}
          trigger={overflowTrigger(`More actions for ${session.name}`)}
        />
      }
      runtime={boardRuntimeFixture(session)}
      selection={selection}
      session={session}
      sessionActions={actions}
      testIdPrefix="session-menus"
      wrapRow={row => <SessionContextMenu nodes={menu} render={row} />}
    />
  );
}

/** The sidebar's session list: a project group, its root rows and one open folder. */
export function ListSurface({
  pinned,
  selected = [],
}: {
  pinned: PinnedMenu;
  selected?: readonly string[];
}) {
  const [expanded, setExpanded] = useState(true);
  return (
    <aside
      aria-label="Sessions"
      className="flex flex-col gap-1 rounded-lg bg-sunken px-2.5 py-2"
      style={{ width: SESSION_SIDEBAR_DEFAULT_WIDTH }}
    >
      {selected.length > 0 ? (
        <SelectionBar pinned={pinned} sessions={selected.map(boardSession)} />
      ) : null}
      <SessionListGroupToggle
        collapsed={false}
        count={LIST_ROOT_IDS.length + LIST_FOLDER_IDS.length}
        identity={
          <WorkspaceBadge identity={{ kind: "project", name: "compozy" }} variant="chrome" />
        }
      />
      <div className="flex flex-col gap-0.5 pl-2.5">
        <SessionListFolder
          count={LIST_FOLDER_IDS.length}
          expanded={expanded}
          folder={{ id: "fix-patches", name: "fix-patches" }}
          onDelete={() => undefined}
          onRename={() => undefined}
          onToggle={() => setExpanded(value => !value)}
          testIdPrefix="session-menus"
        >
          {LIST_FOLDER_IDS.map(id => (
            <ListRow
              folderId="fix-patches"
              key={id}
              pinned={pinned}
              selected={selected}
              session={boardSession(id)}
            />
          ))}
        </SessionListFolder>
        {LIST_ROOT_IDS.map(id => (
          <ListRow key={id} pinned={pinned} selected={selected} session={boardSession(id)} />
        ))}
      </div>
    </aside>
  );
}

const BoardState = createContext<{ pinned: PinnedMenu; selected: readonly string[] }>({
  pinned: {},
  selected: [],
});

/** `SessionBoardCard`'s frame with the grouped menus in place of the flat ones. */
function DraftBoardCard({ actions: _actions, ...props }: SessionBoardCardProps) {
  const { pinned, selected } = use(BoardState);
  const { session } = props;
  const inSelection = selected.includes(session.id);
  const nodes =
    inSelection && selected.length > 1
      ? selectionMenuTree(selected.map(boardSession), SELECTION_FACTS)
      : sessionMenuTree(boardSession(session.id), boardFacts(session.id));
  const mode = selected.length > 0;
  return (
    <SessionContextMenu
      nodes={nodes}
      render={<div className="group/session-card relative w-full min-w-0 shrink-0" />}
    >
      <SessionBoardCardView
        {...props}
        runtime={boardRuntimeFixture(session)}
        selection={mode ? { selected: inSelection, mode } : undefined}
      />
      {mode ? (
        <Checkbox
          aria-label={`Select ${session.name}`}
          checked={inSelection}
          className={SESSION_BOARD_CARD_CHECKBOX}
        />
      ) : (
        <span className="absolute top-2 right-2 rounded-md bg-surface-2 opacity-0 transition-opacity duration-fast group-focus-within/session-card:opacity-100 group-hover/session-card:opacity-100 group-has-data-popup-open/session-card:opacity-100">
          <SessionDropdownMenu
            align="start"
            nodes={nodes}
            open={pinned.sessionId === session.id}
            openSubs={pinned.openSubs}
            trigger={overflowTrigger(`More actions for ${session.name}`)}
          />
        </span>
      )}
    </SessionContextMenu>
  );
}

const NONE: readonly string[] = [];

/** The Kanban board from the shipped view and fixtures; the selection bar takes the head. */
export function BoardSurface({
  pinned,
  selected = NONE,
  head,
}: {
  pinned: PinnedMenu;
  selected?: readonly string[];
  head?: ReactNode;
}) {
  const state = useMemo(() => ({ pinned, selected }), [pinned, selected]);
  return (
    <BoardState value={state}>
      <div className="flex min-h-dvh flex-col">
        <div className="flex h-11 shrink-0 items-center gap-3 border-b border-border-subtle px-4">
          <span className="text-small-body font-medium text-fg">Sessions</span>
          <span className="min-w-0 flex-1" />
          {selected.length > 0 ? (
            <SelectionBar
              className="min-h-0 px-0 py-0"
              pinned={pinned}
              sessions={selected.map(boardSession)}
            />
          ) : (
            head
          )}
        </div>
        <SessionBoardCardSlot value={DraftBoardCard}>
          <SessionsBoardView
            actions={actions}
            board={sessionsBoardViewFixture()}
            collapsedColumns={{ done: true }}
            onColumnCollapsedChange={() => undefined}
            onOpen={() => undefined}
          />
        </SessionBoardCardSlot>
      </div>
    </BoardState>
  );
}
