import type { ReactNode } from "react";

import { SessionContinueHost, useSessionContinueHost } from "@/systems/session";

import { useAttentionJump } from "../hooks/use-attention-jump";

export interface OsSessionsContinueHostProps {
  /** Workspace used when a source row does not name its own. */
  workspaceId: string | null;
  /** The child landed in its window; the list surface may step aside. */
  onLanded?: () => void;
  /** Receives whether the dialog is open, so an enclosing modal can hold dismissal. */
  children: (state: { continueOpen: boolean }) => ReactNode;
}

/**
 * Continue for session lists that live outside any session window (the
 * sessions modal, agent detail). There is no "this" session window here, so
 * the child always opens in its own window through the attention jump.
 */
export function OsSessionsContinueHost({
  workspaceId,
  onLanded,
  children,
}: OsSessionsContinueHostProps) {
  const jump = useAttentionJump();
  const host = useSessionContinueHost({ workspaceId: workspaceId ?? "" });
  return (
    <SessionContinueHost
      host={host}
      openInNewWindow={child => {
        onLanded?.();
        jump({
          sessionId: child.id,
          agentName: child.agent_name,
          workspaceId: child.workspace_id?.trim() || host.workspaceId,
        });
      }}
    >
      {children({ continueOpen: host.open })}
    </SessionContinueHost>
  );
}
