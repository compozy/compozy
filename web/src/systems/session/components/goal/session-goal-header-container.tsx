import { useSessionGoalHeader } from "../../hooks/use-session-goal-header";
import { SessionGoalHeader } from "./session-goal-header";

interface SessionGoalHeaderContainerProps {
  sessionId: string;
  workspaceId: string;
  onPrefillComposer?: (text: string) => void;
  enabled?: boolean;
}

export function SessionGoalHeaderContainer({
  sessionId,
  workspaceId,
  onPrefillComposer,
  enabled = true,
}: SessionGoalHeaderContainerProps) {
  const goal = useSessionGoalHeader(workspaceId, sessionId, {
    enabled,
    onPrefillComposer,
    stream: enabled,
  });
  return <SessionGoalHeader {...goal} />;
}
