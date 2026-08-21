// Suite: profile live-delivery seam
// Invariant: a profile event refreshes the remembered-choice projection and, for lifecycle
// events, the list and the row — and it never moves this client's active view, so an
// operator switching profiles elsewhere cannot yank an open client (US-010.EC-4).
// Boundary IN: the stream URL, the frame parser, and the cache application.
// Boundary OUT: the daemon's own filtering (Go stream suites) and the React lifecycle
// (use-profile-event-stream is exercised through this seam).

import { QueryClient } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  applyProfileEvent,
  openProfileEventStream,
  parseProfileEvent,
  PROFILE_EVENT_NAMES,
  PROFILE_SELECTION_CHANGED_EVENT,
  profileEventStreamUrl,
} from "../profile-event-stream";
import { profileKeys } from "../query-keys";
import {
  localProfileView,
  profileViewStore,
  resetProfileViews,
  setProfileView,
} from "../../stores/profile-view-store";

function frame(payload: unknown): MessageEvent {
  return new MessageEvent("message", { data: JSON.stringify(payload) });
}

function selectionFrame(profileName = "marketing") {
  return frame({
    type: PROFILE_SELECTION_CHANGED_EVENT,
    content: {
      name: PROFILE_SELECTION_CHANGED_EVENT,
      profile_name: profileName,
    },
  });
}

describe("profileEventStreamUrl", () => {
  it("Should filter server-side by component and skip replay", () => {
    expect(profileEventStreamUrl()).toBe("/api/logs/stream?component=profile&replay=false");
  });

  it("Should never scope by workspace, because profile events are global", () => {
    // The daemon filters workspace_id by exact match, so a workspace here would
    // silently drop every profile row.
    expect(profileEventStreamUrl()).not.toContain("workspace_id");
  });
});

describe("parseProfileEvent", () => {
  it("Should read the name and profile from the snake_case payload", () => {
    expect(parseProfileEvent(selectionFrame())).toEqual({
      name: PROFILE_SELECTION_CHANGED_EVENT,
      profileName: "marketing",
    });
  });

  it("Should fall back to the envelope type when content omits the name", () => {
    expect(parseProfileEvent(frame({ type: "profile.archived" }))).toEqual({
      name: "profile.archived",
      profileName: "",
    });
  });

  it("Should drop a malformed frame rather than project it", () => {
    expect(parseProfileEvent(new MessageEvent("message", { data: "not json" }))).toBeNull();
    expect(parseProfileEvent(frame({ nope: true }))).toBeNull();
    expect(parseProfileEvent(frame({ type: "profile.unknown" }))).toBeNull();
    expect(parseProfileEvent(new Event("message"))).toBeNull();
  });
});

describe("openProfileEventStream", () => {
  it("Should listen by event name for every profile event, never onmessage", () => {
    const listeners = new Map<string, EventListener>();
    const addEventListener = vi.fn((name: string, listener: EventListener) => {
      listeners.set(name, listener);
    });
    const removeEventListener = vi.fn();
    const close = vi.fn();
    const onProfileEvent = vi.fn();
    const teardown = openProfileEventStream(
      { onProfileEvent, onReconcile: vi.fn(), onStatusChange: vi.fn() },
      () => ({ addEventListener, removeEventListener, close }) as never
    );
    const listened = addEventListener.mock.calls.map(call => call[0]);
    for (const name of PROFILE_EVENT_NAMES) expect(listened).toContain(name);
    expect(listened).toContain("open");
    expect(listened).not.toContain("message");

    listeners.get("profile.archived")?.(
      frame({
        type: "profile.archived",
        content: { name: "profile.archived", profile_name: "marketing" },
      })
    );
    expect(onProfileEvent).toHaveBeenCalledExactlyOnceWith({
      name: "profile.archived",
      profileName: "marketing",
    });

    teardown();
    expect(close).toHaveBeenCalledOnce();
    expect(removeEventListener).toHaveBeenCalledTimes(addEventListener.mock.calls.length);
  });

  it("Should reconcile on open, because a reconnect may have missed frames", () => {
    const listeners = new Map<string, EventListener>();
    const onReconcile = vi.fn();
    const onStatusChange = vi.fn();
    openProfileEventStream(
      { onProfileEvent: vi.fn(), onReconcile, onStatusChange },
      () =>
        ({
          addEventListener: (name: string, listener: EventListener) =>
            listeners.set(name, listener),
          removeEventListener: () => {},
          close: () => {},
        }) as never
    );
    listeners.get("open")?.(new Event("open"));
    expect(onReconcile).toHaveBeenCalledOnce();
    expect(onStatusChange).toHaveBeenCalledWith("live");

    listeners.get("error")?.(new Event("error"));
    expect(onStatusChange).toHaveBeenLastCalledWith("stale");
  });
});

describe("applyProfileEvent", () => {
  beforeEach(() => {
    resetProfileViews();
  });

  it("Should refresh the remembered choice on a selection change", () => {
    const queryClient = new QueryClient();
    const invalidate = vi.spyOn(queryClient, "invalidateQueries").mockResolvedValue(undefined);
    applyProfileEvent(queryClient, {
      name: PROFILE_SELECTION_CHANGED_EVENT,
      profileName: "marketing",
    });
    expect(invalidate).toHaveBeenCalledExactlyOnceWith({ queryKey: profileKeys.selections() });
  });

  it("Should leave the active view untouched when the switch happened elsewhere", () => {
    const queryClient = new QueryClient();
    vi.spyOn(queryClient, "invalidateQueries").mockResolvedValue(undefined);
    const lens = { scope: "workspace", workspaceId: "ws-acme" } as const;
    setProfileView(lens, { kind: "profile", profile: "consulting" });
    const before = profileViewStore.getSnapshot().context;

    applyProfileEvent(queryClient, {
      name: PROFILE_SELECTION_CHANGED_EVENT,
      profileName: "marketing",
    });

    // The remembered choice moved; what this client is looking at did not.
    expect(profileViewStore.getSnapshot().context).toEqual(before);
    expect(localProfileView(lens)).toEqual({ kind: "profile", profile: "consulting" });
  });

  it("Should refresh the list and the row for a lifecycle event", () => {
    const queryClient = new QueryClient();
    const invalidate = vi.spyOn(queryClient, "invalidateQueries").mockResolvedValue(undefined);
    applyProfileEvent(queryClient, { name: "profile.archived", profileName: "old-agency" });
    expect(invalidate.mock.calls.map(([options]) => options)).toEqual([
      { queryKey: profileKeys.selections() },
      { queryKey: profileKeys.lists() },
      { queryKey: profileKeys.detail("old-agency") },
    ]);
  });
});
