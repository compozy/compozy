import type { ReactNode } from "react";

import { SessionDeriveHost, useSessionDeriveHost } from "@/systems/session";

import { useAttentionJump } from "../hooks/use-attention-jump";

export interface OsSessionsDeriveHostProps {
  /** Workspace used when a source row does not name its own. */
  workspaceId: string | null;
  /** The child landed in its window; the list surface may step aside. */
  onLanded?: () => void;
  /** Receives whether the dialog is open, so an enclosing modal can hold dismissal. */
  children: (state: { deriveOpen: boolean }) => ReactNode;
}

/**
 * Continue and Fork for session lists that live outside any session window (the
 * sessions modal, agent detail). There is no "this" session window here, so
 * the child always opens in its own window through the attention jump.
 */
export function OsSessionsDeriveHost({
  workspaceId,
  onLanded,
  children,
}: OsSessionsDeriveHostProps) {
  const jump = useAttentionJump();
  const host = useSessionDeriveHost({ workspaceId: workspaceId ?? "" });
  return (
    <SessionDeriveHost
      host={host}
      openInNewWindow={child => {
        onLanded?.();
        jump({
          sessionId: child.id,
          agentName: child.agent_name,
          workspaceId: child.workspace_id?.trim() || host.workspaceId,
          placement: "split",
        });
      }}
    >
      {children({ deriveOpen: host.open })}
    </SessionDeriveHost>
  );
}
