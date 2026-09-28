import {
  type SessionOriginContextValue,
  type SessionPayload,
  useSessionContinueHost,
  useSessionOrigin,
} from "@/systems/session";

import { useAttentionJump } from "../../hooks/use-attention-jump";

/**
 * Continue/origin wiring for one session window. New window lands through the
 * attention jump (`userOpen`, which focuses a window the child already owns);
 * the window content adds This window (the sidebar's in-place retarget). The
 * origin link opens the source the same way a notification would.
 */
export function useSessionWindowDerive({
  session,
  workspaceId,
}: {
  session: SessionPayload;
  workspaceId: string;
}) {
  const jump = useAttentionJump();
  const continueHost = useSessionContinueHost({ workspaceId, currentSession: session });
  const origin = useSessionOrigin(session, workspaceId);

  const openInNewWindow = (child: SessionPayload) =>
    jump({
      sessionId: child.id,
      agentName: child.agent_name,
      workspaceId: child.workspace_id?.trim() || workspaceId,
    });
  const onOpenOriginSource = (sessionId: string) => jump({ sessionId, workspaceId });
  const originContext: SessionOriginContextValue | null = origin
    ? { origin, onOpenSource: onOpenOriginSource }
    : null;

  return {
    continueHost,
    openInNewWindow,
    origin,
    originContext,
    onOpenOriginSource,
    onContinue: () => continueHost.request(),
  };
}
