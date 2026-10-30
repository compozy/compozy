import { Checkbox } from "@compozy/ui";

import { cn } from "@/lib/utils";

import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import { useSessionEntryPreload } from "../../hooks/use-session-entry-preload";
import { useSessionRowRuntimeLabel } from "../../hooks/use-session-row-runtime-label";
import { useSessionHasDraft } from "../../hooks/use-session-store";
import { useSessionBoardCardSelection } from "../../hooks/use-sessions-board-selection";
import type { SessionBoardPayload } from "../../lib/session-board-contract";
import { getSessionDisplayTitle } from "../../lib/session-display-title";
import { SessionRowActions } from "../session-row-actions";
import { SessionRowContextMenu } from "../session-row-context-menu";
import { SessionBoardCardView } from "./session-board-card-view";
import { SessionsBoardSelectionMenu } from "./sessions-board-selection-menu";
import { SESSION_BOARD_CARD_CHECKBOX } from "./sessions-board-frame";

export interface SessionBoardCardProps {
  session: SessionBoardPayload;
  /** All projects: the workspace name, shown in the meta row when the card has no PR. */
  workspaceName?: string | null;
  current?: boolean;
  archived?: boolean;
  onOpen: (session: SessionBoardPayload) => void;
  /**
   * The session list's lifecycle verbs: the card then carries the same actions
   * as a list row, on right-click (and Shift+F10) and behind its `…` button.
   */
  actions?: SessionLifecycleActionHandlers;
}

type CardSelection = NonNullable<ReturnType<typeof useSessionBoardCardSelection>>;

/**
 * One session on the board (`_uiux.md` S5), bound to the live runtime label
 * and the session-entry intent preload. The card body is one button that opens
 * the session. With `actions`, the card also takes the session list row's menu:
 * right-click opens it as a context menu, and a `…` button revealed on hover or
 * focus opens it as a dropdown. That button sits beside the open button, never
 * inside it, so choosing a verb never opens the session.
 *
 * Inside the board selection the card selects like a list row: Cmd/Ctrl-click
 * toggles it, Shift-click ranges within its column, and while a selection is
 * open every click toggles. Its checkbox takes the provider mark's place, the
 * `…` gives way as a list row's actions do, and a card inside a selection of
 * several opens the selection's "N sessions" menu instead of its own.
 */
export function SessionBoardCard({ actions, ...props }: SessionBoardCardProps) {
  const { session } = props;
  const selection = useSessionBoardCardSelection(session.id);
  const button = <SessionBoardCardButton {...props} selection={selection} />;
  if (!actions) return button;
  return (
    <SessionRowContextMenu
      actions={actions}
      render={
        <div
          className="group/session-card relative w-full min-w-0 shrink-0"
          onContextMenu={selection?.retarget}
        />
      }
      selectionMenu={
        selection?.inMultiSelection ? (
          <SessionsBoardSelectionMenu testId="sessions-board-selection-menu" />
        ) : undefined
      }
      session={session}
    >
      {button}
      {selection ? <SessionBoardCardCheckbox selection={selection} session={session} /> : null}
      {selection?.mode ? null : (
        <span
          className="absolute top-2 right-2 rounded-md bg-surface-2 opacity-0 transition-opacity duration-fast group-focus-within/session-card:opacity-100 group-hover/session-card:opacity-100 group-has-data-popup-open/session-card:opacity-100 motion-reduce:transition-none"
          data-testid={`session-board-card-actions-${session.id}`}
        >
          <SessionRowActions actions={actions} session={session} />
        </span>
      )}
    </SessionRowContextMenu>
  );
}

function SessionBoardCardButton({
  selection,
  ...props
}: Omit<SessionBoardCardProps, "actions"> & { selection: CardSelection | null }) {
  // A click inside an open selection toggles, so it never warms the session's entry.
  const preloadHandlers = useSessionEntryPreload(props.session, !props.current && !selection?.mode);
  const runtime = useSessionRowRuntimeLabel(props.session);
  const draft = useSessionHasDraft(props.session.id);
  return (
    <SessionBoardCardView
      {...props}
      {...preloadHandlers}
      onActivate={
        selection
          ? event => {
              if (!selection.activate(event)) props.onOpen(props.session);
            }
          : undefined
      }
      draft={draft}
      runtime={runtime}
      selection={selection ? { selected: selection.selected, mode: selection.mode } : undefined}
    />
  );
}

/** Revealed on hover or focus, and on every card while a selection is open. */
function SessionBoardCardCheckbox({
  session,
  selection,
}: {
  session: SessionBoardPayload;
  selection: CardSelection;
}) {
  return (
    <Checkbox
      aria-label={`Select ${getSessionDisplayTitle(session)}`}
      checked={selection.selected}
      className={cn(
        SESSION_BOARD_CARD_CHECKBOX,
        "transition-opacity duration-fast motion-reduce:transition-none",
        selection.mode
          ? "opacity-100"
          : "pointer-events-none opacity-0 group-focus-within/session-card:pointer-events-auto group-focus-within/session-card:opacity-100 group-hover/session-card:pointer-events-auto group-hover/session-card:opacity-100"
      )}
      data-testid={`session-board-card-select-${session.id}`}
      onCheckedChange={(_checked, details) => selection.check(details.event)}
    />
  );
}
