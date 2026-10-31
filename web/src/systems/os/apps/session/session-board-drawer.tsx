import { useQuery } from "@tanstack/react-query";
import { shallowEqual } from "@xstate/store";
import {
  ChevronDown,
  ChevronRight,
  ChevronUp,
  Maximize2,
  XIcon,
  type LucideIcon,
} from "@compozy/ui/icons";
import { useState, type KeyboardEvent, type ReactNode } from "react";

import {
  Button,
  createTopbarSlotStore,
  Kbd,
  ResizableDrawer,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
  Topbar,
  TopbarSlotProvider,
  useTopbarSlotValue,
  type TopbarSlotStore,
} from "@compozy/ui";

import { useProfileReadScope } from "@/systems/profiles";
import {
  isEditableTarget,
  SESSION_BOARD_DRAWER_INLINE_BREAKPOINT,
  SESSION_BOARD_DRAWER_MAX_WIDTH,
  SESSION_BOARD_DRAWER_MIN_WIDTH,
  SessionRowActions,
  getSessionDisplayTitle,
  sessionScopedDetailOptions,
  sessionTreeParentId,
  SessionWindowPresentationContext,
  type SessionLifecycleActionHandlers,
} from "@/systems/session";

import { useDesktop } from "../../hooks/use-desktop";
import {
  parseShortcutChord,
  primaryShortcutModifier,
  shortcutActionLabel,
  shortcutMatches,
  type ShortcutMap,
} from "../../lib/window-manager-shortcuts";
import type { SessionWindowSidebarTarget } from "./use-session-window-open-session";

/** Escape that a menu, dialog or picker layered over the page handles for itself. */
const OTHER_LAYER_SELECTOR =
  '[role="menu"], [role="listbox"], [role="dialog"]:not([data-slot="resizable-drawer"]), [role="alertdialog"]';

export interface SessionBoardDrawerProps {
  /** The window's session view (`SessionWindowSession`), mounted only while the drawer is open. */
  session: ReactNode;
  sessionId: string | null;
  open: boolean;
  width: number;
  onWidthChange: (width: number) => void;
  onClose: () => void;
  onStep: (direction: "previous" | "next") => void;
  onExpand: () => void;
  /** Opens another session in the drawer: the parent crumb of a child session. */
  onOpenSession: (target: SessionWindowSidebarTarget) => void;
  returnFocus: () => HTMLElement | null;
  actions: SessionLifecycleActionHandlers;
  /** The board: stays mounted and live beside (or under) the drawer. */
  children: ReactNode;
}

/**
 * The board's session drawer (#1052): the window's own session view in a resizable pane
 * beside the board. It is the same view List shows — the session publishes its head into a
 * drawer-local topbar slot, which the drawer renders with its own controls — and it is not
 * modal: the board, its menus and dialogs stay usable while it is open. On a window too
 * narrow for the drawer and a board column side by side, it covers the board instead.
 */
export function SessionBoardDrawer({
  session,
  sessionId,
  open,
  width,
  onWidthChange,
  onClose,
  onStep,
  onExpand,
  onOpenSession,
  returnFocus,
  actions,
  children,
}: SessionBoardDrawerProps) {
  const [slotStore] = useState(createTopbarSlotStore);
  const effectiveShortcuts = useDesktop(state => state.windowManagerConfig?.effectiveShortcuts);
  // The dialog keeps arrow keys from leaving it, so the shell never sees the Previous/Next
  // session chords pressed in the drawer; the drawer answers them itself.
  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    const direction = sessionCycleDirection(event, effectiveShortcuts);
    if (direction === null) return;
    event.preventDefault();
    event.stopPropagation();
    onStep(direction);
  };
  return (
    <ResizableDrawer
      breakpointBasis="container"
      inlineBreakpoint={SESSION_BOARD_DRAWER_INLINE_BREAKPOINT}
      minWidth={SESSION_BOARD_DRAWER_MIN_WIDTH}
      maxWidth={SESSION_BOARD_DRAWER_MAX_WIDTH}
      modal={false}
      open={open}
      onOpenChange={(next, details) => {
        if (next) return;
        if (details.reason === "escape-key") {
          const event = details.event;
          const target = event.target instanceof Element ? event.target : null;
          if (event.defaultPrevented || target?.closest(OTHER_LAYER_SELECTOR)) return;
        }
        onClose();
      }}
      width={width}
      onWidthChange={onWidthChange}
      returnFocus={returnFocus}
      title={<SessionBoardDrawerTitle store={slotStore} />}
      header={
        <TopbarSlotProvider store={slotStore}>
          <div className="contents" onKeyDown={onKeyDown}>
            <Topbar
              data-testid="session-board-drawer-head"
              title="Session"
              leading={
                sessionId ? (
                  <SessionBoardDrawerParentCrumb onOpen={onOpenSession} sessionId={sessionId} />
                ) : null
              }
              controls={
                <SessionBoardDrawerControls
                  actions={actions}
                  onClose={onClose}
                  onExpand={onExpand}
                  onStep={onStep}
                  sessionId={sessionId}
                />
              }
            />
          </div>
        </TopbarSlotProvider>
      }
      content={
        <SessionWindowPresentationContext value="board-drawer">
          <TopbarSlotProvider store={slotStore}>
            <div
              className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden bg-canvas"
              data-testid="session-board-drawer-body"
              onKeyDown={onKeyDown}
            >
              {session}
            </div>
          </TopbarSlotProvider>
        </SessionWindowPresentationContext>
      }
    >
      {children}
    </ResizableDrawer>
  );
}

/** The Previous/Next session chord this keystroke is, outside text entry; else `null`. */
function sessionCycleDirection(
  event: KeyboardEvent,
  effective: ShortcutMap | undefined
): "previous" | "next" | null {
  if (event.defaultPrevented || event.repeat || isEditableTarget(event.target)) return null;
  const primary = primaryShortcutModifier(navigator.platform);
  for (const direction of ["previous", "next"] as const) {
    const chords = (effective?.[`session.cycle.${direction}`] ?? []).flatMap(
      raw => parseShortcutChord(raw) ?? []
    );
    if (chords.some(chord => shortcutMatches(event.nativeEvent, chord, primary))) return direction;
  }
  return null;
}

/** The dialog's accessible name: the session title its view published, once it has one. */
function SessionBoardDrawerTitle({ store }: { store: TopbarSlotStore }) {
  return (
    <TopbarSlotProvider store={store}>
      <SessionBoardDrawerTitleText />
    </TopbarSlotProvider>
  );
}

function SessionBoardDrawerTitleText() {
  const crumb = useTopbarSlotValue()?.crumb;
  return typeof crumb === "string" && crumb !== "" ? crumb : "Session";
}

function SessionBoardDrawerControls({
  sessionId,
  actions,
  onStep,
  onExpand,
  onClose,
}: {
  sessionId: string | null;
  actions: SessionLifecycleActionHandlers;
  onStep: (direction: "previous" | "next") => void;
  onExpand: () => void;
  onClose: () => void;
}) {
  const shortcuts = useDesktop(state => {
    const effective = state.windowManagerConfig?.effectiveShortcuts;
    const platform = typeof navigator === "undefined" ? "" : navigator.platform;
    return {
      previous: shortcutActionLabel(effective, "session.cycle.previous", platform),
      next: shortcutActionLabel(effective, "session.cycle.next", platform),
    };
  }, shallowEqual);
  return (
    <div className="flex items-center gap-0.5" data-testid="session-board-drawer-controls">
      <SessionBoardDrawerButton
        icon={ChevronUp}
        label="Previous session"
        shortcut={shortcuts.previous}
        testId="session-board-drawer-previous"
        onClick={() => onStep("previous")}
      />
      <SessionBoardDrawerButton
        icon={ChevronDown}
        label="Next session"
        shortcut={shortcuts.next}
        testId="session-board-drawer-next"
        onClick={() => onStep("next")}
      />
      <SessionBoardDrawerButton
        icon={Maximize2}
        label="Open full session"
        testId="session-board-drawer-expand"
        onClick={onExpand}
      />
      {sessionId ? <SessionBoardDrawerMenu actions={actions} sessionId={sessionId} /> : null}
      <SessionBoardDrawerButton
        icon={XIcon}
        label="Close session drawer"
        testId="session-board-drawer-close"
        onClick={onClose}
      />
    </div>
  );
}

/**
 * A child session's way back: its parent, ahead of the title, opens in the drawer. Forks and
 * continuations are new top-level sessions; their source stays the origin pill.
 */
function SessionBoardDrawerParentCrumb({
  sessionId,
  onOpen,
}: {
  sessionId: string;
  onOpen: (target: SessionWindowSidebarTarget) => void;
}) {
  const { params } = useProfileReadScope();
  const session = useQuery(sessionScopedDetailOptions(sessionId, params)).data;
  const parentId = session ? sessionTreeParentId(session) : "";
  const parent = useQuery({
    ...sessionScopedDetailOptions(parentId, params),
    enabled: parentId !== "",
  }).data;
  if (!parent) return null;
  const title = getSessionDisplayTitle(parent);
  return (
    <span className="flex min-w-0 items-center gap-1" data-testid="session-board-drawer-parent">
      <button
        aria-label={`Open parent session ${title}`}
        className="min-w-0 truncate rounded-xs text-small-body text-muted transition-colors duration-fast hover:text-fg focus-visible:shadow-focus-ring focus-visible:outline-none"
        onClick={() =>
          onOpen({
            sessionId: parent.id,
            agentName: parent.agent_name,
            workspaceId: parent.workspace_id ?? "",
          })
        }
        type="button"
      >
        {title}
      </button>
      <ChevronRight aria-hidden="true" className="size-3.5 shrink-0 text-subtle" />
    </span>
  );
}

/** The session list row menu for the drawer's session: rename, stop, archive, delete. */
function SessionBoardDrawerMenu({
  sessionId,
  actions,
}: {
  sessionId: string;
  actions: SessionLifecycleActionHandlers;
}) {
  const { params } = useProfileReadScope();
  const session = useQuery(sessionScopedDetailOptions(sessionId, params)).data;
  return session ? <SessionRowActions actions={actions} session={session} /> : null;
}

function SessionBoardDrawerButton({
  icon: Icon,
  label,
  shortcut,
  testId,
  onClick,
}: {
  icon: LucideIcon;
  label: string;
  shortcut?: string | null;
  testId: string;
  onClick: () => void;
}) {
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button
            aria-label={label}
            data-testid={testId}
            onClick={onClick}
            size="icon-sm"
            type="button"
            variant="quiet"
          />
        }
      >
        <Icon aria-hidden="true" className="size-4" />
      </TooltipTrigger>
      <TooltipContent>
        {label}
        {shortcut ? <Kbd>{shortcut}</Kbd> : null}
      </TooltipContent>
    </Tooltip>
  );
}
