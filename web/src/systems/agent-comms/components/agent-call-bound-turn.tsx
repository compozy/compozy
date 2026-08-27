/**
 * The child's first plate: who asked, and what they asked.
 *
 * Never labels the child as the asker. A missing caller name falls back to
 * "the caller" rather than a session id or the child's own name.
 */
import { MonoId } from "@compozy/ui";

import { callerDisplayName, operatorAskGist } from "../lib/ask-gist";
import type { SyntheticTurn } from "../lib/synthetic-turn";

export interface AgentCallBoundTurnProps {
  turn: SyntheticTurn;
  text: string;
  "data-testid"?: string;
}

export function AgentCallBoundTurn({ turn, text, "data-testid": testId }: AgentCallBoundTurnProps) {
  const caller = callerDisplayName(turn);
  const gist = operatorAskGist(text);
  return (
    <div
      className="rounded-md border border-line-soft px-2.5 py-2"
      data-call-id={turn.callId ?? undefined}
      data-synthetic-kind={turn.kind}
      data-testid={testId}
    >
      <p className="text-small-body text-fg">
        Asked by <span className="font-medium">{caller}</span>
        {gist !== "" ? <span> — {gist}</span> : null}
      </p>
      {turn.contractDigest ? (
        <p className="mt-1 text-transcript-meta text-muted">
          contract <MonoId value={turn.contractDigest} />
          {turn.requiredKeyCount !== null ? ` · ${String(turn.requiredKeyCount)} keys` : null}
        </p>
      ) : null}
    </div>
  );
}
