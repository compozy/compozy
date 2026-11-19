import { ResizableDrawer } from "@compozy/ui";
import type { ReactNode } from "react";

import {
  SessionSideCloseDialog,
  SessionSideHeader,
  SessionSidePanelBody,
  type SessionPayload,
  SIDE_CHAT_COPY,
} from "@/systems/session";
import { useSessionSideDrawer } from "./hooks/use-session-side-drawer";
import type { SessionWindowSideChat } from "./use-session-window-side-chat";

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
  const { hostRef, panelRef, body, drawer, header, closeDialog } = useSessionSideDrawer({
    chat,
    session,
    workspaceId,
  });

  return (
    <ResizableDrawer
      {...drawer}
      modal={false}
      title={SIDE_CHAT_COPY.toggle}
      header={<SessionSideHeader {...header} />}
      content={
        <>
          <div className="contents" ref={panelRef}>
            <SessionSidePanelBody state={body} />
          </div>
          <SessionSideCloseDialog {...closeDialog} />
        </>
      }
    >
      <div className="contents" ref={hostRef}>
        {children}
      </div>
    </ResizableDrawer>
  );
}
