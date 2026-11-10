// Suite: docs search palette queries
// Invariant: Search asks the canonical `/api/search/` route, a failed answer is reported as a
// failure (never as "no page matches") and is not cached, and retrying asks again.
// Boundary IN: useDocsSearch's requests and returned state.
// Boundary OUT: The search index and the palette's markup.
import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { useDocsSearch } from "../hooks/use-docs-search";

const requests: string[] = [];

function answerWith(...responses: Array<() => Response>) {
  const queue = [...responses];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: URL) => {
      requests.push(url.pathname);
      const next = queue.shift();
      if (!next) throw new Error("unexpected search request");
      return next();
    })
  );
}

afterEach(() => {
  requests.length = 0;
  vi.unstubAllGlobals();
});

describe("useDocsSearch", () => {
  it("reports a failed answer, keeps it out of the cache and asks again on retry", async () => {
    answerWith(
      () => new Response("unavailable", { status: 503 }),
      () => Response.json([{ id: "a", url: "/docs/sessions", type: "page", content: "Sessions" }])
    );
    const { result } = renderHook(() => useDocsSearch("lifecycle", null));
    await waitFor(() => expect(result.current.failed).toBe(true));
    expect(result.current.hits).toEqual([]);
    expect(requests).toEqual(["/api/search/"]);

    act(() => result.current.retry());
    expect(result.current.loading).toBe(true);
    await waitFor(() => expect(result.current.hits.map(hit => hit.title)).toEqual(["Sessions"]));
    expect(result.current.failed).toBe(false);
    expect(requests).toEqual(["/api/search/", "/api/search/"]);
  });

  it("serves a query it already answered without asking again", async () => {
    answerWith(() =>
      Response.json([{ id: "b", url: "/docs/loops", type: "page", content: "Loops" }])
    );
    const first = renderHook(() => useDocsSearch("loops cache", null));
    await waitFor(() => expect(first.result.current.hits).toHaveLength(1));
    first.unmount();
    const again = renderHook(() => useDocsSearch("loops cache", null));
    expect(again.result.current).toMatchObject({ loading: false, failed: false });
    expect(again.result.current.hits.map(hit => hit.title)).toEqual(["Loops"]);
    expect(requests).toEqual(["/api/search/"]);
  });
});
