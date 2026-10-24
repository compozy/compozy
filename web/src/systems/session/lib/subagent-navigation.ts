import { createStore } from "@xstate/store";

import type { SubagentView } from "../components/subagents/types";
import type { SubagentNavigationTarget } from "../contexts/session-subagents-context-value";

interface SubagentRevealRequest {
  key: number;
  workspaceId: string;
  sessionId: string;
  subagentId: string;
}

/** One pending user navigation, retained while its parent window mounts. Never persisted. */
export const subagentRevealStore = createStore({
  context: { key: 0, request: null as SubagentRevealRequest | null },
  on: {
    requested: (context, event: Omit<SubagentRevealRequest, "key">) => {
      const key = context.key + 1;
      return { key, request: { ...event, key } };
    },
    settled: (context, event: { key: number }) =>
      context.request?.key === event.key ? { ...context, request: null } : context,
  },
});

/** Delegated work owns a session; provider-native work lives in the parent's transcript. */
export function subagentNavigationTarget(
  subagent: SubagentView,
  workspaceId: string
): SubagentNavigationTarget | null {
  const child = subagent.child_session_id;
  if (child) return { sessionId: child, agentName: subagent.runtime.agent ?? "", workspaceId };
  if (subagent.origin !== "provider_native" || !subagent.parent_session_id) return null;
  return {
    sessionId: subagent.parent_session_id,
    // Resolve the parent owner; the native runtime describes the subagent.
    agentName: "",
    workspaceId,
    subagentId: subagent.id,
  };
}
