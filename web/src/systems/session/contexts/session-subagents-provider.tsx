import { use, type ReactNode } from "react";

import type { SubagentOpenOptions } from "../components/subagents/subagent-card";
import type { SubagentView } from "../components/subagents/types";
import { SubagentDrawerContext } from "./subagent-drawer-context";
import { subagentNavigationTarget } from "../lib/subagent-navigation";
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
  const drawer = use(SubagentDrawerContext);
  const onOpen = drawer
    ? (subagent: SubagentView, options: SubagentOpenOptions) =>
        drawer.open(
          { ...subagent, parent_session_id: subagent.parent_session_id || sessionId },
          options,
          workspaceId
        )
    : navigation
      ? (subagent: SubagentView, { newWindow }: SubagentOpenOptions) => {
          const target = subagentNavigationTarget(subagent, workspaceId);
          if (target) navigation.openSession(target, { newWindow });
        }
      : undefined;
  return (
    <SessionSubagentsContext value={{ workspaceId, sessionId, onOpen }}>
      {children}
    </SessionSubagentsContext>
  );
}
