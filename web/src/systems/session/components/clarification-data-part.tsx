import { useSessionRuntimeRenderContext } from "../hooks/use-session-runtime-render-context";
import { parseClarifyEvent } from "../lib/clarify-event";
import { interactionExpiredByRestart } from "../lib/session-pending-interactions";
import type { AgentEventPayload } from "../types";
import { ClarificationReceipt } from "./clarification-receipt";

export interface ClarificationDataPartProps {
  data: AgentEventPayload;
}

/**
 * Transcript record for a `clarify` data-compozy-event. A pending question
 * lives on the composer decision dock — the transcript keeps nothing until the
 * lifecycle turns terminal, then renders the one-line receipt from durable
 * evidence. A question the daemon expired at a restart never reached a terminal
 * lifecycle event, so its receipt comes from the durable interaction row.
 */
export function ClarificationDataPart({ data }: ClarificationDataPartProps) {
  const expiredInteractions = useSessionRuntimeRenderContext()?.expiredInteractions;
  const view = parseClarifyEvent(data);
  if (!view) {
    return null;
  }
  if (view.status !== "pending") {
    return <ClarificationReceipt view={view} />;
  }
  const expired = expiredInteractions?.get(view.requestId);
  if (expired && interactionExpiredByRestart(expired)) {
    return <ClarificationReceipt view={{ ...view, status: "canceled" }} cause="restart" />;
  }
  return null;
}
