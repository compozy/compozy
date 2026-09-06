import type { SteerProvenance } from "@/systems/session";

import { UserMessageBubble } from "./session-user-message";
import { SessionUserSteerMeta } from "./session-user-steer-meta";

/**
 * Guidance the daemon recorded but never dispatched as a normal message — a
 * steer still waiting on the current tool, or one superseded before delivery —
 * rendered as the operator's bubble from the marker's own `authored_text`,
 * with the same meta line as a real message (VC-07). Exactly one per identity;
 * it disappears the moment the real user message is loaded.
 */
export function SessionSteerReceipt({ provenance }: { provenance: SteerProvenance }) {
  const superseded = provenance.kind === "superseded";
  return (
    <div
      className="flex w-full min-w-0 justify-end pt-1 pb-transcript-turn-gap"
      data-message-id-receipt={provenance.messageId}
      data-steer={provenance.kind}
      data-testid="user-message-receipt"
    >
      <div className="flex max-w-[80%] min-w-0 flex-col items-end gap-transcript-meta-gap">
        <UserMessageBubble subdued={superseded}>
          <span className="whitespace-pre-wrap">{provenance.authoredText}</span>
        </UserMessageBubble>
        <SessionUserSteerMeta kind={provenance.kind} />
      </div>
    </div>
  );
}
