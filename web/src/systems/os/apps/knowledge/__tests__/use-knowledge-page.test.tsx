import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { MemoryHeader, MemoryDecision, MemorySearchResponse } from "@/systems/knowledge/types";

const useMemoriesMock = vi.fn();
const useMemoryMock = vi.fn();
const useMemorySearchMock = vi.fn();
const useMemoryDecisionsMock = vi.fn();
const deleteMutateAsync = vi.fn();
const deleteReset = vi.fn();
const editMutateAsync = vi.fn();
const editReset = vi.fn();
const writeMutateAsync = vi.fn();
const writeReset = vi.fn();
const revertMutateAsync = vi.fn();
const revertReset = vi.fn();
const fetchNextPage = vi.fn();
const refetchMemories = vi.fn();
let mockMemoriesFetchNextPageError = false;
let mockMemoriesRefetchError = false;
let mockDeletePending = false;
let mockDeleteError: Error | null = null;
let mockEditPending = false;
let mockEditError: Error | null = null;
let mockWritePending = false;
let mockWriteError: Error | null = null;
let mockRevertPending = false;
let mockRevertError: Error | null = null;

vi.mock("@/systems/workspace", () => ({
  useActiveWorkspace: () => ({
    activeWorkspace: {
      id: "ws_signalforge",
      root_dir: "/workspaces/signalforge",
      add_dirs: [],
      name: "signalforge",
      created_at: "2026-04-25T12:00:00Z",
      updated_at: "2026-04-25T12:00:00Z",
    },
    activeWorkspaceId: "ws_signalforge",
  }),
}));

vi.mock("@/systems/knowledge", async () => {
  const actual = await vi.importActual("@/systems/knowledge");
  return {
    ...actual,
    useMemories: (...args: unknown[]) => useMemoriesMock(...args),
    useMemory: (...args: unknown[]) => useMemoryMock(...args),
    useMemorySearch: (...args: unknown[]) => useMemorySearchMock(...args),
    useMemoryDecisions: (...args: unknown[]) => useMemoryDecisionsMock(...args),
    useDeleteMemory: () => ({
      error: mockDeleteError,
      isPending: mockDeletePending,
      mutateAsync: deleteMutateAsync,
      reset: deleteReset,
    }),
    useEditMemory: () => ({
      error: mockEditError,
      isPending: mockEditPending,
      mutateAsync: editMutateAsync,
      reset: editReset,
    }),
    useWriteMemory: () => ({
      error: mockWriteError,
      isPending: mockWritePending,
      mutateAsync: writeMutateAsync,
      reset: writeReset,
    }),
    useRevertMemoryDecision: () => ({
      error: mockRevertError,
      isPending: mockRevertPending,
      mutateAsync: revertMutateAsync,
      reset: revertReset,
    }),
  };
});

import { useKnowledgePage } from "@/systems/os/apps/knowledge/use-knowledge-page";

const GLOBAL_MEMORY: MemoryHeader = {
  filename: "operator-playbook-0425.md",
  mod_time: "2026-04-25T21:00:00Z",
  name: "Operator Playbook 0425",
  description: "Reusable operator checklist",
  type: "reference",
  scope: "global",
  recall_count: 0,
  injection: true,
  system_managed: false,
};

const WORKSPACE_MEMORY: MemoryHeader = {
  filename: "launch-brief-0425.md",
  mod_time: "2026-04-25T21:01:00Z",
  name: "Launch Brief 0425",
  description: "Workspace launch brief",
  type: "project",
  scope: "workspace",
  workspace_id: "ws_signalforge",
  recall_count: 0,
  injection: true,
  system_managed: false,
};

const AGENT_MEMORY: MemoryHeader = {
  filename: "cto-tone.md",
  mod_time: "2026-04-25T21:02:00Z",
  name: "CTO Tone",
  description: "Direct, calm tone for CTO summaries",
  type: "user",
  scope: "agent",
  agent_name: "cto",
  agent_tier: "workspace",
  workspace_id: "ws_signalforge",
  recall_count: 4,
  injection: true,
  system_managed: false,
};

const SEARCH_RESPONSE: MemorySearchResponse = {
  results: [
    {
      memory: WORKSPACE_MEMORY,
      score: 0.92,
      snippet: "launch brief snippet",
      why_recalled: ["fts5:exact"],
    },
  ],
  recall: { blocks: [], header: { content_hash: "h", text: "" } },
};

describe("useKnowledgePage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockDeletePending = false;
    mockDeleteError = null;
    mockEditPending = false;
    mockEditError = null;
    mockWritePending = false;
    mockWriteError = null;
    mockRevertPending = false;
    mockRevertError = null;
    fetchNextPage.mockResolvedValue(undefined);
    refetchMemories.mockResolvedValue(undefined);
    mockMemoriesFetchNextPageError = false;
    mockMemoriesRefetchError = false;
    useMemoriesMock.mockImplementation(selector => {
      if (!selector) {
        return catalogResult([], 0, false);
      }
      if (selector.scope === "global") {
        return catalogResult([GLOBAL_MEMORY], 17, true);
      }
      if (selector.scope === "workspace") {
        return catalogResult([WORKSPACE_MEMORY], 1, false);
      }
      if (selector.scope === "agent") {
        return catalogResult([AGENT_MEMORY], 1, false);
      }
      return catalogResult([], 0, false);
    });
    useMemoryMock.mockReturnValue({
      data: { ...GLOBAL_MEMORY, content: "# Memory content" },
      isLoading: false,
      error: null,
    });
    useMemorySearchMock.mockReturnValue({ data: undefined, isLoading: false, error: null });
    useMemoryDecisionsMock.mockReturnValue({
      data: { decisions: [] },
      isLoading: false,
      error: null,
    });
    deleteReset.mockImplementation(() => {
      mockDeleteError = null;
    });
    editReset.mockImplementation(() => {
      mockEditError = null;
    });
    writeReset.mockImplementation(() => {
      mockWriteError = null;
    });
    revertReset.mockImplementation(() => {
      mockRevertError = null;
    });
    deleteMutateAsync.mockResolvedValue(undefined);
    editMutateAsync.mockResolvedValue(undefined);
    writeMutateAsync.mockResolvedValue({
      applied: true,
      decision: {
        id: "dec_write",
        candidate_hash: "h",
        op: "add",
        scope: "workspace",
        source: "rule",
        confidence: 1,
        decided_at: "2026-04-25T21:05:00Z",
        target_filename: "launch-memory.md",
        frontmatter: {
          filename: "launch-memory.md",
          mod_time: "2026-04-25T21:05:00Z",
          name: "Launch Memory",
          type: "project",
        },
      },
    });
    revertMutateAsync.mockResolvedValue(undefined);
  });

  it("Should default to the global scope and load global memories on mount", async () => {
    const { result } = renderHook(() => useKnowledgePage());

    await waitFor(() => {
      expect(result.current.activeScope).toBe("global");
      expect(result.current.memories).toHaveLength(1);
    });

    expect(useMemoriesMock).toHaveBeenLastCalledWith(
      { scope: "global", includeSystem: false, limit: 50, sort: "recent" },
      expect.objectContaining({ enabled: true })
    );
    expect(result.current.memoryCount).toBe(17);
    expect(result.current.hasMoreMemories).toBe(true);

    act(() => {
      result.current.loadMoreMemories();
    });
    expect(fetchNextPage).toHaveBeenCalledTimes(1);
  });

  it("Should switch to workspace scope and pass the active workspace id to the list query", async () => {
    const { result } = renderHook(() => useKnowledgePage());

    act(() => {
      result.current.setActiveScope("workspace");
    });

    await waitFor(() => {
      expect(result.current.activeScope).toBe("workspace");
    });

    expect(useMemoriesMock).toHaveBeenLastCalledWith(
      {
        scope: "workspace",
        workspaceId: "ws_signalforge",
        includeSystem: false,
        limit: 50,
        sort: "recent",
      },
      expect.objectContaining({ enabled: true })
    );
  });

  it("Should retry a failed continuation with the same next-page operation", async () => {
    mockMemoriesFetchNextPageError = true;
    const { result } = renderHook(() => useKnowledgePage());

    act(() => {
      result.current.retryMemories();
    });

    expect(fetchNextPage).toHaveBeenCalledTimes(1);
    expect(refetchMemories).not.toHaveBeenCalled();
  });

  it("Should retry a failed refetch without advancing the page cursor", async () => {
    mockMemoriesRefetchError = true;
    const { result } = renderHook(() => useKnowledgePage());

    act(() => {
      result.current.retryMemories();
    });

    expect(refetchMemories).toHaveBeenCalledTimes(1);
    expect(fetchNextPage).not.toHaveBeenCalled();
  });

  it("Should switch to agent scope and require an agent name before issuing the list query", async () => {
    const { result } = renderHook(() => useKnowledgePage());

    act(() => {
      result.current.setActiveScope("agent");
    });

    await waitFor(() => {
      expect(result.current.guardMessage).toMatch(/agent name/i);
    });

    act(() => {
      result.current.setAgentName("cto");
    });

    await waitFor(() => {
      expect(result.current.guardMessage).toBeNull();
    });

    expect(useMemoriesMock).toHaveBeenLastCalledWith(
      {
        scope: "agent",
        agentName: "cto",
        agentTier: "workspace",
        workspaceId: "ws_signalforge",
        includeSystem: false,
        limit: 50,
        sort: "recent",
      },
      expect.objectContaining({ enabled: true })
    );
  });

  it("Should switch to server-backed search when a query is entered", async () => {
    useMemorySearchMock.mockReturnValue({
      data: SEARCH_RESPONSE,
      isLoading: false,
      error: null,
    });

    const { result } = renderHook(() => useKnowledgePage());

    act(() => {
      result.current.setActiveScope("workspace");
    });
    act(() => {
      result.current.setSearchQuery("launch");
    });

    await waitFor(() => {
      expect(result.current.searchActive).toBe(true);
    });

    expect(useMemorySearchMock).toHaveBeenLastCalledWith(
      { scope: "workspace", workspaceId: "ws_signalforge" },
      "launch",
      expect.objectContaining({ enabled: true })
    );
    expect(result.current.memories.map(memory => memory.filename)).toEqual([
      "launch-brief-0425.md",
    ]);
    expect(result.current.searchInfo).toContain("Recall");
  });

  it("Should delete the selected memory using its full selector", async () => {
    const { result } = renderHook(() => useKnowledgePage());

    act(() => {
      result.current.setActiveScope("agent");
    });
    act(() => {
      result.current.setAgentName("cto");
    });

    await waitFor(() => {
      expect(result.current.memories).toHaveLength(1);
    });

    const memory = result.current.memories[0];
    if (!memory) throw new Error("expected an agent memory");
    await act(async () => {
      await result.current.handleDelete(memory);
    });

    expect(deleteMutateAsync).toHaveBeenCalledWith({
      selector: {
        scope: "agent",
        agentName: "cto",
        agentTier: "workspace",
        workspaceId: "ws_signalforge",
      },
      filename: "cto-tone.md",
    });
  });

  it("Should edit the selected memory through the controller mutation", async () => {
    const { result } = renderHook(() => useKnowledgePage());

    await waitFor(() => {
      expect(result.current.selectedMemory).toBeTruthy();
    });

    const memory = result.current.selectedMemory;
    if (!memory) throw new Error("expected a selected memory");
    await act(async () => {
      await result.current.handleEdit(memory, {
        content: "next body",
        description: "tightened",
      });
    });

    expect(editMutateAsync).toHaveBeenCalledWith({
      filename: "operator-playbook-0425.md",
      body: expect.objectContaining({
        content: "next body",
        description: "tightened",
        scope: "global",
      }),
    });
  });

  it("Should omit the immutable name and type from the edit request", async () => {
    const { result } = renderHook(() => useKnowledgePage());

    await waitFor(() => {
      expect(result.current.selectedMemory).toBeTruthy();
    });

    const memory = result.current.selectedMemory;
    if (!memory) throw new Error("expected a selected memory");
    await act(async () => {
      await result.current.handleEdit(memory, { content: "next body" });
    });

    const [params] = editMutateAsync.mock.calls.at(-1) ?? [];
    expect(params?.body).not.toHaveProperty("name");
    expect(params?.body).not.toHaveProperty("type");
  });

  it("Should write a new memory using the active selector and select the created filename", async () => {
    const { result } = renderHook(() => useKnowledgePage());

    act(() => {
      result.current.setActiveScope("workspace");
    });

    await act(async () => {
      await result.current.handleCreate({
        type: "project",
        name: "Launch Memory",
        description: "workspace contract",
        content: "Use the launch playbook.",
      });
    });

    expect(writeMutateAsync).toHaveBeenCalledWith({
      scope: "workspace",
      workspace_id: "ws_signalforge",
      agent_name: undefined,
      agent_tier: undefined,
      type: "project",
      name: "Launch Memory",
      description: "workspace contract",
      content: "Use the launch playbook.",
    });
    expect(result.current.createOpen).toBe(false);
  });

  it("Should request controller decisions for the selected memory before the server limit", async () => {
    useMemoryDecisionsMock.mockReturnValue({
      data: {
        decisions: [
          {
            id: "dec_match",
            candidate_hash: "h",
            op: "update",
            scope: "global",
            source: "rule",
            confidence: 0.9,
            decided_at: "2026-04-25T21:03:00Z",
            target_filename: "operator-playbook-0425.md",
            frontmatter: {
              filename: "operator-playbook-0425.md",
              mod_time: "2026-04-25T21:00:00Z",
              name: "Operator Playbook 0425",
              type: "reference",
            },
          },
        ],
      },
      isLoading: false,
      error: null,
    });

    const { result } = renderHook(() => useKnowledgePage());

    await waitFor(() => {
      expect(result.current.decisions).toHaveLength(1);
    });

    expect(result.current.decisions[0]?.id).toBe("dec_match");
    expect(useMemoryDecisionsMock).toHaveBeenLastCalledWith(
      {
        scope: "global",
        filename: "operator-playbook-0425.md",
        limit: 10,
      },
      expect.objectContaining({ enabled: true })
    );
  });

  it("Should revert an applied decision through the decision mutation", async () => {
    const decision: MemoryDecision = {
      id: "dec_match",
      candidate_hash: "h",
      op: "update",
      scope: "global",
      source: "rule",
      confidence: 0.9,
      decided_at: "2026-04-25T21:03:00Z",
      applied_at: "2026-04-25T21:03:01Z",
      target_filename: "operator-playbook-0425.md",
      frontmatter: {
        filename: "operator-playbook-0425.md",
        mod_time: "2026-04-25T21:00:00Z",
        name: "Operator Playbook 0425",
        type: "reference",
      },
    };
    const { result } = renderHook(() => useKnowledgePage());

    await act(async () => {
      await result.current.handleRevertDecision(decision);
    });

    expect(revertMutateAsync).toHaveBeenCalledWith({
      decisionID: "dec_match",
      body: { reason: "operator reverted from Knowledge" },
    });
  });

  it("Should clear delete error when the user changes the selected memory", async () => {
    const { result } = renderHook(() => useKnowledgePage());

    deleteMutateAsync.mockImplementationOnce(async () => {
      mockDeleteError = new Error("Delete failed");
      throw mockDeleteError;
    });

    await waitFor(() => {
      expect(result.current.selectedMemory).toBeTruthy();
    });

    const memory = result.current.selectedMemory;
    if (!memory) throw new Error("expected a selected memory");
    await act(async () => {
      await expect(result.current.handleDelete(memory)).rejects.toThrow("Delete failed");
    });

    await waitFor(() => {
      expect(result.current.deleteError).toBe("Delete failed");
    });

    act(() => {
      result.current.setSelectedMemoryKey("global:other.md");
    });

    expect(result.current.deleteError).toBeNull();
  });

  it("Should expose the search guard when search is empty", async () => {
    const { result } = renderHook(() => useKnowledgePage());

    await waitFor(() => {
      expect(result.current.searchActive).toBe(false);
      expect(result.current.searchInfo).toBeNull();
    });
  });
});

function catalogResult(memories: MemoryHeader[], total: number, hasNextPage: boolean) {
  return {
    data: memories,
    error: null,
    fetchNextPage,
    hasNextPage,
    isFetchNextPageError: mockMemoriesFetchNextPageError,
    isFetchingNextPage: false,
    isLoading: false,
    isRefetchError: mockMemoriesRefetchError,
    refetch: refetchMemories,
    total,
  };
}
