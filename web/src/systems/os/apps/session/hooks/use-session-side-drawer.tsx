import { type ResizableDrawerProps, toast } from "@compozy/ui";
import { useSelector } from "@xstate/store-react";
import { useRef, useState } from "react";

import { SessionThreadSkeleton } from "@/components/assistant-ui/session-thread-skeleton";
import {
  findSessionFocusTarget,
  nextSideChatId,
  type SessionPayload,
  type SessionSidePanelBodyState,
  SIDE_CHAT_COPY,
  useCloseSideChat,
} from "@/systems/session";

import type { SessionWindowSideChat } from "../use-session-window-side-chat";
import { useSideThreadBody } from "./use-side-thread-body";

/**
 * The side chat panel's behavior (`_uiux.md` S3): what the panel shows, closing with a
 * confirmation bound to the side chat it opened for, focus back to the main pane, and Escape
 * only while focus is inside the non-modal panel.
 */
export function useSessionSideDrawer({
  chat,
  session,
  workspaceId,
}: {
  chat: SessionWindowSideChat;
  session: SessionPayload;
  workspaceId: string;
}) {
  const hostRef = useRef<HTMLDivElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  const width = useSelector(chat.store, snapshot => snapshot.context.width);
  /** The side chat the close confirmation is for: confirming acts on it, not the current one. */
  const [closingId, setClosingId] = useState<string | null>(null);
  const close = useCloseSideChat({
    workspaceId,
    parentSessionId: session.id,
    profile: session.profile_name,
  });
  const { panel, list, opener, store } = chat;
  const view = panel.view;
  const currentId = view?.kind === "side" ? view.id : null;
  const current = list.sides.find(side => side.id === currentId) ?? null;
  // A confirmation whose side chat left the list (closed elsewhere) dismisses itself.
  const closeTarget = list.sides.find(side => side.id === closingId) ?? null;
  const threadBody = useSideThreadBody(workspaceId, currentId);

  const mainComposer = () => findSessionFocusTarget(hostRef.current, "composer");
  const focusMain = () => {
    const permission =
      chat.mainStatus?.kind === "needs-approval"
        ? findSessionFocusTarget(hostRef.current, "permissionAllowOnce")
        : null;
    (permission ?? mainComposer())?.focus();
  };
  /** Focus is in this panel: the drawer's popup holds the panel body. */
  const panelHasFocus = () => {
    const popup = panelRef.current?.closest<HTMLElement>('[data-slot="resizable-drawer"]');
    return Boolean(popup?.contains(popup.ownerDocument.activeElement));
  };

  const closeSide = (id: string) => {
    close.mutate(
      { sideId: id },
      {
        onSuccess: () => {
          setClosingId(null);
          store.trigger.sideClosed({ id, nextId: nextSideChatId(list.sides, id) });
        },
        onError: () => toast.error(SIDE_CHAT_COPY.closeFailed),
      }
    );
  };
  // Closing a side chat that is still answering stops it: confirm first (US-008.AC-2).
  const requestClose = () => {
    if (!current) return;
    if (current.prompting) setClosingId(current.id);
    else closeSide(current.id);
  };

  const body = ((): SessionSidePanelBodyState => {
    // The toggle's default waits for the first list: loading, or Retry when it can't be read.
    if (view?.kind === "pending") {
      return list.isError
        ? { kind: "unavailable", onRetry: list.refetch }
        : { kind: "thread", thread: <SessionThreadSkeleton /> };
    }
    if (view === null || view.kind === "new") {
      const { draft } = panel;
      return {
        kind: "new",
        draft,
        onDraftChange: next => store.trigger.sideDraftChanged({ draft: next }),
        onSend: () => {
          const message = draft.trim();
          if (message) void opener.open(message, { fromPanel: true });
        },
      };
    }
    if (view.kind === "opening") {
      return view.error === null
        ? { kind: "opening", message: view.message }
        : {
            kind: "failed",
            message: view.message,
            error: view.error,
            onRetry: opener.retry,
            retrying: opener.pending,
          };
    }
    return threadBody;
  })();

  return {
    hostRef,
    panelRef,
    body,
    drawer: {
      open: panel.open,
      width,
      returnFocus: mainComposer,
      onOpenChange: ((open, details) => {
        if (open) return;
        // Non-modal: Escape reaches every open drawer, so only one holding focus hides (`_uiux.md`).
        if (details.reason === "escape-key" && !panelHasFocus()) return;
        store.trigger.sideHidden();
      }) satisfies ResizableDrawerProps["onOpenChange"],
      onWidthChange: (next: number) => store.trigger.resized({ width: next }),
    },
    header: {
      closing: close.isPending,
      currentId,
      mainStatus: chat.mainStatus,
      onClose: current ? requestClose : undefined,
      onHide: () => store.trigger.sideHidden(),
      onMainStatusActivate: focusMain,
      onNew: () => chat.show({ kind: "new" }),
      onSelect: (id: string) => chat.show({ kind: "side", id }),
      pendingTitle: view?.kind === "opening" ? view.message : undefined,
      sides: list.sides,
    },
    closeDialog: {
      closing: close.isPending,
      open: closeTarget !== null,
      onConfirm: () => {
        if (closeTarget) closeSide(closeTarget.id);
      },
      onOpenChange: (open: boolean) => {
        if (!open) setClosingId(null);
      },
    },
  };
}
