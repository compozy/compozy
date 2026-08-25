/**
 * A call as it appears inside the conversation that made it.
 *
 * **Closed by default.** The resting card is one 34px head row — caret, agent,
 * state chip, liveness, age — and that is deliberate: a transcript is a reading
 * surface, and a delegation that unfolds its ask and its answer inline turns
 * every hand-off into a wall. A closed card still informs, because the chip
 * grammar carries the state and the bell carries the urgency; opening it is for
 * when the operator wants the detail, and the full record is one click further.
 *
 * Uses a native `<details>` rather than managed state: disclosure is exactly
 * what the element is for, it is keyboard- and screen-reader-correct with no
 * work, and it survives the transcript virtualizing around it.
 */
import { ChevronRight } from "lucide-react";

import { OwnerAvatar, Time } from "@compozy/ui";

import {
  AgentCallLiveness,
  AgentCallStatePill,
  AgentCallVerdictChip,
} from "./agent-call-state-pill";
import { toCallState, toCallVerdict } from "../lib/call-state";
import type { CallPayload } from "../types";

/** A one-line gist of the answer, or an honest word about why there is none. */
function resultGist(call: CallPayload): string {
  if (call.result_preview !== undefined) {
    const encoded = JSON.stringify(call.result_preview);
    if (encoded !== undefined) {
      return encoded.length > 120 ? `${encoded.slice(0, 117)}…` : encoded;
    }
  }
  if (call.failure_detail) return call.failure_detail;
  return "No answer was recorded.";
}

export interface AgentCallTurnCardProps {
  call: CallPayload;
  /** The ask, once the projection carries it. */
  prompt?: string | null;
  onOpenCall: (callId: string) => void;
  /** Stage the card open — used by stories and by a deep link into one call. */
  defaultOpen?: boolean;
  "data-testid"?: string;
}

export function AgentCallTurnCard({
  call,
  prompt = null,
  onOpenCall,
  defaultOpen = false,
  "data-testid": testId,
}: AgentCallTurnCardProps) {
  const state = toCallState(call.state);
  const agentName = call.agent ?? call.call_id;
  return (
    <details
      data-testid={testId}
      data-call-id={call.call_id}
      open={defaultOpen || undefined}
      className="rounded-md border border-line-soft bg-canvas-soft"
    >
      <summary className="flex min-h-8.5 cursor-pointer list-none items-center gap-2 px-2.5 [&::-webkit-details-marker]:hidden">
        <ChevronRight
          className="size-3 shrink-0 text-muted transition-transform in-[details[open]]:rotate-90"
          aria-hidden="true"
        />
        <OwnerAvatar ownerKind="agent" ownerId={agentName} size="sm" />
        <span className="text-form text-muted">Asked</span>
        <span className="truncate text-small-body text-fg">{agentName}</span>
        <AgentCallStatePill state={state} fallbackLabel={call.state} />
        <AgentCallVerdictChip verdict={toCallVerdict(call.verdict)} />
        <AgentCallLiveness state={state} />
        <span className="flex-1" />
        <Time iso={call.updated_at} className="shrink-0 text-form text-muted" />
      </summary>

      <div className="flex flex-col gap-2 border-t border-line-soft px-2.5 py-2">
        {prompt ? <p className="text-small-body text-fg">{prompt}</p> : null}
        <div className="flex items-center gap-2">
          <span className="min-w-0 flex-1 truncate font-mono text-form text-muted">
            {resultGist(call)}
          </span>
          <button
            type="button"
            className="shrink-0 text-form text-accent underline-offset-4 hover:underline"
            onClick={() => onOpenCall(call.call_id)}
          >
            Open call
          </button>
        </div>
      </div>
    </details>
  );
}
