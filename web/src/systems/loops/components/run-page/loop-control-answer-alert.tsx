import { Alert, AlertDescription, AlertTitle } from "@compozy/ui";

import type { LoopControlAnswer } from "../../lib/loop-node-controls";

interface LoopControlAnswerAlertProps {
  answer: LoopControlAnswer;
}

/** Deterministic daemon answer — information, never a transport failure. */
export function LoopControlAnswerAlert({ answer }: LoopControlAnswerAlertProps) {
  return (
    <Alert data-testid="loop-control-answer" variant={answer.tone}>
      <AlertTitle>{answer.title}</AlertTitle>
      <AlertDescription>{answer.detail}</AlertDescription>
      {answer.micro ? (
        <p className="mt-1.5 font-mono text-pill-group-badge text-faint">{answer.micro}</p>
      ) : null}
    </Alert>
  );
}
