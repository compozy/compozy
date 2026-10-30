import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import { useSessionEntryPreload } from "../../hooks/use-session-entry-preload";
import { useSessionRowRuntimeLabel } from "../../hooks/use-session-row-runtime-label";
import { useSessionHasDraft } from "../../hooks/use-session-store";
import type { SessionBoardPayload } from "../../lib/session-board-contract";
import { SessionRowActions } from "../session-row-actions";
import { SessionRowContextMenu } from "../session-row-context-menu";
import { SessionBoardCardView } from "./session-board-card-view";

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

/**
 * One session on the board (`_uiux.md` S5), bound to the live runtime label
 * and the session-entry intent preload. The card body is one button that opens
 * the session in List. With `actions`, the card also takes the session list
 * row's menu: right-click opens it as a context menu, and a `…` button revealed
 * on hover or focus opens it as a dropdown. That button sits beside the open
 * button, never inside it, so choosing a verb never opens the session.
 */
export function SessionBoardCard({ actions, ...props }: SessionBoardCardProps) {
  const button = <SessionBoardCardButton {...props} />;
  if (!actions) return button;
  const { session } = props;
  return (
    <SessionRowContextMenu
      actions={actions}
      render={<div className="group/session-card relative w-full min-w-0 shrink-0" />}
      session={session}
    >
      {button}
      <span
        className="absolute top-2 right-2 rounded-md bg-surface-2 opacity-0 transition-opacity duration-fast group-focus-within/session-card:opacity-100 group-hover/session-card:opacity-100 group-has-data-popup-open/session-card:opacity-100 motion-reduce:transition-none"
        data-testid={`session-board-card-actions-${session.id}`}
      >
        <SessionRowActions actions={actions} session={session} />
      </span>
    </SessionRowContextMenu>
  );
}

function SessionBoardCardButton(props: Omit<SessionBoardCardProps, "actions">) {
  const preloadHandlers = useSessionEntryPreload(props.session, !props.current);
  const runtime = useSessionRowRuntimeLabel(props.session);
  const draft = useSessionHasDraft(props.session.id);
  return <SessionBoardCardView {...props} {...preloadHandlers} draft={draft} runtime={runtime} />;
}
