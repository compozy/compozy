// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { createElement, type ReactNode } from "react";

import type { NetworkPeerSummary } from "../../types";

const listNetworkPeersMock = vi.fn();

vi.mock("../../adapters/network-api", async original => {
  const actual = (await original()) as Record<string, unknown>;
  return {
    ...actual,
    listNetworkPeers: (...args: unknown[]) => listNetworkPeersMock(...args),
  };
});

import { useChannelMembers } from "../use-channel-members";

const WORKSPACE_ID = "ws";

function buildPeer(overrides: Partial<NetworkPeerSummary> & Pick<NetworkPeerSummary, "peer_id">) {
  return {
    channel: "ops",
    display_name: "",
    local: true,
    peer_card: {
      peer_id: overrides.peer_id,
      profiles_supported: [],
      capabilities: [],
      artifacts_supported: [],
      trust_modes_supported: [],
    },
    presence_state: "local",
    ...overrides,
  } as NetworkPeerSummary;
}

function createWrapper() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

describe("useChannelMembers", () => {
  it("Should classify peers by session_id presence and surface aggregate counts", async () => {
    listNetworkPeersMock.mockResolvedValueOnce([
      buildPeer({ peer_id: "agent-a", session_id: "session-1", presence_state: "local" }),
      buildPeer({ peer_id: "agent-b", session_id: "session-2", presence_state: "local" }),
      buildPeer({ peer_id: "human-a", presence_state: "local" }),
    ]);

    const { result } = renderHook(() => useChannelMembers("ops", { workspaceId: WORKSPACE_ID }), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.agentCount).toBe(2);
    expect(result.current.humanCount).toBe(1);
    expect(result.current.members.map(member => member.peerId)).toEqual([
      "agent-a",
      "agent-b",
      "human-a",
    ]);
    expect(result.current.members[0].role).toBe("agent");
    expect(result.current.members[1].presenceState).toBe("local");
    expect(result.current.members[2].role).toBe("human");
    expect(result.current.members[2].presenceState).toBe("local");
    expect(listNetworkPeersMock).toHaveBeenCalledWith(WORKSPACE_ID, "ops", expect.any(AbortSignal));
  });

  it("Should stay disabled and report zero counts when no channel is supplied", () => {
    const { result } = renderHook(() => useChannelMembers(null), { wrapper: createWrapper() });

    expect(result.current.members).toEqual([]);
    expect(result.current.agentCount).toBe(0);
    expect(result.current.humanCount).toBe(0);
    expect(result.current.isLoading).toBe(false);
  });
});
