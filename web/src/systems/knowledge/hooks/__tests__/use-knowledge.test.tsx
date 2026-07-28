import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  useMemories,
  useMemory,
  useMemoryDecisions,
  useMemorySearch,
} from "@/systems/knowledge/hooks/use-knowledge";

vi.mock("@/systems/knowledge/adapters/knowledge-api", () => ({
  listMemories: vi.fn(),
  listMemoryDecisions: vi.fn(),
  readMemory: vi.fn(),
  searchMemory: vi.fn(),
  deleteMemory: vi.fn(),
  editMemory: vi.fn(),
  writeMemory: vi.fn(),
  triggerMemoryDream: vi.fn(),
}));

import {
  listMemories,
  listMemoryDecisions,
  readMemory,
  searchMemory,
} from "@/systems/knowledge/adapters/knowledge-api";
import type { MemoryHeader } from "@/systems/knowledge/types";

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

const validHeader: MemoryHeader = {
  filename: "user_role.md",
  mod_time: "2026-04-01T12:00:00Z",
  name: "User Role",
  scope: "global",
  type: "user",
  recall_count: 0,
  injection: true,
  system_managed: false,
};

describe("useMemories", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("Should pass selector and abort signal to the list adapter", async () => {
    vi.mocked(listMemories).mockResolvedValue({
      memories: [validHeader],
      page: { has_more: false, limit: 50, total: 7 },
    });

    const { result } = renderHook(() => useMemories({ scope: "global", workspaceId: "ws" }), {
      wrapper: createWrapper(),
    });

    expect(result.current.isLoading).toBe(true);

    await waitFor(() => {
      expect(result.current.data).toHaveLength(1);
    });

    expect(result.current.total).toBe(7);

    expect(listMemories).toHaveBeenCalledWith(
      { scope: "global", workspaceId: "ws" },
      expect.any(AbortSignal)
    );
  });

  it("Should append catalog pages without reordering backend results", async () => {
    const firstHeader = {
      ...validHeader,
      agent_name: "agent:workspace",
      workspace_id: "ws",
    };
    const secondHeader = {
      ...validHeader,
      agent_name: "workspace",
      workspace_id: "ws:agent",
      name: "A name that must remain second",
    };
    vi.mocked(listMemories)
      .mockResolvedValueOnce({
        memories: [firstHeader],
        page: { has_more: true, limit: 1, next_cursor: "next", total: 2 },
      })
      .mockResolvedValueOnce({
        memories: [secondHeader],
        page: { has_more: false, limit: 1, total: 2 },
      });

    const { result } = renderHook(() => useMemories({ scope: "global", limit: 1 }), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.data).toHaveLength(1));
    await result.current.fetchNextPage();
    await waitFor(() => {
      expect(result.current.data?.map(memory => memory.filename)).toEqual([
        "user_role.md",
        "user_role.md",
      ]);
      expect(result.current.data?.map(memory => memory.workspace_id)).toEqual(["ws", "ws:agent"]);
    });
    expect(result.current.total).toBe(2);
    expect(listMemories).toHaveBeenNthCalledWith(
      2,
      { scope: "global", cursor: "next", limit: 1 },
      expect.any(AbortSignal)
    );
  });

  it("Should not fetch when no selector is provided", () => {
    renderHook(() => useMemories(undefined), { wrapper: createWrapper() });
    expect(listMemories).not.toHaveBeenCalled();
  });
});

describe("useMemory", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("Should load a memory's summary plus content", async () => {
    vi.mocked(readMemory).mockResolvedValue({ ...validHeader, content: "# Memory content" });

    const { result } = renderHook(() => useMemory({ scope: "global" }, "test.md"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.data?.content).toBe("# Memory content");
    });

    expect(readMemory).toHaveBeenCalledWith(
      { scope: "global" },
      "test.md",
      expect.any(AbortSignal)
    );
  });

  it("Should not fetch when selector is omitted", () => {
    renderHook(() => useMemory(undefined, "test.md"), { wrapper: createWrapper() });
    expect(readMemory).not.toHaveBeenCalled();
  });

  it("Should not fetch when filename is empty", () => {
    renderHook(() => useMemory({ scope: "global" }, ""), { wrapper: createWrapper() });
    expect(readMemory).not.toHaveBeenCalled();
  });
});

describe("useMemorySearch", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("Should not call the search adapter when query text is empty", () => {
    renderHook(() => useMemorySearch({ scope: "global" }, "   "), {
      wrapper: createWrapper(),
    });
    expect(searchMemory).not.toHaveBeenCalled();
  });

  it("Should call the search adapter with the trimmed query and selector", async () => {
    vi.mocked(searchMemory).mockResolvedValue({
      results: [{ memory: validHeader, score: 0.5 }],
      recall: { blocks: [], header: { content_hash: "h", text: "" } },
    });

    const { result } = renderHook(
      () => useMemorySearch({ scope: "global", workspaceId: "ws" }, "  rollout  ", { topK: 4 }),
      {
        wrapper: createWrapper(),
      }
    );

    await waitFor(() => {
      expect(result.current.data?.results).toHaveLength(1);
    });

    expect(searchMemory).toHaveBeenCalledWith(
      expect.objectContaining({
        query_text: "rollout",
        scope: "global",
        workspace_id: "ws",
        top_k: 4,
      }),
      expect.any(AbortSignal)
    );
  });
});

describe("useMemoryDecisions", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("Should not call decisions adapter when params are missing", () => {
    renderHook(() => useMemoryDecisions(undefined), { wrapper: createWrapper() });
    expect(listMemoryDecisions).not.toHaveBeenCalled();
  });

  it("Should call decisions adapter with selector and filter params", async () => {
    vi.mocked(listMemoryDecisions).mockResolvedValue({ decisions: [] });

    const { result } = renderHook(
      () =>
        useMemoryDecisions({
          scope: "global",
          limit: 5,
        }),
      { wrapper: createWrapper() }
    );

    await waitFor(() => {
      expect(result.current.data?.decisions).toEqual([]);
    });

    expect(listMemoryDecisions).toHaveBeenCalledWith(
      { scope: "global", limit: 5 },
      expect.any(AbortSignal)
    );
  });
});
