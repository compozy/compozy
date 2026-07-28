import { QueryClient, type QueryFunctionContext } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  listNetworkThreadMessages: vi.fn().mockResolvedValue({
    messages: [],
    page: { has_more: false, limit: 120 },
  }),
  listNetworkDirectRoomMessages: vi.fn().mockResolvedValue({
    messages: [],
    page: { has_more: false, limit: 120 },
  }),
  listNetworkThreads: vi.fn().mockResolvedValue({
    threads: [],
    page: { has_more: false, limit: 50, total: 0 },
  }),
  listNetworkChannels: vi.fn().mockResolvedValue({ channels: [], recents: [] }),
}));

vi.mock("../../adapters/network-api", async () => {
  const actual = await vi.importActual("../../adapters/network-api");

  return {
    ...actual,
    listNetworkThreadMessages: mocks.listNetworkThreadMessages,
    listNetworkDirectRoomMessages: mocks.listNetworkDirectRoomMessages,
    listNetworkThreads: mocks.listNetworkThreads,
    listNetworkChannels: mocks.listNetworkChannels,
  };
});

import { NetworkApiError } from "../../adapters/network-api";
import {
  networkDirectDetailOptions,
  networkDirectMessagesOptions,
  networkThreadDetailOptions,
  networkThreadMessagesOptions,
  networkThreadsOptions,
  networkChannelsOptions,
} from "../query-options";
import { networkKeys } from "../query-keys";
import { flattenNetworkMessages } from "../infinite-data";
import { rollNetworkMessageTail } from "../network-message-tail";

function makeQueryContext<TQueryKey extends readonly unknown[], TPageParam = never>(
  queryKey: TQueryKey,
  pageParam?: TPageParam
) {
  return {
    client: new QueryClient(),
    direction: "forward" as const,
    meta: undefined,
    pageParam,
    queryKey,
    signal: new AbortController().signal,
  } as QueryFunctionContext<TQueryKey, TPageParam>;
}

function requireQueryFn<TQueryKey extends readonly unknown[], TPageParam = never>(
  queryFn: ((context: QueryFunctionContext<TQueryKey, TPageParam>) => unknown) | undefined
) {
  if (!queryFn) {
    throw new Error("Expected queryFn to be defined");
  }

  return queryFn;
}

function requireRetry(
  retry: boolean | number | ((failureCount: number, error: Error) => boolean) | undefined
) {
  if (typeof retry !== "function") {
    throw new Error("Expected retry to be a function");
  }

  return retry;
}

function networkMessage(messageId: string, text = messageId) {
  return {
    body: {},
    channel: "builders",
    direction: "received",
    kind: "say",
    message_id: messageId,
    peer_from: "peer",
    text,
    timestamp: "2026-07-11T00:01:00Z",
  };
}

describe("network query options , surface isolation", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("threads tab queries keep stable filters in the base key and message cursors in pageParam", async () => {
    const options = networkThreadMessagesOptions("ws_alpha", "builders", "thread_one");
    expect(options.queryKey).toEqual([
      "network",
      "workspace",
      "ws_alpha",
      "channel",
      "builders",
      "thread",
      "messages",
      "thread_one",
      "",
      "",
      120,
    ]);
    expect(options.initialPageParam).toBeNull();
    expect(options.refetchInterval).toBeUndefined();
    expect(options.refetchOnWindowFocus).toBe(false);
    await requireQueryFn(options.queryFn)(
      makeQueryContext(options.queryKey, { before: "msg_older" })
    );
    expect(mocks.listNetworkThreadMessages).toHaveBeenCalledWith(
      "ws_alpha",
      "builders",
      "thread_one",
      { before: "msg_older", limit: 120 },
      expect.any(AbortSignal)
    );
  });

  it("loads the bounded embedded recents projection with the channel catalog", async () => {
    const options = networkChannelsOptions("ws_alpha");
    expect(options.queryKey).toEqual(["network", "workspace", "ws_alpha", "channels", "list", 5]);
    if (typeof options.queryFn !== "function") throw new Error("Expected channel query function");
    await options.queryFn(makeQueryContext(options.queryKey));
    expect(mocks.listNetworkChannels).toHaveBeenCalledWith(
      "ws_alpha",
      { recent_limit: 5 },
      expect.any(AbortSignal)
    );
  });

  it("preserves server order while replacing a live-tail duplicate in place", () => {
    const data = rollNetworkMessageTail(
      {
        pages: [
          {
            messages: [networkMessage("msg-z", "first"), networkMessage("msg-y", "stale")],
            page: { has_more: false, limit: 4 },
          },
        ],
        pageParams: [null],
      },
      [networkMessage("msg-y", "updated"), networkMessage("msg-a", "last")]
    );

    expect(data.pages[0]?.messages.map(item => item.message_id)).toEqual([
      "msg-z",
      "msg-y",
      "msg-a",
    ]);
    expect(data.pages[0]?.messages[1]?.text).toBe("updated");
  });

  it("keeps the live tail bounded while rolling its oldest message into loaded history", () => {
    const data = rollNetworkMessageTail(
      {
        pages: [
          {
            messages: [networkMessage("msg-z"), networkMessage("msg-y"), networkMessage("msg-x")],
            page: { has_more: false, limit: 3 },
          },
        ],
        pageParams: [null],
      },
      [networkMessage("msg-a")]
    );

    expect(data.pages[0]?.messages.map(item => item.message_id)).toEqual([
      "msg-y",
      "msg-x",
      "msg-a",
    ]);
    expect(data.pages[1]?.messages.map(item => item.message_id)).toEqual(["msg-z"]);
  });

  it("flattens older pages before the live tail without inferring order from timestamp or id", () => {
    const flattened = flattenNetworkMessages({
      pages: [
        {
          messages: [networkMessage("msg-b"), networkMessage("msg-a")],
          page: { has_more: true, limit: 2, next_cursor: "msg-b" },
        },
        {
          messages: [networkMessage("msg-z"), networkMessage("msg-y")],
          page: { has_more: false, limit: 2 },
        },
      ],
      pageParams: [null, { before: "msg-b" }],
    });

    expect(flattened.map(item => item.message_id)).toEqual(["msg-z", "msg-y", "msg-b", "msg-a"]);
  });

  it("deduplicates overlapping message pages without moving the canonical position", () => {
    const flattened = flattenNetworkMessages({
      pages: [
        {
          messages: [networkMessage("msg-z", "updated"), networkMessage("msg-a", "last")],
          page: { has_more: true, limit: 2, next_cursor: "msg-z" },
        },
        {
          messages: [networkMessage("msg-z", "stale"), networkMessage("msg-y", "second")],
          page: { has_more: false, limit: 2 },
        },
      ],
      pageParams: [null, { before: "msg-z" }],
    });

    expect(flattened.map(item => item.message_id)).toEqual(["msg-z", "msg-y", "msg-a"]);
    expect(flattened[0]?.text).toBe("updated");
  });

  it("directs tab queries are namespaced with channel + surface + direct id", async () => {
    const options = networkDirectMessagesOptions("ws_alpha", "builders", "direct_one");
    expect(options.queryKey).toEqual([
      "network",
      "workspace",
      "ws_alpha",
      "channel",
      "builders",
      "direct",
      "messages",
      "direct_one",
      "",
      "",
      120,
    ]);
    expect(options.initialPageParam).toBeNull();
    await requireQueryFn(options.queryFn)(makeQueryContext(options.queryKey, null));
    expect(mocks.listNetworkDirectRoomMessages).toHaveBeenCalledWith(
      "ws_alpha",
      "builders",
      "direct_one",
      { limit: 120 },
      expect.any(AbortSignal)
    );
  });

  it("catalog queries preserve counted envelopes and bind only stable server filters to the key", async () => {
    const options = networkThreadsOptions("ws_alpha", "builders", {
      has_work: true,
      session_id: "session.alpha",
      query: "release",
      sort: "alphabetical",
    });

    expect(options.queryKey).toEqual([
      "network",
      "workspace",
      "ws_alpha",
      "channel",
      "builders",
      "thread",
      "list",
      "release",
      "session.alpha",
      true,
      "alphabetical",
      50,
    ]);
    expect(options.initialPageParam).toBeNull();

    const page = await requireQueryFn(options.queryFn)(
      makeQueryContext(options.queryKey, "thread_cursor")
    );
    expect(page).toEqual({ threads: [], page: { has_more: false, limit: 50, total: 0 } });
    expect(mocks.listNetworkThreads).toHaveBeenCalledWith(
      "ws_alpha",
      "builders",
      {
        after: "thread_cursor",
        has_work: true,
        limit: 50,
        session_id: "session.alpha",
        query: "release",
        sort: "alphabetical",
      },
      expect.any(AbortSignal)
    );
  });

  it("a directs query never shares the same key as a threads query in the same channel", () => {
    const threadsOpts = networkThreadMessagesOptions("ws_alpha", "builders", "shared_id");
    const directsOpts = networkDirectMessagesOptions("ws_alpha", "builders", "shared_id");
    expect(threadsOpts.queryKey).not.toEqual(directsOpts.queryKey);

    // Hierarchical key check using factories.
    expect(networkKeys.threadsList("ws_alpha", "builders").slice(0, 7)).toEqual([
      "network",
      "workspace",
      "ws_alpha",
      "channel",
      "builders",
      "thread",
      "list",
    ]);
    expect(networkKeys.directsList("ws_alpha", "builders").slice(0, 7)).toEqual([
      "network",
      "workspace",
      "ws_alpha",
      "channel",
      "builders",
      "direct",
      "list",
    ]);
  });

  it("a directs query never matches threads query under TanStack predicate", () => {
    const client = new QueryClient();
    const threadOpts = networkThreadMessagesOptions("ws_alpha", "builders", "container_x");
    const directOpts = networkDirectMessagesOptions("ws_alpha", "builders", "container_x");
    client.setQueryData(threadOpts.queryKey, {
      pages: [
        {
          messages: [
            {
              message_id: "thread-msg",
              body: {},
              channel: "builders",
              direction: "sent",
              kind: "say",
              peer_from: "p",
              timestamp: "",
            },
          ],
          page: { has_more: false, limit: 120 },
        },
      ],
      pageParams: [null],
    });
    client.setQueryData(directOpts.queryKey, {
      pages: [
        {
          messages: [
            {
              message_id: "direct-msg",
              body: {},
              channel: "builders",
              direction: "sent",
              kind: "say",
              peer_from: "p",
              timestamp: "",
            },
          ],
          page: { has_more: false, limit: 120 },
        },
      ],
      pageParams: [null],
    });

    const threadCacheValue = client.getQueryData(threadOpts.queryKey);
    const directCacheValue = client.getQueryData(directOpts.queryKey);
    expect(threadCacheValue?.pages[0]?.messages[0]?.message_id).toBe("thread-msg");
    expect(directCacheValue?.pages[0]?.messages[0]?.message_id).toBe("direct-msg");

    const threadEntry = client.getQueryCache().findAll({
      queryKey: networkKeys.threadsList("ws_alpha", "builders").slice(0, 6),
      exact: false,
    });
    const directEntry = client.getQueryCache().findAll({
      queryKey: networkKeys.directsList("ws_alpha", "builders").slice(0, 6),
      exact: false,
    });

    expect(threadEntry.flatMap(query => query.queryKey)).toContain("thread");
    expect(threadEntry.flatMap(query => query.queryKey)).not.toContain("direct");
    expect(directEntry.flatMap(query => query.queryKey)).toContain("direct");
    expect(directEntry.flatMap(query => query.queryKey)).not.toContain("thread");
  });

  it("normalizes message limit defaults inside both surfaces", async () => {
    const noLimit = networkThreadMessagesOptions("ws_alpha", "builders", "thread_one");
    const explicit = networkThreadMessagesOptions("ws_alpha", "builders", "thread_one", {
      limit: 120,
    });
    expect(noLimit.queryKey).toEqual(explicit.queryKey);
    await requireQueryFn(noLimit.queryFn)(makeQueryContext(noLimit.queryKey));
    expect(mocks.listNetworkThreadMessages).toHaveBeenLastCalledWith(
      "ws_alpha",
      "builders",
      "thread_one",
      { limit: 120 },
      expect.any(AbortSignal)
    );
  });

  it("does not retry 4xx conversation detail failures", () => {
    const threadRetry = requireRetry(
      networkThreadDetailOptions("ws_alpha", "builders", "missing").retry
    );
    const directRetry = requireRetry(
      networkDirectDetailOptions("ws_alpha", "builders", "missing").retry
    );

    expect(threadRetry(0, new NetworkApiError("Thread not found", 404))).toBe(false);
    expect(directRetry(0, new NetworkApiError("Invalid direct id", 400))).toBe(false);
  });

  it("retries transient conversation detail failures within the detail retry budget", () => {
    const retry = requireRetry(
      networkThreadDetailOptions("ws_alpha", "builders", "thread_one").retry
    );

    expect(retry(0, new Error("temporary network failure"))).toBe(true);
    expect(retry(1, new Error("temporary network failure"))).toBe(true);
    expect(retry(2, new Error("temporary network failure"))).toBe(false);
  });
});
