import { Activity } from "lucide-react";

import { MessageMarkdown } from "@/systems/session/components/message-markdown";
import { Marker, MarkerMeta } from "@compozy/ui";

export function SessionMessageText({
  text,
  streaming = false,
}: {
  text: string;
  streaming?: boolean;
}) {
  return <MessageMarkdown content={text} streaming={streaming} />;
}

/**
 * Unrecognized data event as a one-line marker: the raw event name is the mono
 * meta — never a pill, never a payload-preview card. The full payload stays
 * inspectable through the session inspector, not the transcript.
 */
export function SessionDataEventCard({ name }: { name: string; data: unknown }) {
  return (
    <Marker data-testid="session-data-part" icon={<Activity strokeWidth={1.8} />}>
      <b>Data event</b> <MarkerMeta>{name}</MarkerMeta>
    </Marker>
  );
}
