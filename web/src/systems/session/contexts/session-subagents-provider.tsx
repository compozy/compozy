import { use, type ReactNode } from "react";

import type { SubagentOpenOptions } from "../components/subagents/subagent-card";
import type { SubagentView } from "../components/subagents/types";
import {
  SessionSubagentsContext,
  SubagentNavigationContext,
} from "./session-subagents-context-value";

/**
 * Names the rendered session's roster for the transcript's cards, the composer
 * banner and the status line (S1–S7), and wires card drill-in to the window.
 */
export function SessionSubagentsProvider({
  workspaceId,
  sessionId,
  children,
}: {
  workspaceId: string;
  sessionId: string;
  children: ReactNode;
}) {
  const navigation = use(SubagentNavigationContext);
  const onOpen = navigation
    ? (subagent: SubagentView, { newWindow }: SubagentOpenOptions) => {
        const child = subagent.child_session_id;
        if (!child) return;
        navigation.openSession(
          { sessionId: child, agentName: subagent.runtime.agent ?? "", workspaceId },
          { newWindow }
        );
      }
    : undefined;
  return (
    <SessionSubagentsContext value={{ workspaceId, sessionId, onOpen }}>
      {children}
    </SessionSubagentsContext>
  );
}
