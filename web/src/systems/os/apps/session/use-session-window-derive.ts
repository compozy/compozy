import {
  type SessionOriginContextValue,
  type SessionPayload,
  useSessionDeriveHost,
  useSessionOrigin,
} from "@/systems/session";

import { useAttentionJump } from "../../hooks/use-attention-jump";

/**
 * Continue/Fork/origin wiring for one session window. New window lands through the
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
  const deriveHost = useSessionDeriveHost({ workspaceId, currentSession: session });
  const origin = useSessionOrigin(session, workspaceId);

  // New window keeps the source visible, so the child splits beside it rather
  // than joining its frame as a tab (the default for other opens).
  const openInNewWindow = (child: SessionPayload) =>
    jump({
      sessionId: child.id,
      agentName: child.agent_name,
      workspaceId: child.workspace_id?.trim() || workspaceId,
      placement: "split",
    });
  const onOpenOriginSource = (sessionId: string) => jump({ sessionId, workspaceId });
  const originContext: SessionOriginContextValue | null = origin
    ? { origin, onOpenSource: onOpenOriginSource }
    : null;

  return {
    deriveHost,
    openInNewWindow,
    origin,
    originContext,
    onOpenOriginSource,
    onContinue: () => deriveHost.request(),
    onFork: () => deriveHost.requestFork(),
  };
}
