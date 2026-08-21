import type { QueryClient } from "@tanstack/react-query";
import { z } from "zod";

import { createStreamEventSource, type StreamEventSource } from "@/lib/ticketed-event-source";

import { profileKeys } from "./query-keys";

/**
 * Live delivery for profile lifecycle events.
 *
 * These ride the daemon's generic logs stream rather than a bespoke endpoint:
 * every `profile.*` event is already written durably and emitted there as a
 * NAMED event, filtered server-side by the event registry. Consuming it through
 * named listeners rather than `onmessage` keeps other families on the socket
 * from triggering profile refetches (L-017).
 */
export const PROFILE_SELECTION_CHANGED_EVENT = "profile.selection_changed";

export const PROFILE_LIFECYCLE_EVENTS = [
  "profile.created",
  "profile.renamed",
  "profile.identity_updated",
  "profile.archived",
  "profile.unarchived",
  "profile.deleted",
] as const;

export const PROFILE_EVENT_NAMES = [
  PROFILE_SELECTION_CHANGED_EVENT,
  ...PROFILE_LIFECYCLE_EVENTS,
] as const;

export type ProfileEventName = (typeof PROFILE_EVENT_NAMES)[number];

export type ProfileStreamStatus = "live" | "stale" | "disabled";

/**
 * The log envelope carries the profile payload in an untyped `content` field, so
 * it is parsed rather than trusted. A malformed frame is dropped, never
 * projected into the cache.
 */
const profileEventSchema = z.object({
  type: z.string(),
  content: z
    .object({
      name: z.string().optional(),
      profile_name: z.string().optional(),
    })
    .optional(),
});

export interface ProfileEvent {
  name: ProfileEventName;
  profileName: string;
}

export type ProfileEventSourceFactory = (url: string) => StreamEventSource;

export interface ProfileStreamHandlers {
  onProfileEvent: (event: ProfileEvent) => void;
  /** A reconnect may have missed frames, so `open` reconciles unconditionally. */
  onReconcile: () => void;
  onStatusChange: (status: ProfileStreamStatus) => void;
}

/**
 * Profile events are global scope and carry no workspace, and the daemon filters
 * `workspace_id` by exact match — passing one here would silently drop every
 * profile row. `replay=false` keeps a reconnect from re-announcing history.
 */
export function profileEventStreamUrl(): string {
  return "/api/logs/stream?component=profile&replay=false";
}

export function parseProfileEvent(event: Event): ProfileEvent | null {
  if (!(event instanceof MessageEvent) || typeof event.data !== "string") return null;
  try {
    const parsed = profileEventSchema.safeParse(JSON.parse(event.data));
    if (!parsed.success) return null;
    const name = parsed.data.content?.name?.trim() || parsed.data.type.trim();
    const eventName = PROFILE_EVENT_NAMES.find(candidate => candidate === name);
    if (eventName === undefined) return null;
    return { name: eventName, profileName: parsed.data.content?.profile_name?.trim() ?? "" };
  } catch {
    return null;
  }
}

export function openProfileEventStream(
  handlers: ProfileStreamHandlers,
  eventSourceFactory: ProfileEventSourceFactory = createStreamEventSource
): () => void {
  const handleOpen: EventListener = () => {
    handlers.onStatusChange("live");
    handlers.onReconcile();
  };
  const handleError: EventListener = () => handlers.onStatusChange("stale");
  const handleProfileEvent: EventListener = event => {
    const payload = parseProfileEvent(event);
    if (payload === null) return;
    handlers.onProfileEvent(payload);
  };

  const source = eventSourceFactory(profileEventStreamUrl());
  const detach = () => {
    source.removeEventListener("open", handleOpen);
    source.removeEventListener("error", handleError);
    for (const name of PROFILE_EVENT_NAMES) {
      source.removeEventListener(name, handleProfileEvent);
    }
  };
  try {
    source.addEventListener("open", handleOpen);
    source.addEventListener("error", handleError);
    for (const name of PROFILE_EVENT_NAMES) {
      source.addEventListener(name, handleProfileEvent);
    }
  } catch (error) {
    detach();
    source.close();
    throw error;
  }

  return () => {
    detach();
    source.close();
  };
}

/**
 * Applies one event to the cache.
 *
 * This deliberately touches query state only. The remembered choice is a
 * projection; refreshing it changes what a client would default to on next entry
 * and never what it is currently showing, so an operator switching profiles in
 * the terminal cannot yank an open browser out from under someone
 * (US-010.EC-4). Nothing in this path can reach the active-view store.
 */
export function applyProfileEvent(queryClient: QueryClient, event: ProfileEvent): void {
  void queryClient.invalidateQueries({ queryKey: profileKeys.selections() });
  if (event.name === PROFILE_SELECTION_CHANGED_EVENT) return;
  void queryClient.invalidateQueries({ queryKey: profileKeys.lists() });
  if (event.profileName !== "") {
    void queryClient.invalidateQueries({ queryKey: profileKeys.detail(event.profileName) });
  }
}

/** Full reconciliation after a (re)connect. */
export function reconcileProfiles(queryClient: QueryClient): void {
  void queryClient.invalidateQueries({ queryKey: profileKeys.all });
}
