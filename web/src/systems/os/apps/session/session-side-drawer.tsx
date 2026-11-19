import { ResizableDrawer, toast } from "@compozy/ui";
import { useQuery } from "@tanstack/react-query";
import { useSelector } from "@xstate/store-react";
import { useRef, useState, type ReactNode } from "react";

import { SessionThreadSkeleton } from "@/components/assistant-ui/session-thread-skeleton";
import {
  findSessionFocusTarget,
  nextSideChatId,
  SessionSideCloseDialog,
  SessionSideHeader,
  SessionSidePanelBody,
  type SessionSidePanelBodyState,
  type SessionPayload,
  SIDE_CHAT_COPY,
  sessionDetailOptions,
  useCloseSideChat,
} from "@/systems/session";
import { SessionSideThread } from "./session-side-thread";
import type { SessionWindowSideChat } from "./use-session-window-side-chat";

/** The body for an existing side chat: its thread once its session reads, Retry when it can't. */
function useSideThreadBody(workspaceId: string, sideId: string | null): SessionSidePanelBodyState {
  const query = useQuery(sessionDetailOptions(workspaceId, sideId ?? ""));
  if (query.isError) return { kind: "unavailable", onRetry: () => void query.refetch() };
  const side: SessionPayload | undefined = query.data;
  if (!side) return { kind: "thread", thread: <SessionThreadSkeleton /> };
  return {
    kind: "thread",
    stopped: side.state === "stopped",
    thread: <SessionSideThread side={side} workspaceId={workspaceId} />,
  };
}

/**
 * The side chat panel (`_uiux.md` S3): the window's drawer placement shared with the subagent
 * drawer, non-modal so the main session stays usable beside it. Hiding keeps every side chat
 * running; focus returns to the main composer.
 */
export function SessionSideDrawer({
  chat,
  session,
  workspaceId,
  children,
}: {
  chat: SessionWindowSideChat;
  session: SessionPayload;
  workspaceId: string;
  children: ReactNode;
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

  return (
    <ResizableDrawer
      open={panel.open}
      onOpenChange={(open, details) => {
        if (open) return;
        // Non-modal: Escape reaches every open drawer, so only one holding focus hides (`_uiux.md`).
        if (details.reason === "escape-key" && !panelHasFocus()) return;
        store.trigger.sideHidden();
      }}
      modal={false}
      width={width}
      onWidthChange={next => store.trigger.resized({ width: next })}
      returnFocus={mainComposer}
      title={SIDE_CHAT_COPY.toggle}
      header={
        <SessionSideHeader
          closing={close.isPending}
          currentId={currentId}
          mainStatus={chat.mainStatus}
          onClose={current ? requestClose : undefined}
          onHide={() => store.trigger.sideHidden()}
          onMainStatusActivate={focusMain}
          onNew={() => chat.show({ kind: "new" })}
          onSelect={id => chat.show({ kind: "side", id })}
          pendingTitle={view?.kind === "opening" ? view.message : undefined}
          sides={list.sides}
        />
      }
      content={
        <>
          <div className="contents" ref={panelRef}>
            <SessionSidePanelBody state={body} />
          </div>
          <SessionSideCloseDialog
            closing={close.isPending}
            onConfirm={() => {
              if (closeTarget) closeSide(closeTarget.id);
            }}
            onOpenChange={open => {
              if (!open) setClosingId(null);
            }}
            open={closeTarget !== null}
          />
        </>
      }
    >
      <div className="contents" ref={hostRef}>
        {children}
      </div>
    </ResizableDrawer>
  );
}
