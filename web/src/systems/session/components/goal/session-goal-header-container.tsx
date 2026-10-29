import { useSessionGoalHeader } from "../../hooks/use-session-goal-header";
import type { SessionState } from "../../types";
import { SessionGoalHeader } from "./session-goal-header";

interface SessionGoalHeaderContainerProps {
  sessionId: string;
  workspaceId: string;
  onPrefillComposer?: (text: string) => void;
  enabled?: boolean;
  sessionState?: SessionState;
  stopping?: boolean;
  readOnly?: boolean;
}

export function SessionGoalHeaderContainer({
  sessionId,
  workspaceId,
  onPrefillComposer,
  enabled = true,
  sessionState,
  stopping,
  readOnly,
}: SessionGoalHeaderContainerProps) {
  const goal = useSessionGoalHeader(workspaceId, sessionId, {
    enabled,
    onPrefillComposer,
    stream: enabled,
    sessionState,
    stopping,
    readOnly,
  });
  return <SessionGoalHeader {...goal} />;
}
