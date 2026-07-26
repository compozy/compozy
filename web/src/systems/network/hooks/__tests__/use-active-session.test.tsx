// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { createElement, type ReactNode } from "react";

import type { NetworkChannel } from "../../types";

const getNetworkChannelMock = vi.fn();

vi.mock("../../adapters/network-api", async () => {
  const actual = await vi.importActual<typeof import("../../adapters/network-api")>(
    "../../adapters/network-api"
  );
  return {
    ...actual,
    getNetworkChannel: (...args: unknown[]) => getNetworkChannelMock(...args),
  };
});

import { useActiveNetworkSession } from "../use-active-session";

const WORKSPACE_ID = "ws";

function makeWrapper() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client }, children);
}

function buildChannel(overrides: Partial<NetworkChannel> = {}): NetworkChannel {
  return {
    channel: "ops",
    created_at: "2026-04-17T14:00:00Z",
    last_activity_at: "2026-04-17T18:00:00Z",
    last_message_preview: "",
    local_peer_count: 1,
    message_count: 0,
    peer_count: 1,
    purpose: "test",
    session_count: 1,
    workspace_id: "ws",
    peers: [],
    sessions: [],
    ...overrides,
  } as NetworkChannel;
}

describe("useActiveNetworkSession", () => {
  it("Should return disabled with reason when no channel is provided", () => {
    const { result } = renderHook(() => useActiveNetworkSession(null), { wrapper: makeWrapper() });
    expect(result.current.session).toBeNull();
    expect(result.current.disabledReason).toBeTruthy();
  });

  it("Should resolve to the first local peer once the channel detail loads", async () => {
    getNetworkChannelMock.mockResolvedValue(
      buildChannel({
        peers: [
          {
            channel: "ops",
            display_name: "Codex",
            joined_at: "2026-04-17T14:00:00Z",
            local: true,
            peer_card: {
              peer_id: "peer-self",
              display_name: "Codex",
              capabilities: [],
              artifacts_supported: [],
              profiles_supported: [],
              trust_modes_supported: [],
            },
            peer_id: "peer-self",
            presence_state: "local",
            session_id: "sess-1",
          },
        ],
      })
    );

    const { result } = renderHook(
      () => useActiveNetworkSession("ops", { workspaceId: WORKSPACE_ID }),
      { wrapper: makeWrapper() }
    );
    await waitFor(() => expect(result.current.session).not.toBeNull());
    expect(result.current.session?.peerId).toBe("peer-self");
    expect(result.current.session?.sessionId).toBe("sess-1");
    expect(result.current.disabledReason).toBeNull();
    expect(getNetworkChannelMock).toHaveBeenCalledWith(
      WORKSPACE_ID,
      "ops",
      expect.any(AbortSignal)
    );
  });

  it("Should expose a disabled reason when the channel has no local peer", async () => {
    getNetworkChannelMock.mockResolvedValue(buildChannel({ peers: [] }));
    const { result } = renderHook(
      () => useActiveNetworkSession("ops", { workspaceId: WORKSPACE_ID }),
      { wrapper: makeWrapper() }
    );
    await waitFor(() => expect(result.current.disabledReason).not.toBeNull());
    expect(result.current.session).toBeNull();
  });
});
