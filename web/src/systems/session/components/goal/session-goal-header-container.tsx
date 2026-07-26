import { useSessionGoalHeader } from "../../hooks/use-session-goal-header";
import { SessionGoalHeader } from "./session-goal-header";

interface SessionGoalHeaderContainerProps {
  sessionId: string;
  workspaceId: string;
  onPrefillComposer?: (text: string) => void;
}

export function SessionGoalHeaderContainer({
  sessionId,
  workspaceId,
  onPrefillComposer,
}: SessionGoalHeaderContainerProps) {
  const goal = useSessionGoalHeader(workspaceId, sessionId, onPrefillComposer);
  return <SessionGoalHeader {...goal} />;
}
