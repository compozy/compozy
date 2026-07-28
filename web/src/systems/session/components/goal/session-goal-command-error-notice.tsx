import { Alert, AlertDescription } from "@compozy/ui";
import { CircleAlert } from "lucide-react";

import { useSessionGoalFeedback } from "@/systems/session/hooks/use-session-store";
import { describeGoalCommandFailure } from "@/systems/session/lib/session-goal-chat-transport";

export function SessionGoalCommandErrorNotice({ sessionId }: { sessionId: string }) {
  const feedback = useSessionGoalFeedback(sessionId);
  const result = feedback?.result;
  const visible = feedback?.errorVisible === true;
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
