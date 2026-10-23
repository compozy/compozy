import { Activity } from "lucide-react";

import { MessageMarkdown } from "@/systems/session/components/message-markdown";
import { Marker, MarkerMeta, type StreamMarkdownSlots } from "@compozy/ui";

export function SessionMessageText({
  text,
  streaming = false,
  reveal = false,
  slots,
  userLinks = false,
}: {
  text: string;
  streaming?: boolean;
  /** Assistant prose: smooth reveal while streaming (ADR-008). */
  reveal?: boolean;
  /** Inline nodes `text` places with `composeMarkdownSlots`. */
  slots?: StreamMarkdownSlots;
  userLinks?: boolean;
}) {
  return (
    <MessageMarkdown
      content={text}
      streaming={streaming}
      reveal={reveal}
      slots={slots}
      userLinks={userLinks}
    />
  );
}

/**
 * Unrecognized data event as a one-line marker: the raw event name is the mono
 * meta — never a pill, never a payload-preview card. The full payload stays
 * inspectable through the session inspector, not the transcript.
 */
export function SessionDataEventMarker({ name }: { name: string }) {
  return (
    <Marker data-testid="session-data-part" icon={<Activity />}>
      <b>Data event</b> <MarkerMeta>{name}</MarkerMeta>
    </Marker>
  );
}
