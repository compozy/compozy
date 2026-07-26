import { Alert, AlertDescription } from "@agh/ui";
import { CircleAlert } from "lucide-react";

import { useSessionStore } from "@/systems/session/hooks/use-session-store";
import { describeGoalCommandFailure } from "@/systems/session/lib/session-goal-chat-transport";

export function SessionGoalCommandErrorNotice({ sessionId }: { sessionId: string }) {
  const result = useSessionStore(state => state.goalResults[sessionId]);
  const visible = useSessionStore(state => state.goalErrorVisible[sessionId] === true);
  const guidance =
    visible && result?.outcome === "error" ? describeGoalCommandFailure(result.reason_code) : null;

  if (!guidance) {
    return null;
  }

  return (
    <Alert variant="danger" data-testid="session-goal-command-error">
      <CircleAlert />
      <AlertDescription>{guidance}</AlertDescription>
    </Alert>
  );
}
