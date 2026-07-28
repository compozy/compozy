// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";

import { networkThreadMessagesOptions } from "../../lib/query-options";

const listThreadMessages = vi.hoisted(() => vi.fn());

vi.mock("../../adapters/network-api", async () => ({
  ...(await vi.importActual("../../adapters/network-api")),
  listNetworkThreadMessages: (...args: unknown[]) => listThreadMessages(...args),
}));

import { useNetworkMessageTail } from "../use-network-message-tail";

function message(id: string) {
  return {
    body: {},
    channel: "ops",
    direction: "received",
    kind: "say",
    message_id: id,
    peer_from: "peer",
    timestamp: "2026-07-11T00:01:00Z",
  };
}

describe("useNetworkMessageTail", () => {
  beforeEach(() => {
    listThreadMessages.mockReset();
    listThreadMessages.mockResolvedValue({
      messages: [message("msg-a")],
      page: { has_more: false, limit: 120 },
    });
  });

  it("Should poll only after the canonical tail cursor and merge into page zero", async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const key = networkThreadMessagesOptions("ws-url", "ops", "thread-url").queryKey;
    queryClient.setQueryData(key, {
      pages: [
        {
          messages: [message("msg-z"), message("msg-y")],
          page: { has_more: true, limit: 120, next_cursor: "older" },
        },
        {
          messages: [message("msg-old")],
          page: { has_more: false, limit: 120 },
        },
      ],
      pageParams: [null, { before: "older" }],
    });
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );

    renderHook(
      () =>
        useNetworkMessageTail({
          workspaceId: "ws-url",
          channel: "ops",
          surface: "thread",
          containerId: "thread-url",
          filters: {},
          enabled: true,
        }),
      { wrapper }
    );

    await waitFor(() => {
      expect(listThreadMessages).toHaveBeenCalledWith(
        "ws-url",
        "ops",
        "thread-url",
        { after: "msg-y", limit: 120 },
        expect.any(AbortSignal)
      );
    });
    await waitFor(() => {
      const data = queryClient.getQueryData(key);
      expect(data?.pages[0]?.messages.map(item => item.message_id)).toEqual([
        "msg-z",
        "msg-y",
        "msg-a",
      ]);
      expect(data?.pages[1]?.messages.map(item => item.message_id)).toEqual(["msg-old"]);
    });
  });
});
