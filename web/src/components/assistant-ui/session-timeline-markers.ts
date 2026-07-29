// Cluster identity for consecutive runtime markers. Only agent-event payloads
// cluster — never clarification lifecycles, permission records, or goal prompts
// (interactive/receipt surfaces stay individual rows). The key is the event type
// plus the marker/failure kind string so only same-kind neighbors merge.

import { CLARIFY_EVENT_TYPE } from "@/systems/session/lib/clarify-event";
import { isAgentEventPayload } from "@/systems/session/lib/message-parts";

import type { SessionTimelineDataPart } from "./session-timeline.logic";

export function markerClusterKey(part: SessionTimelineDataPart): string | null {
  if (part.name !== "data-compozy-event") return null;
  const data = part.data;
  if (!isAgentEventPayload(data)) return null;
  if (data.type === CLARIFY_EVENT_TYPE) return null;
  if (data.goal) return null;
  const kind = data.marker?.kind ?? data.failure?.kind ?? "";
  return `${data.type}:${kind}`;
}
