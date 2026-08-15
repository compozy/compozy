import { renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("@tanstack/react-query", async importOriginal => {
  const actual = await importOriginal<typeof import("@tanstack/react-query")>();
  return { ...actual, useQuery: vi.fn() };
});

import { useQuery } from "@tanstack/react-query";

import { useLoopNodeExists } from "../use-loop-node-exists";

describe("useLoopNodeExists", () => {
  it("Should read errors as absent even when TanStack retains stale data", () => {
    vi.mocked(useQuery).mockReturnValue({
      data: { items: [{ node_id: "stale" }] },
      isError: true,
    } as never);
    const { result } = renderHook(() => useLoopNodeExists("ws_alpha", "waiting"));
    expect(result.current).toBe(false);
  });

  it("Should return true when a successful probe has items", () => {
    vi.mocked(useQuery).mockReturnValue({
      data: { items: [{ node_id: "wait_1" }] },
      isError: false,
    } as never);
    const { result } = renderHook(() => useLoopNodeExists("ws_alpha", "waiting"));
    expect(result.current).toBe(true);
  });
});
