import { useEffect, useEffectEvent, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";

import {
  applyProfileEvent,
  openProfileEventStream,
  reconcileProfiles,
  type ProfileEvent,
  type ProfileEventSourceFactory,
  type ProfileStreamStatus,
} from "../lib/profile-event-stream";

export interface UseProfileEventStreamOptions {
  enabled?: boolean;
  eventSourceFactory?: ProfileEventSourceFactory;
}

/**
 * Keeps the profile projections live.
 *
 * A `compozy profile use` in a terminal, or a switch in a second browser, lands
 * here and refreshes the remembered choice. It never moves this client's active
 * view — the handler can only reach the query cache (US-010.EC-4).
 */
export function useProfileEventStream({
  enabled = true,
  eventSourceFactory,
}: UseProfileEventStreamOptions = {}): ProfileStreamStatus {
  const queryClient = useQueryClient();
  // Only the connection's own health lives in state; "disabled" is a fact about
  // the caller and is derived, so nothing has to be written back during render.
  const [connection, setConnection] = useState<"live" | "stale">("stale");

  const onProfileEvent = useEffectEvent((event: ProfileEvent) => {
    applyProfileEvent(queryClient, event);
  });
  const onReconcile = useEffectEvent(() => reconcileProfiles(queryClient));

  const connectable =
    enabled && (Boolean(eventSourceFactory) || typeof EventSource !== "undefined");

  useEffect(() => {
    if (!connectable) return undefined;
    return openProfileEventStream(
      {
        onProfileEvent,
        onReconcile,
        // The stream only ever reports connection health; "disabled" is this
        // hook's own answer and never arrives from the socket.
        onStatusChange: status => {
          if (status !== "disabled") setConnection(status);
        },
      },
      eventSourceFactory
    );
  }, [connectable, eventSourceFactory]);

  return connectable ? connection : "disabled";
}
