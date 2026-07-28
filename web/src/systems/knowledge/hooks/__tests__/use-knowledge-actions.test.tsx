import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  useDeleteMemory,
  useEditMemory,
  useRevertMemoryDecision,
  useTriggerMemoryDream,
  useWriteMemory,
} from "@/systems/knowledge/hooks/use-knowledge-actions";
import {
  memoryDecisionRevertFixture,
  memoryDeleteFixture,
  memoryDreamTriggerFixture,
  memoryEditFixture,
  memoryHeadersFixture,
  memoryWriteFixture,
} from "@/systems/knowledge/mocks";
import { knowledgeKeys } from "@/systems/knowledge/lib/query-keys";

vi.mock("@/systems/knowledge/adapters/knowledge-api", () => ({
  listMemories: vi.fn(),
  listMemoryDecisions: vi.fn(),
  readMemory: vi.fn(),
  revertMemoryDecision: vi.fn(),
  searchMemory: vi.fn(),
  deleteMemory: vi.fn(),
  editMemory: vi.fn(),
  writeMemory: vi.fn(),
  triggerMemoryDream: vi.fn(),
}));

import {
  deleteMemory,
  editMemory,
  revertMemoryDecision,
  triggerMemoryDream,
  writeMemory,
} from "@/systems/knowledge/adapters/knowledge-api";

describe("useDeleteMemory", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("Should call deleteMemory with the selector and invalidate the knowledge cache", async () => {
    vi.mocked(deleteMemory).mockResolvedValue(memoryDeleteFixture);

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const listKey = knowledgeKeys.list({ scope: "global", limit: 1 });
    const cachedList = {
      pageParams: [undefined],
      pages: [
        {
          memories: memoryHeadersFixture.slice(0, 1),
          page: { has_more: false, limit: 1, total: 1 },
        },
      ],
    };
    queryClient.setQueryData(listKey, cachedList);
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const wrapper = ({ children }: { children: ReactNode }) =>
      createElement(QueryClientProvider, { client: queryClient }, children);

    const { result } = renderHook(() => useDeleteMemory(), { wrapper });

    act(() => {
      result.current.mutate({
        selector: { scope: "agent", agentName: "cto", agentTier: "workspace", workspaceId: "ws" },
        filename: "old.md",
      });
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(deleteMemory).toHaveBeenCalledWith(
      { scope: "agent", agentName: "cto", agentTier: "workspace", workspaceId: "ws" },
      "old.md"
    );
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["knowledge"] });
    expect(queryClient.getQueryData(listKey)).toEqual(cachedList);
  });

  it("Should surface daemon failures as the mutation error", async () => {
    const failure = new Error("daemon down");
    vi.mocked(deleteMemory).mockRejectedValue(failure);

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });

    const wrapper = ({ children }: { children: ReactNode }) =>
      createElement(QueryClientProvider, { client: queryClient }, children);

    const { result } = renderHook(() => useDeleteMemory(), { wrapper });

    act(() => {
      result.current.mutate({ selector: { scope: "global" }, filename: "missing.md" });
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toBe(failure);
  });
});

describe("useEditMemory", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("Should call editMemory and invalidate the knowledge cache on settle", async () => {
    vi.mocked(editMemory).mockResolvedValue(memoryEditFixture);

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const wrapper = ({ children }: { children: ReactNode }) =>
      createElement(QueryClientProvider, { client: queryClient }, children);

    const { result } = renderHook(() => useEditMemory(), { wrapper });

    act(() => {
      result.current.mutate({
        filename: "operator-style.md",
        body: { content: "next", scope: "global", type: "user", name: "Operator Style" },
      });
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(editMemory).toHaveBeenCalledWith("operator-style.md", {
      content: "next",
      scope: "global",
      type: "user",
      name: "Operator Style",
    });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["knowledge"] });
  });

  it("Should surface daemon failures as the mutation error", async () => {
    const failure = new Error("policy reject");
    vi.mocked(editMemory).mockRejectedValue(failure);

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });

    const wrapper = ({ children }: { children: ReactNode }) =>
      createElement(QueryClientProvider, { client: queryClient }, children);

    const { result } = renderHook(() => useEditMemory(), { wrapper });

    act(() => {
      result.current.mutate({
        filename: "operator-style.md",
        body: { content: "x" },
      });
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toBe(failure);
  });
});

describe("useWriteMemory", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("Should call writeMemory and invalidate the knowledge cache on settle", async () => {
    vi.mocked(writeMemory).mockResolvedValue(memoryWriteFixture);

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const wrapper = ({ children }: { children: ReactNode }) =>
      createElement(QueryClientProvider, { client: queryClient }, children);

    const { result } = renderHook(() => useWriteMemory(), { wrapper });

    act(() => {
      result.current.mutate({
        scope: "workspace",
        workspace_id: "ws_launch",
        type: "project",
        name: "Launch Memory",
        content: "memory body",
      });
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(writeMemory).toHaveBeenCalledWith({
      scope: "workspace",
      workspace_id: "ws_launch",
      type: "project",
      name: "Launch Memory",
      content: "memory body",
    });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["knowledge"] });
  });
});

describe("useRevertMemoryDecision", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("Should call revertMemoryDecision and invalidate cache on settle", async () => {
    vi.mocked(revertMemoryDecision).mockResolvedValue(memoryDecisionRevertFixture);

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const wrapper = ({ children }: { children: ReactNode }) =>
      createElement(QueryClientProvider, { client: queryClient }, children);

    const { result } = renderHook(() => useRevertMemoryDecision(), { wrapper });

    act(() => {
      result.current.mutate({
        decisionID: "dec_edit_fixture",
        body: { reason: "operator reverted from Knowledge" },
      });
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(revertMemoryDecision).toHaveBeenCalledWith("dec_edit_fixture", {
      reason: "operator reverted from Knowledge",
    });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["knowledge"] });
  });
});

describe("useTriggerMemoryDream", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("Should call triggerMemoryDream and invalidate cache on settle", async () => {
    vi.mocked(triggerMemoryDream).mockResolvedValue(memoryDreamTriggerFixture);

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const wrapper = ({ children }: { children: ReactNode }) =>
      createElement(QueryClientProvider, { client: queryClient }, children);

    const { result } = renderHook(() => useTriggerMemoryDream(), { wrapper });

    act(() => {
      result.current.mutate({ workspaceID: "ws_launch" });
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(triggerMemoryDream).toHaveBeenCalledWith("ws_launch");
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["knowledge"] });
  });
});
