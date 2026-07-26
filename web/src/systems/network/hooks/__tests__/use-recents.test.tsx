// @vitest-environment jsdom

import { renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { NetworkChannelsResponse } from "../../types";
import { useNetworkRecents } from "../use-recents";

vi.mock("@/systems/workspace", () => ({
  useActiveWorkspace: () => ({ activeWorkspaceId: "stale-workspace" }),
}));

const embedded: NonNullable<NetworkChannelsResponse["recents"]> = [
  {
    channel: "ops",
    container_id: "direct_ops_one",
    last_activity_at: "2026-04-17T18:30:00Z",
    last_message_preview: "ops dm preview",
    session_a: "session-a",
    session_b: "session-b",
    surface: "direct",
  },
  {
    channel: "ops",
    container_id: "thread_ops_one",
    last_activity_at: "2026-04-17T18:16:00Z",
    last_message_preview: "thread preview ops",
    participant_count: 4,
    surface: "thread",
    title: "Ops thread",
  },
  {
    channel: "design",
    container_id: "thread_design_one",
    last_activity_at: "2026-04-17T17:00:00Z",
    participant_count: 2,
    surface: "thread",
  },
  {
    channel: "design",
    container_id: "direct_design_one",
    last_activity_at: "2026-04-17T17:00:00Z",
    session_a: "session-x",
    session_b: "session-y",
    surface: "direct",
  },
  {
    channel: "ops",
    container_id: "direct_ops_two",
    last_activity_at: "2026-04-17T15:00:00Z",
    session_a: "session-c",
    session_b: "session-d",
    surface: "direct",
  },
];

describe("useNetworkRecents", () => {
  beforeEach(() => window.localStorage.clear());
  afterEach(() => window.localStorage.clear());

  it("preserves the server projection order, including timestamp ties", () => {
    const { result } = renderHook(() =>
      useNetworkRecents(embedded, { workspaceId: "w1", limit: 5 })
    );
    expect(result.current.recents.map(entry => entry.containerId)).toEqual([
      "direct_ops_one",
      "thread_ops_one",
      "thread_design_one",
      "direct_design_one",
      "direct_ops_two",
    ]);
    expect(result.current.isLoading).toBe(false);
  });

  it("caps the embedded projection defensively without issuing per-channel reads", () => {
    const { result } = renderHook(() =>
      useNetworkRecents(embedded, { workspaceId: "w1", limit: 2 })
    );
    expect(result.current.recents.map(entry => entry.containerId)).toEqual([
      "direct_ops_one",
      "thread_ops_one",
    ]);
  });

  it("uses the explicit workspace when comparing the local last-read marker", () => {
    window.localStorage.setItem(
      "network:last-read",
      JSON.stringify({ "w1:ops:thread:thread_ops_one": "2026-04-17T18:00:00Z" })
    );
    const { result } = renderHook(() =>
      useNetworkRecents(embedded, { workspaceId: "w1", limit: 5 })
    );
    expect(
      result.current.recents.find(entry => entry.containerId === "thread_ops_one")?.hasUnread
    ).toBe(true);
  });
});
