import { toast } from "@compozy/ui";
import { useSelector, useStore } from "@xstate/store-react";
import { use, useEffect, useEffectEvent, useSyncExternalStore } from "react";

import type { SessionComposerSubmitInterceptor } from "@/components/assistant-ui/session-composer";
import {
  canHostSideChats,
  defaultSideChatId,
  isSessionTransportDisconnected,
  parseSideCommand,
  type SessionPayload,
  sideChatHostRefusal,
  type SessionSideView,
  SessionSideToggle,
  SIDE_CHAT_COPY,
  sessionWindowDrawerLogic,
  sideMainStatus,
  subscribeSessionSideToggle,
  useOpenSideChat,
  useSessionSides,
  useSessionTransportState,
} from "@/systems/session";

import { OsShellContext } from "../../contexts/os-shell-context";
import { shortcutActionLabel } from "../../lib/window-manager-shortcuts";

const noSubscription = () => () => undefined;
const noShortcut = () => null;

/**
 * The effective `session.side.toggle` chord for the toggle's tooltip, from the keymap; null
 * outside the OS shell (a standalone route, a test).
 */
function useSideToggleShortcut(): string | null {
  const projection = use(OsShellContext)?.projection;
  return useSyncExternalStore(
    projection
      ? onChange => {
          const subscription = projection.subscribe(onChange);
          return () => subscription.unsubscribe();
        }
      : noSubscription,
    projection
      ? () =>
          shortcutActionLabel(
            projection.get().windowManagerConfig?.effectiveShortcuts,
            "session.side.toggle",
            typeof navigator === "undefined" ? "" : navigator.platform
          )
      : noShortcut,
    noShortcut
  );
}

/**
 * One session window's side chats (`_uiux.md` Component plan): the window's single drawer store
 * (shared with the subagent drawer), the side list, the open mutation, the head toggle, the
 * `session.side.toggle` shortcut, and the `/side` routing for both composer submit paths.
 */
export function useSessionWindowSideChat({
  windowId,
  session,
  workspaceId,
}: {
  windowId: string;
  session: SessionPayload;
  workspaceId: string;
}) {
  const store = useStore(sessionWindowDrawerLogic);
  const panel = useSelector(store, snapshot => snapshot.context.side);
  const supported = canHostSideChats(session);
  const list = useSessionSides(workspaceId, session.id, { enabled: supported });
  const opener = useOpenSideChat({
    workspaceId,
    sessionId: session.id,
    store,
    onRefused: message => toast.error(message),
  });
  const transport = useSessionTransportState();
  const shortcut = useSideToggleShortcut();
  // Truthful UI: while the main stream reconnects the chip shows nothing, never a stale state.
  const mainStatus = sideMainStatus(
    session,
    transport.phase !== "failed" && !isSessionTransportDisconnected(transport)
  );

  /**
   * The kept view while its side chat is still open, else the last viewed or newest one. Before
   * the first list arrives that choice is `pending`: an empty placeholder list must not pick a new
   * side chat over existing ones (B-009).
   */
  const restoredView = (): SessionSideView => {
    const { view, lastViewedId } = store.getSnapshot().context.side;
    if (!list.loaded) return view && view.kind !== "side" ? view : { kind: "pending" };
    if (view && (view.kind !== "side" || list.sides.some(side => side.id === view.id))) return view;
    const id = defaultSideChatId(list.sides, lastViewedId);
    return id ? { kind: "side", id } : { kind: "new" };
  };
  const show = (view?: SessionSideView) => {
    store.trigger.sideShown({ view: view ?? restoredView() });
  };
  const toggle = () => {
    if (!supported) return;
    if (store.getSnapshot().context.side.open) store.trigger.sideHidden();
    else show();
  };

  const onShortcut = useEffectEvent(toggle);
  useEffect(() => subscribeSessionSideToggle(windowId, () => onShortcut()), [windowId]);

  // Each successful list read is the server's word on which side chats are open: it settles a
  // pending default and moves off a side chat closed or deleted in another window (US-008.EC-1).
  const onListed = useEffectEvent(() => {
    store.trigger.sidesListed({ ids: list.sides.map(side => side.id) });
  });
  const listedAt = list.listedAt;
  useEffect(() => {
    if (listedAt > 0) onListed();
  }, [listedAt]);

  const submitInterceptor: SessionComposerSubmitInterceptor = {
    claims: text => parseSideCommand(text) !== null,
    submit: async ({ text, attachmentCount }) => {
      const command = parseSideCommand(text);
      if (!command) return false;
      // The same eligibility as the toggle and the command menu; the text stays in the composer.
      const refusal = sideChatHostRefusal(session);
      if (refusal) {
        toast.error(refusal);
        return false;
      }
      if (attachmentCount > 0) {
        toast.error(SIDE_CHAT_COPY.attachmentsRefused);
        return false;
      }
      // A bare `/side` opens a new, empty side chat and sends nothing (US-001.AC-4).
      if (command.message === "") {
        show({ kind: "new" });
        return true;
      }
      return (await opener.open(command.message)) !== "refused";
    },
  };

  /** The `/side` chip picked from the command menu: a new, empty side chat. */
  const onCommandAction = (token: string): boolean => {
    if (token !== "/side" || !supported) return false;
    show({ kind: "new" });
    return true;
  };

  return {
    store,
    panel,
    list,
    opener,
    mainStatus,
    show,
    toggle,
    submitInterceptor,
    onCommandAction,
    /** The head toggle; absent where the session can't hold side chats. */
    control: supported ? (
      <SessionSideToggle
        attention={list.attention}
        count={list.sides.length}
        onToggle={toggle}
        open={panel.open}
        shortcut={shortcut ?? undefined}
      />
    ) : undefined,
  };
}

export type SessionWindowSideChat = ReturnType<typeof useSessionWindowSideChat>;
