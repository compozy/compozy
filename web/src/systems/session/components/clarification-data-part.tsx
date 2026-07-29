import { parseClarifyEvent } from "../lib/clarify-event";
import type { AgentEventPayload } from "../types";
import { ClarificationReceipt } from "./clarification-receipt";

export interface ClarificationDataPartProps {
  data: AgentEventPayload;
}

/**
 * Transcript record for a `clarify` data-compozy-event. A pending question
 * lives on the composer decision dock — the transcript keeps nothing until the
 * lifecycle turns terminal, then renders the one-line receipt from durable
 * evidence.
 */
export function ClarificationDataPart({ data }: ClarificationDataPartProps) {
  const view = parseClarifyEvent(data);
  if (!view) {
    return null;
  }
  if (view.status === "pending") {
    return null;
  }
  return <ClarificationReceipt view={view} />;
}
