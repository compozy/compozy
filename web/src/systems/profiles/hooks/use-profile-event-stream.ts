import { useEffect, useEffectEvent, useRef, useState } from "react";
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
  /**
   * Runs after a `profile.renamed` event, for caches outside this system, and
   * after every reconnect: `replay=false` means a rename made while the socket
   * was down never arrives as an event (#1220).
   */
  onProfileRenamed?: () => void;
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
  onProfileRenamed,
}: UseProfileEventStreamOptions = {}): ProfileStreamStatus {
  const queryClient = useQueryClient();
  const onProfileEvent = useEffectEvent((event: ProfileEvent) => {
    applyProfileEvent(queryClient, event);
    if (event.name === "profile.renamed") onProfileRenamed?.();
  });
  // The first open has nothing to catch up on; every later one may have missed a rename.
  const connectedBefore = useRef(false);
  const onReconcile = useEffectEvent(() => {
    reconcileProfiles(queryClient);
    if (connectedBefore.current) onProfileRenamed?.();
    connectedBefore.current = true;
  });

  const connectable = enabled && (Boolean(eventSourceFactory) || typeof WebSocket !== "undefined");
  const [connection, setConnection] = useState<"live" | "stale">("stale");

  useEffect(() => {
    if (!connectable) return undefined;
    const close = openProfileEventStream(
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
    return () => {
      close();
      setConnection("stale");
    };
  }, [connectable, eventSourceFactory]);

  return connectable ? connection : "disabled";
}
