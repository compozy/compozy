// @vitest-environment jsdom

import { QueryClient, QueryClientProvider, type InfiniteData } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createElement, type ReactNode } from "react";

import {
  networkDirectMessagesOptions,
  networkThreadMessagesOptions,
} from "../../lib/query-options";
import type {
  NetworkConversationMessage,
  NetworkDirectRoomMessagesResponse,
  NetworkMessagePageParam,
  NetworkThreadMessagesResponse,
} from "../../types";

const sendNetworkMessageMock = vi.fn();
const resolveNetworkDirectRoomMock = vi.fn();
const toastErrorMock = vi.fn();

vi.mock("../../adapters/network-api", async () => {
  const actual = await vi.importActual<typeof import("../../adapters/network-api")>(
    "../../adapters/network-api"
  );
  return {
    ...actual,
    sendNetworkMessage: (...args: unknown[]) => sendNetworkMessageMock(...args),
    resolveNetworkDirectRoom: (...args: unknown[]) => resolveNetworkDirectRoomMock(...args),
  };
});

vi.mock("@agh/ui", async () => {
  const actual = await vi.importActual<typeof import("@agh/ui")>("@agh/ui");
  return {
    ...actual,
    toast: {
      error: (...args: unknown[]) => toastErrorMock(...args),
    },
  };
});

vi.mock("@/systems/workspace", () => ({
  useActiveWorkspace: () => ({ activeWorkspaceId: "ws_alpha" }),
}));

import {
  THREAD_COLLISION_TOAST,
  useCreateNetworkThread,
  useResolveNetworkDirectRoom,
  useSendNetworkMessage,
} from "../use-network-actions";

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
  return { queryClient, wrapper };
}

function makeMessage(messageId: string, text: string): NetworkConversationMessage {
  return {
    body: { text },
    channel: "ops",
    direction: "sent",
    kind: "say",
    message_id: messageId,
    peer_from: "peer-self",
    text,
    timestamp: "2026-07-11T00:00:00Z",
  };
}

function makeMessageData<
  TPage extends NetworkThreadMessagesResponse | NetworkDirectRoomMessagesResponse,
>(pages: TPage[]): InfiniteData<TPage, NetworkMessagePageParam> {
  return {
    pages,
    pageParams: pages.map((_, index) => (index === 0 ? null : { before: `page-${index}` })),
  };
}

describe("useSendNetworkMessage", () => {
  beforeEach(() => {
    sendNetworkMessageMock.mockReset();
    toastErrorMock.mockReset();
  });

  it("Should update only the canonical tail, preserve older pages, and reconcile from the response", async () => {
    const { queryClient, wrapper } = createWrapper();
    const canonicalKey = networkThreadMessagesOptions("ws_alpha", "ops", "thread-1").queryKey;
    const filteredKey = networkThreadMessagesOptions("ws_alpha", "ops", "thread-1", {
      kind: "trace",
    }).queryKey;
    queryClient.setQueryData(
      canonicalKey,
      makeMessageData([
        {
          messages: [makeMessage("tail", "Newest persisted")],
          page: { has_more: true, limit: 120, next_cursor: "older-cursor" },
        },
        {
          messages: [makeMessage("older", "Older history")],
          page: { has_more: false, limit: 120 },
        },
      ])
    );
    queryClient.setQueryData(
      filteredKey,
      makeMessageData([
        {
          messages: [makeMessage("trace-only", "Filtered history")],
          page: { has_more: false, limit: 120 },
        },
      ])
    );
    sendNetworkMessageMock.mockResolvedValue({
      message: {
        id: "server-id",
        kind: "say",
        channel: "ops",
        session_id: "sess-1",
        surface: "thread",
        thread_id: "thread-1",
      },
    });

    const { result } = renderHook(() => useSendNetworkMessage(), { wrapper });

    let promise: Promise<unknown> | null = null;
    await act(async () => {
      promise = result.current.send({
        surface: "thread",
        channel: "ops",
        threadId: "thread-1",
        sessionId: "sess-1",
        peerFrom: "peer-self",
        text: "Hello world",
      });
      // Allow the mutationFn microtask to run so the optimistic cache update
      // becomes observable before we assert against it.
      await Promise.resolve();
    });

    const cacheAfterMutate =
      queryClient.getQueryData<
        InfiniteData<NetworkThreadMessagesResponse, NetworkMessagePageParam>
      >(canonicalKey);
    expect(cacheAfterMutate).toBeDefined();
    expect(cacheAfterMutate?.pages[0]?.messages.at(-1)?.text).toBe("Hello world");
    expect(cacheAfterMutate?.pages[1]?.messages[0]?.message_id).toBe("older");

    await act(async () => {
      await promise;
    });

    const cacheAfterSuccess =
      queryClient.getQueryData<
        InfiniteData<NetworkThreadMessagesResponse, NetworkMessagePageParam>
      >(canonicalKey);
    expect(cacheAfterSuccess?.pages).toHaveLength(2);
    const replaced = cacheAfterSuccess?.pages[0]?.messages.at(-1) as NetworkConversationMessage & {
      optimistic?: string;
    };
    expect(replaced.message_id).toBe("server-id");
    expect(replaced.thread_id).toBe("thread-1");
    expect(replaced?.optimistic).toBeUndefined();
    expect(
      queryClient.getQueryData<
        InfiniteData<NetworkThreadMessagesResponse, NetworkMessagePageParam>
      >(filteredKey)
    ).toEqual(
      makeMessageData([
        {
          messages: [makeMessage("trace-only", "Filtered history")],
          page: { has_more: false, limit: 120 },
        },
      ])
    );
  });

  it("Should mark the optimistic message as failed when the send rejects", async () => {
    const { queryClient, wrapper } = createWrapper();
    const key = networkThreadMessagesOptions("ws_alpha", "ops", "thread-1").queryKey;
    queryClient.setQueryData(
      key,
      makeMessageData([
        { messages: [], page: { has_more: false, limit: 120 } },
      ] satisfies NetworkThreadMessagesResponse[])
    );
    sendNetworkMessageMock.mockRejectedValue(new Error("boom"));

    const { result } = renderHook(() => useSendNetworkMessage(), { wrapper });

    await expect(
      act(() =>
        result.current.send({
          surface: "thread",
          channel: "ops",
          threadId: "thread-1",
          sessionId: "sess-1",
          peerFrom: "peer-self",
          text: "Hello world",
        })
      )
    ).rejects.toThrow("boom");

    const cache =
      queryClient.getQueryData<
        InfiniteData<NetworkThreadMessagesResponse, NetworkMessagePageParam>
      >(key);
    const failed = cache?.pages[0]?.messages[0] as NetworkConversationMessage & {
      optimistic?: string;
    };
    expect(failed?.optimistic).toBe("failed");
  });

  it("Should never construct a request body containing kind:'direct'", async () => {
    const { queryClient, wrapper } = createWrapper();
    const key = networkDirectMessagesOptions("ws_alpha", "ops", "direct-1").queryKey;
    queryClient.setQueryData(
      key,
      makeMessageData([
        { messages: [], page: { has_more: false, limit: 120 } },
      ] satisfies NetworkDirectRoomMessagesResponse[])
    );
    sendNetworkMessageMock.mockResolvedValue({
      message: { id: "x", kind: "say", channel: "ops", session_id: "sess-1" },
    });

    const { result } = renderHook(() => useSendNetworkMessage(), { wrapper });
    await act(async () => {
      await result.current.send({
        surface: "direct",
        channel: "ops",
        directId: "direct-1",
        sessionId: "sess-1",
        peerFrom: "peer-self",
        text: "secret",
      });
    });

    expect(sendNetworkMessageMock).toHaveBeenCalledTimes(1);
    const sent = sendNetworkMessageMock.mock.calls[0]?.[1] as Record<string, unknown>;
    expect(sendNetworkMessageMock.mock.calls[0]?.[0]).toBe("ws_alpha");
    expect(sent.kind).toBe("say");
    expect(sent.surface).toBe("direct");
    expect(sent.direct_id).toBe("direct-1");
    expect(sent).not.toHaveProperty("interaction_id");
  });

  it("Should drop the optimistic message when discard is invoked", async () => {
    const { queryClient, wrapper } = createWrapper();
    const key = networkThreadMessagesOptions("ws_alpha", "ops", "thread-1").queryKey;
    queryClient.setQueryData(
      key,
      makeMessageData([
        { messages: [], page: { has_more: false, limit: 120 } },
      ] satisfies NetworkThreadMessagesResponse[])
    );
    sendNetworkMessageMock.mockRejectedValue(new Error("boom"));

    const { result } = renderHook(() => useSendNetworkMessage(), { wrapper });
    let failedId = "";
    await act(async () => {
      try {
        await result.current.send({
          surface: "thread",
          channel: "ops",
          threadId: "thread-1",
          sessionId: "sess-1",
          peerFrom: "peer-self",
          text: "Hello",
        });
      } catch {
        // expected
      }
    });

    const cache =
      queryClient.getQueryData<
        InfiniteData<NetworkThreadMessagesResponse, NetworkMessagePageParam>
      >(key);
    failedId = cache?.pages[0]?.messages[0]?.message_id ?? "";
    expect(failedId).toBeTruthy();

    act(() => {
      result.current.discard(
        {
          surface: "thread",
          channel: "ops",
          threadId: "thread-1",
          sessionId: "sess-1",
          peerFrom: "peer-self",
          text: "Hello",
        },
        failedId
      );
    });

    const after =
      queryClient.getQueryData<
        InfiniteData<NetworkThreadMessagesResponse, NetworkMessagePageParam>
      >(key);
    expect(after?.pages[0]?.messages).toEqual([]);
  });
});

describe("useCreateNetworkThread", () => {
  beforeEach(() => {
    sendNetworkMessageMock.mockReset();
    toastErrorMock.mockReset();
  });
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("Should retry exactly once when the first attempt fails", async () => {
    const { wrapper } = createWrapper();
    sendNetworkMessageMock.mockRejectedValueOnce(new Error("collision"));
    sendNetworkMessageMock.mockResolvedValueOnce({
      message: { id: "second", kind: "say", channel: "ops", session_id: "sess-1" },
    });

    const { result } = renderHook(() => useCreateNetworkThread(), { wrapper });

    let outcome: { threadId: string; rootMessageId: string } | null = null;
    await act(async () => {
      outcome = await result.current.createThread({
        channel: "ops",
        sessionId: "sess-1",
        peerFrom: "peer-self",
        text: "Open this thread",
      });
    });

    expect(sendNetworkMessageMock).toHaveBeenCalledTimes(2);
    const finalOutcome = outcome as { threadId: string; rootMessageId: string } | null;
    expect(finalOutcome?.threadId.startsWith("thread_")).toBe(true);
    const firstCall = sendNetworkMessageMock.mock.calls[0] ?? [];
    const secondCall = sendNetworkMessageMock.mock.calls[1] ?? [];
    const firstThreadId = (firstCall[1] as { thread_id: string }).thread_id;
    const secondThreadId = (secondCall[1] as { thread_id: string }).thread_id;
    expect(firstThreadId).not.toBe(secondThreadId);
    expect(toastErrorMock).not.toHaveBeenCalled();
  });

  it("Should surface a single Sonner toast after the second collision", async () => {
    const { wrapper } = createWrapper();
    sendNetworkMessageMock.mockRejectedValue(new Error("collision again"));

    const { result } = renderHook(() => useCreateNetworkThread(), { wrapper });

    await expect(
      act(() =>
        result.current.createThread({
          channel: "ops",
          sessionId: "sess-1",
          peerFrom: "peer-self",
          text: "Open this thread",
        })
      )
    ).rejects.toBeTruthy();

    expect(sendNetworkMessageMock).toHaveBeenCalledTimes(2);
    expect(toastErrorMock).toHaveBeenCalledTimes(1);
    expect(toastErrorMock).toHaveBeenCalledWith(THREAD_COLLISION_TOAST);
  });
});

describe("useResolveNetworkDirectRoom", () => {
  beforeEach(() => {
    resolveNetworkDirectRoomMock.mockReset();
  });

  it("Should call the resolve adapter with the channel and body", async () => {
    const { wrapper } = createWrapper();
    resolveNetworkDirectRoomMock.mockResolvedValue({
      channel: "ops",
      direct_id: "direct-x",
      session_a: "sess-1",
      session_b: "sess-2",
      message_count: 0,
      open_work_count: 0,
    });

    const { result } = renderHook(() => useResolveNetworkDirectRoom(), { wrapper });
    type Resolved = { direct_id: string };
    let resolved: Resolved | null = null;
    await act(async () => {
      resolved = (await result.current.resolveRoom({
        channel: "ops",
        body: { peer_id: "a", session_id: "sess-1" },
      })) as Resolved;
    });

    await waitFor(() => expect(result.current.isResolving).toBe(false));
    expect(resolveNetworkDirectRoomMock).toHaveBeenCalledWith("ws_alpha", "ops", {
      peer_id: "a",
      session_id: "sess-1",
    });
    expect((resolved as Resolved | null)?.direct_id).toBe("direct-x");
  });
});
