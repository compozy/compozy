import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import { sessionKeys } from "@/systems/session";

import { createNetworkChannel } from "../../adapters/network-api";
import { networkKeys } from "../../lib/query-keys";
import type { CreateNetworkChannelRequest, CreateNetworkChannelResponse } from "../../types";
import { useCreateNetworkChannel } from "../use-network-channel-actions";

vi.mock("../../adapters/network-api", async importOriginal => {
  const actual = await importOriginal<typeof import("../../adapters/network-api")>();
  return { ...actual, createNetworkChannel: vi.fn() };
});

vi.mock("@/systems/workspace", () => ({
  useActiveWorkspace: () => ({ activeWorkspaceId: "ws_active" }),
}));

function wrapper(queryClient: QueryClient) {
  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

describe("useCreateNetworkChannel", () => {
  it("Should invalidate the same explicit workspace used by the mutation", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { mutations: { retry: false }, queries: { retry: false } },
    });
    const input: CreateNetworkChannelRequest = {
      agent_names: [],
      channel: "deployments",
      purpose: "Coordinate deployment checks",
      workspace_id: "ws_payload",
    };
    vi.mocked(createNetworkChannel).mockResolvedValue({
      channel: { channel: input.channel },
    } as CreateNetworkChannelResponse);

    for (const workspaceId of ["ws_explicit", "ws_payload"]) {
      queryClient.setQueryData(networkKeys.channelsRoot(workspaceId), []);
      queryClient.setQueryData(sessionKeys.list({ workspace: workspaceId }), []);
    }

    const { result } = renderHook(() => useCreateNetworkChannel({ workspaceId: "ws_explicit" }), {
      wrapper: wrapper(queryClient),
    });

    await act(async () => {
      await result.current.mutateAsync(input);
    });

    expect(createNetworkChannel).toHaveBeenCalledWith("ws_explicit", input);
    expect(queryClient.getQueryState(networkKeys.channelsRoot("ws_explicit"))?.isInvalidated).toBe(
      true
    );
    expect(
      queryClient.getQueryState(sessionKeys.list({ workspace: "ws_explicit" }))?.isInvalidated
    ).toBe(true);
    expect(queryClient.getQueryState(networkKeys.channelsRoot("ws_payload"))?.isInvalidated).toBe(
      false
    );
    expect(
      queryClient.getQueryState(sessionKeys.list({ workspace: "ws_payload" }))?.isInvalidated
    ).toBe(false);
  });
});
