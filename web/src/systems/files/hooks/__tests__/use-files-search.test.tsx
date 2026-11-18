import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { FilesRootRef } from "../../adapters/files-api";
import { useFilesFind } from "../use-files-find";
import { FILES_SEARCH_DEBOUNCE_MS, useFilesSearch } from "../use-files-search";
import { DEFAULT_FILES_SEARCH_OPTIONS } from "../use-files-search-prefs";

// Suite: explorer search hooks (file-explorer US-013, US-014; UT-091, UT-092).
// Invariant: only the newest Go to file / Search query renders, short queries never reach the
// daemon, typed queries wait 300 ms idle, Enter runs at once, and a replaced request is aborted.
// Owning layer: the files search hooks over the files adapter. Boundary: `fetch` (stubbed).
const ROOT: FilesRootRef = { kind: "session", workspaceId: "ws_1", sessionId: "sess-1" };
const ROOT_PAYLOAD = { kind: "workspace", workspace_id: "ws_1", name: "compozy" };

function wrapper() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );
}

interface Captured {
  url: URL;
  signal: AbortSignal | null;
  resolve: (body: unknown) => void;
}

function stubDeferredFetch() {
  const calls: Captured[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn((input: RequestInfo | URL) => {
      const request = input instanceof Request ? input : new Request(String(input));
      return new Promise<Response>((resolve, reject) => {
        request.signal.addEventListener("abort", () =>
          reject(new DOMException("aborted", "AbortError"))
        );
        calls.push({
          url: new URL(request.url),
          signal: request.signal,
          resolve: body => resolve(Response.json(body)),
        });
      });
    })
  );
  return calls;
}

function findBody(q: string, path: string) {
  return {
    root: ROOT_PAYLOAD,
    query: q,
    results: [{ path, kind: "file", score: 10, match_ranges: [[0, 1]] }],
    indexed_files: 3,
    index_truncated: false,
  };
}

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe("useFilesFind", () => {
  // UT-091
  it("Should render only the newest query when answers arrive in reverse order", async () => {
    const calls = stubDeferredFetch();
    const { result, rerender } = renderHook(({ input }) => useFilesFind(ROOT, input), {
      wrapper: wrapper(),
      initialProps: { input: "ses" },
    });
    await waitFor(() => expect(calls).toHaveLength(1));
    rerender({ input: "sesr" });
    await waitFor(() => expect(calls).toHaveLength(2));
    const [ses, sesr] = calls;
    expect(ses?.url.searchParams.get("q")).toBe("ses");
    expect(sesr?.url.searchParams.get("q")).toBe("sesr");

    sesr?.resolve(findBody("sesr", "web/src/session-review-panel.tsx"));
    ses?.resolve(findBody("ses", "web/src/session.ts"));

    await waitFor(() => expect(result.current.state.kind).toBe("results"));
    // Let the late `ses` answer settle; it must never replace `sesr`.
    await new Promise(resolve => setTimeout(resolve, 20));
    const state = result.current.state;
    expect(state.kind === "results" && state.response.query).toBe("sesr");
    expect(state.kind === "results" && state.response.results[0]?.path).toBe(
      "web/src/session-review-panel.tsx"
    );
  });

  it("Should parse a line suffix and show recent files for an empty query", () => {
    stubDeferredFetch();
    const { result, rerender } = renderHook(({ input }) => useFilesFind(ROOT, input), {
      wrapper: wrapper(),
      initialProps: { input: "" },
    });
    expect(result.current.state.kind).toBe("recent");
    rerender({ input: "main.tsx:12" });
    expect(result.current.query).toEqual({ name: "main.tsx", line: 12 });
  });
});

describe("useFilesSearch", () => {
  // UT-092
  it("Should skip short queries, debounce typing, run Enter at once, and abort replaced requests", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: false });
    const calls = stubDeferredFetch();
    const { result } = renderHook(() => useFilesSearch(ROOT, DEFAULT_FILES_SEARCH_OPTIONS), {
      wrapper: wrapper(),
    });

    act(() => result.current.setDraft("u"));
    await act(() => vi.advanceTimersByTimeAsync(FILES_SEARCH_DEBOUNCE_MS * 2));
    expect(calls).toHaveLength(0);
    expect(result.current.state.kind).toBe("short");

    act(() => result.current.setDraft("useS"));
    await act(() => vi.advanceTimersByTimeAsync(FILES_SEARCH_DEBOUNCE_MS - 1));
    expect(calls).toHaveLength(0);
    await act(() => vi.advanceTimersByTimeAsync(1));
    expect(calls).toHaveLength(1);
    expect(calls[0]?.url.searchParams.get("q")).toBe("useS");

    act(() => result.current.setDraft("useSession"));
    act(() => result.current.submit());
    await act(() => vi.advanceTimersByTimeAsync(0));
    expect(calls).toHaveLength(2);
    expect(calls[1]?.url.searchParams.get("q")).toBe("useSession");
    expect(calls[0]?.signal?.aborted).toBe(true);
    expect(calls[1]?.signal?.aborted).toBe(false);

    // The debounce for the same query does not send it twice.
    await act(() => vi.advanceTimersByTimeAsync(FILES_SEARCH_DEBOUNCE_MS * 2));
    expect(calls).toHaveLength(2);
  });

  it("Should send toggles and repeated globs, and surface an invalid regex inline", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const request = input instanceof Request ? input : new Request(String(input));
        const url = new URL(request.url);
        expect(url.searchParams.get("regex")).toBe("true");
        expect(url.searchParams.getAll("include")).toEqual(["web/**", "*.go"]);
        expect(url.searchParams.getAll("exclude")).toEqual(["**/*.test.tsx"]);
        return Response.json(
          { code: "invalid_query", error: 'invalid search: missing ) in "foo("' },
          { status: 400 }
        );
      })
    );
    const options = {
      ...DEFAULT_FILES_SEARCH_OPTIONS,
      regex: true,
      include: "web/**, *.go",
      exclude: "**/*.test.tsx",
    };
    const { result } = renderHook(() => useFilesSearch(ROOT, options, "foo("), {
      wrapper: wrapper(),
    });
    act(() => result.current.submit());
    await waitFor(() => expect(result.current.state.kind).toBe("invalid"));
    const state = result.current.state;
    expect(state.kind === "invalid" && state.message).toContain("missing )");
    expect(vi.mocked(fetch)).toHaveBeenCalledTimes(1);
  });
});
