import type { SessionPayload } from "@/systems/session";

import { useSessionWindowDerive } from "../use-session-window-derive";
import { useSessionWindowSideChat } from "../use-session-window-side-chat";

/**
 * The sessions a window derives from its own: Continue/Fork and the origin link (`derive`), and
 * the side chats beside it (`sideChat`).
 */
export function useSessionWindowDerivedSessions({
  windowId,
  session,
  workspaceId,
}: {
  windowId: string;
  session: SessionPayload;
  workspaceId: string;
}) {
  const derive = useSessionWindowDerive({ session, workspaceId });
  const sideChat = useSessionWindowSideChat({ windowId, session, workspaceId });
  return { derive, sideChat };
}
