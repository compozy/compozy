import { use, type ReactNode } from "react";

import type { SubagentOpenOptions } from "../components/subagents/subagent-card";
import type { SubagentView } from "../components/subagents/types";
import { useSubagentRoster } from "../hooks/use-subagent-roster";
import {
  SessionSubagentsContext,
  SubagentNavigationContext,
} from "./session-subagents-context-value";

/**
 * One roster subscription per rendered session: the transcript's cards, the
 * composer banner and the status line read it from here (S1–S7).
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
  const roster = useSubagentRoster(workspaceId, sessionId);
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
    <SessionSubagentsContext value={{ workspaceId, sessionId, roster, onOpen }}>
      {children}
    </SessionSubagentsContext>
  );
}
