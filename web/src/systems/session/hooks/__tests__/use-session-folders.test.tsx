// Suite: session folder layout writes
// Invariant: a manual reorder patches the served catalog page at once, sends the
// container's full order to the daemon, and restores the whole envelope when
// the daemon refuses; a delete clears membership optimistically.
// Owning layer: useSessionFolders (query cache + adapter); fetch is the I/O boundary.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const { toastError } = vi.hoisted(() => ({ toastError: vi.fn() }));
vi.mock("sonner", () => ({ toast: { error: toastError } }));

import { primarySessionFixture } from "../../testing";
import { sessionKeys } from "../../lib/query-keys";
import type { SessionCatalogPageResponse, SessionPayload } from "../../types";
import { useSessionFolders } from "../use-session-folders";

const folder = {
  id: "sfold-a",
  name: "Alpha",
  workspace_id: "ws",
  profile_id: "default",
  position: 0,
  session_count: 1,
  created_at: "2026-10-17T00:00:00Z",
  updated_at: "2026-10-17T00:00:00Z",
};
const session = (id: string, folderId?: string): SessionPayload =>
  ({
    ...primarySessionFixture,
    id,
    archived_at: null,
    ...(folderId ? { folder_id: folderId } : {}),
  }) as SessionPayload;
const pageKey = [...sessionKeys.workspaceLists("ws"), "page", { workspace_id: "ws" }];
const envelope = {
  pageParams: [{ previous: [] }],
  pages: [
    {
      sessions: [session("a"), session("b"), session("c", "sfold-a")],
      page: { has_more: false, limit: 100 },
    } as unknown as SessionCatalogPageResponse,
  ],
};

function json(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function setup(writeStatus: number) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  queryClient.setQueryData(pageKey, structuredClone(envelope));
  const writes: Request[] = [];
  vi.mocked(globalThis.fetch).mockImplementation(async input => {
    const request = input as Request;
    if (request.method === "GET") return json({ folders: [folder] });
    writes.push(request);
    return writeStatus === 200
      ? json({ folders: [folder] })
      : json({ error: "layout changed concurrently" }, writeStatus);
  });
  const { result } = renderHook(
    () => useSessionFolders({ workspaceId: "ws", profile: "default", enabled: true }),
    {
      wrapper: ({ children }: { children: ReactNode }) => (
        <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
      ),
    }
  );
  const order = () =>
    queryClient
      .getQueryData<typeof envelope>(pageKey)
      ?.pages[0]?.sessions.map(item => [item.id, item.folder_id ?? null]);
  return { queryClient, result, writes, order };
}

describe("useSessionFolders", () => {
  beforeEach(() => {
    vi.stubGlobal("fetch", vi.fn());
    toastError.mockClear();
  });
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("Should expose no layout until the folder list loads", async () => {
    const { result } = setup(200);
    expect(result.current).toBeUndefined();
    await waitFor(() => expect(result.current?.folders).toHaveLength(1));
  });

  it("Should reorder optimistically and restore the full envelope when the daemon refuses", async () => {
    const { result, writes, order } = setup(409);
    await waitFor(() => expect(result.current?.folders).toHaveLength(1));

    act(() => result.current?.reorderSessions(null, ["b", "a"]));
    await waitFor(() => expect(writes).toHaveLength(1));
    const request = writes[0]!;
    expect(new URL(request.url).pathname).toBe("/api/workspaces/ws/session-order/sessions");
    expect(request.method).toBe("PUT");
    expect(await request.clone().json()).toEqual({ session_ids: ["b", "a"] });

    await waitFor(() =>
      expect(toastError).toHaveBeenCalledWith("Couldn't reorder the sessions.", expect.anything())
    );
    expect(order()).toEqual([
      ["a", null],
      ["b", null],
      ["c", "sfold-a"],
    ]);
  });

  it("Should keep the optimistic order and folder once the daemon accepts", async () => {
    const { result, writes, order } = setup(200);
    await waitFor(() => expect(result.current?.folders).toHaveLength(1));

    act(() => result.current?.reorderSessions("sfold-a", ["a", "c"]));
    await waitFor(() =>
      expect(order()).toEqual([
        ["a", "sfold-a"],
        ["b", null],
        ["c", "sfold-a"],
      ])
    );
    await waitFor(() => expect(writes).toHaveLength(1));
    expect(await writes[0]!.clone().json()).toEqual({
      folder_id: "sfold-a",
      session_ids: ["a", "c"],
    });
    expect(toastError).not.toHaveBeenCalled();
  });

  it("Should send rapid layout writes one at a time in action order", async () => {
    const { result, writes } = setup(200);
    await waitFor(() => expect(result.current?.folders).toHaveLength(1));
    let releaseFirst: (response: Response) => void = () => undefined;
    const fetchMock = vi.mocked(globalThis.fetch);
    const settled = fetchMock.getMockImplementation()!;
    fetchMock.mockImplementation(async input => {
      const request = input as Request;
      if (request.method !== "GET" && writes.length === 0) {
        writes.push(request);
        return new Promise<Response>(resolve => {
          releaseFirst = resolve;
        });
      }
      return settled(input);
    });

    act(() => {
      result.current?.reorderFolders(["sfold-a"]);
      result.current?.reorderSessions(null, ["b", "a"]);
    });
    await waitFor(() => expect(writes).toHaveLength(1));
    await new Promise(resolve => setTimeout(resolve, 50));
    expect(writes).toHaveLength(1);

    releaseFirst(json({ folders: [folder] }));
    await waitFor(() => expect(writes).toHaveLength(2));
    expect(new URL(writes[0]!.url).pathname).toBe("/api/workspaces/ws/session-order/folders");
    expect(new URL(writes[1]!.url).pathname).toBe("/api/workspaces/ws/session-order/sessions");
  });

  it("Should clear membership at once when a folder is deleted", async () => {
    const { result, writes, order } = setup(200);
    await waitFor(() => expect(result.current?.folders).toHaveLength(1));

    let pending: Promise<void> | undefined;
    act(() => {
      pending = result.current?.deleteFolder("sfold-a");
    });
    await waitFor(() =>
      expect(order()).toEqual([
        ["a", null],
        ["b", null],
        ["c", null],
      ])
    );
    await act(async () => pending);
    expect(writes[0]!.method).toBe("DELETE");
    expect(new URL(writes[0]!.url).pathname).toBe("/api/workspaces/ws/session-folders/sfold-a");
  });
});
