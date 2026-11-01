import { useSessionCreator } from "../../hooks/use-session-creator";
import { useSessionEntryPreload } from "../../hooks/use-session-entry-preload";
import { useSessionRowRuntimeLabel } from "../../hooks/use-session-row-runtime-label";
import { SessionRowActions } from "../session-row-actions";
import { SessionRowContextMenu } from "../session-row-context-menu";
import { StaysOnCreatorBadge } from "../stays-on/stays-on-signals";
import { SessionListRowView, type SessionListRowProps } from "./session-list-row-view";
import { SessionSelectionContextItems } from "./session-list-selection-menu";

export type { SessionListRowProps } from "./session-list-row-view";

/**
 * A session list row bound to the live runtime label, the session-entry intent preload and the
 * row menus (`…` actions and right-click); the presentation is `SessionListRowView`.
 */
export function SessionListRow(props: SessionListRowProps) {
  const { session, current = false, selection, sessionActions, layoutMenu } = props;
  const runtime = useSessionRowRuntimeLabel(session);
  const creator = useSessionCreator(session);
  const preloadHandlers = useSessionEntryPreload(session, !current && !selection?.mode);
  const selected = selection?.selectedIds.has(session.id) ?? false;
  // A row inside a multi-selection offers the selection's verbs; one selected row keeps its own.
  const verbs = selected && selection?.verbs && selection.verbs.count > 1 ? selection.verbs : null;
  return (
    <SessionListRowView
      {...props}
      preloadHandlers={preloadHandlers}
      rowActions={<SessionRowActions actions={sessionActions} session={session} />}
      runtime={runtime}
      trailing={
        creator ? (
          <>
            <StaysOnCreatorBadge creator={creator} />
            {props.trailing}
          </>
        ) : (
          props.trailing
        )
      }
      wrapRow={row => (
        <SessionRowContextMenu
          actions={sessionActions}
          layout={layoutMenu}
          render={row}
          selectionMenu={
            verbs ? (
              <SessionSelectionContextItems
                testId={`${props.testIdPrefix}-selection-menu`}
                verbs={verbs}
              />
            ) : undefined
          }
          session={session}
        />
      )}
    />
  );
}
