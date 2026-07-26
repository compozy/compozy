import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { expectFetchRequest, mockJsonResponse } from "@/test/fetch-test-utils";
import {
  deleteMemory,
  editMemory,
  KnowledgeApiError,
  listMemories,
  listMemoryDecisions,
  readMemory,
  revertMemoryDecision,
  searchMemory,
  triggerMemoryDream,
  writeMemory,
} from "@/systems/knowledge/adapters/knowledge-api";
import {
  memoryDecisionsFixture,
  memoryDecisionRevertFixture,
  memoryDeleteFixture,
  memoryDreamTriggerFixture,
  memoryEditFixture,
  memorySearchFixture,
  memoryWriteFixture,
} from "@/systems/knowledge/mocks";

const validHeader = {
  filename: "user_role.md",
  mod_time: "2026-04-01T12:00:00Z",
  name: "User Role",
  scope: "global",
  type: "user",
  recall_count: 0,
  injection: true,
  system_managed: false,
} as const;

const memoryListPageFixture = {
  memories: [validHeader],
  page: { has_more: true, limit: 1, next_cursor: "memory-next", total: 3 },
};

beforeEach(() => {
  vi.stubGlobal("fetch", vi.fn());
});

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe("listMemories", () => {
  it("Should preserve the counted page and send every stable filter plus cursor", async () => {
    mockJsonResponse(memoryListPageFixture);

    const result = await listMemories({
      scope: "agent",
      workspaceId: "ws_launch",
      agentName: "cto",
      agentTier: "workspace",
      type: "reference",
      sort: "recent",
      includeSystem: true,
      cursor: "memory-cursor",
      limit: 25,
    });

    expect(result).toEqual(memoryListPageFixture);
    await expectFetchRequest({
      path: "/api/memory?scope=agent&workspace_id=ws_launch&agent_name=cto&agent_tier=workspace&type=reference&sort=recent&cursor=memory-cursor&limit=25&include_system=true",
    });
  });

  it("Should call GET /api/memory with no params when no selector is provided", async () => {
    mockJsonResponse({
      memories: [],
      page: { has_more: false, limit: 50, total: 0 },
    });

    const result = await listMemories();

    expect(result).toEqual({
      memories: [],
      page: { has_more: false, limit: 50, total: 0 },
    });
    await expectFetchRequest({ path: "/api/memory" });
  });

  it("Should pass abort signal to fetch", async () => {
    mockJsonResponse({ memories: [], page: { has_more: false, limit: 50, total: 0 } });
    const controller = new AbortController();

    await listMemories({ scope: "global" }, controller.signal);

    await expectFetchRequest({
      path: "/api/memory?scope=global",
      signal: controller.signal,
    });
  });

  it("Should throw KnowledgeApiError on non-2xx response", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 500 }));

    await expect(listMemories()).rejects.toThrow(KnowledgeApiError);
    await expect(listMemories()).rejects.toThrow("Failed to fetch memories: 500");
  });
});

describe("readMemory", () => {
  it("Should call GET /api/memory/:filename with the selector and return summary + content", async () => {
    mockJsonResponse({ memory: { summary: validHeader, content: "# Memory content" } });

    const result = await readMemory({ scope: "global" }, "user_role.md");

    expect(result).toMatchObject({ filename: "user_role.md", content: "# Memory content" });
    await expectFetchRequest({ path: "/api/memory/user_role.md?scope=global" });
  });

  it("Should pass agent and workspace selectors to the query string", async () => {
    mockJsonResponse({ memory: { summary: validHeader, content: "data" } });

    await readMemory(
      {
        scope: "agent",
        workspaceId: "ws_launch",
        agentName: "cto",
        agentTier: "workspace",
      },
      "project_ctx.md"
    );

    await expectFetchRequest({
      path: "/api/memory/project_ctx.md?scope=agent&workspace_id=ws_launch&agent_name=cto&agent_tier=workspace",
    });
  });

  it("Should throw KnowledgeApiError with 404 for unknown memory", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 404 }));

    await expect(readMemory({ scope: "global" }, "missing.md")).rejects.toThrow(
      "Memory not found: missing.md"
    );

    try {
      await readMemory({ scope: "global" }, "missing.md");
    } catch (error) {
      expect(error).toBeInstanceOf(KnowledgeApiError);
      expect((error as KnowledgeApiError).status).toBe(404);
    }
  });

  it("Should throw KnowledgeApiError on other failures", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 503 }));

    await expect(readMemory({ scope: "global" }, "test.md")).rejects.toThrow(
      'Failed to read memory "test.md": 503'
    );
  });

  it("Should encode filename in the URL", async () => {
    mockJsonResponse({ memory: { summary: validHeader, content: "" } });

    await readMemory({ scope: "global" }, "my file.md");

    await expectFetchRequest({ path: "/api/memory/my%20file.md?scope=global" });
  });
});

describe("writeMemory", () => {
  it("Should call POST /api/memory with the controller proposal body", async () => {
    mockJsonResponse(memoryWriteFixture);

    const result = await writeMemory({
      scope: "global",
      type: "reference",
      name: "Test memory",
      content: "content here",
      workspace_id: "ws_launch",
    });

    expect(result).toEqual(memoryWriteFixture);
    await expectFetchRequest({
      body: {
        content: "content here",
        name: "Test memory",
        scope: "global",
        type: "reference",
        workspace_id: "ws_launch",
      },
      method: "POST",
      path: "/api/memory",
    });
  });

  it("Should throw KnowledgeApiError on non-2xx response", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 400 }));

    const body = {
      scope: "global",
      type: "reference",
      name: "Test memory",
      content: "bad",
    } as const;
    await expect(writeMemory(body)).rejects.toThrow(KnowledgeApiError);
    await expect(writeMemory(body)).rejects.toThrow("Failed to write memory: 400");
  });
});

describe("editMemory", () => {
  it("Should call PATCH /api/memory/:filename with the controller edit body", async () => {
    mockJsonResponse(memoryEditFixture);

    const result = await editMemory("operator-style.md", {
      content: "updated body",
      description: "tightened tone",
      scope: "global",
      type: "user",
      name: "Operator Style",
    });

    expect(result).toEqual(memoryEditFixture);
    await expectFetchRequest({
      body: {
        content: "updated body",
        description: "tightened tone",
        scope: "global",
        type: "user",
        name: "Operator Style",
      },
      method: "PATCH",
      path: "/api/memory/operator-style.md",
    });
  });

  it("Should surface 404 when the file is missing", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 404 }));

    await expect(editMemory("missing.md", { content: "x" })).rejects.toThrow(
      "Memory not found: missing.md"
    );
  });

  it("Should throw KnowledgeApiError on policy rejection", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 422 }));

    await expect(editMemory("operator-style.md", { content: "x" })).rejects.toThrow(
      'Failed to edit memory "operator-style.md": 422'
    );
  });
});

describe("deleteMemory", () => {
  it("Should call DELETE /api/memory/:filename with the selector", async () => {
    mockJsonResponse(memoryDeleteFixture);

    const result = await deleteMemory({ scope: "global" }, "old.md");

    expect(result).toEqual(memoryDeleteFixture);
    await expectFetchRequest({
      method: "DELETE",
      path: "/api/memory/old.md?scope=global",
    });
  });

  it("Should pass agent and workspace selectors to the query string", async () => {
    mockJsonResponse(memoryDeleteFixture);

    await deleteMemory(
      {
        scope: "agent",
        workspaceId: "ws_launch",
        agentName: "cto",
        agentTier: "workspace",
      },
      "project.md"
    );

    await expectFetchRequest({
      method: "DELETE",
      path: "/api/memory/project.md?scope=agent&workspace_id=ws_launch&agent_name=cto&agent_tier=workspace",
    });
  });

  it("Should throw KnowledgeApiError with 404 for unknown memory", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 404 }));

    await expect(deleteMemory({ scope: "global" }, "missing.md")).rejects.toThrow(
      "Memory not found: missing.md"
    );
  });
});

describe("searchMemory", () => {
  it("Should POST /api/memory/search with the selector body and return results", async () => {
    mockJsonResponse(memorySearchFixture);

    const result = await searchMemory({
      query_text: "launch",
      scope: "workspace",
      workspace_id: "ws_launch",
      top_k: 3,
    });

    expect(result).toEqual(memorySearchFixture);
    await expectFetchRequest({
      body: {
        query_text: "launch",
        scope: "workspace",
        workspace_id: "ws_launch",
        top_k: 3,
      },
      method: "POST",
      path: "/api/memory/search",
    });
  });

  it("Should throw KnowledgeApiError on non-2xx response", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 500 }));

    await expect(searchMemory({ query_text: "x" })).rejects.toThrow(KnowledgeApiError);
  });
});

describe("listMemoryDecisions", () => {
  it("Should call GET /api/memory/decisions with selector + filter params", async () => {
    mockJsonResponse(memoryDecisionsFixture);

    const result = await listMemoryDecisions({
      scope: "agent",
      agentName: "cto",
      agentTier: "workspace",
      workspaceId: "ws_launch",
      op: "update",
      filename: "launch.md",
      since: "2026-04-25T21:00:00Z",
      limit: 5,
    });

    expect(result).toEqual(memoryDecisionsFixture);
    await expectFetchRequest({
      path: "/api/memory/decisions?scope=agent&workspace_id=ws_launch&agent_name=cto&agent_tier=workspace&op=update&filename=launch.md&since=2026-04-25T21%3A00%3A00Z&limit=5",
    });
  });

  it("Should surface daemon errors as KnowledgeApiError", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 500 }));

    await expect(listMemoryDecisions({ scope: "global" })).rejects.toThrow(KnowledgeApiError);
  });
});

describe("revertMemoryDecision", () => {
  it("Should POST the revert body to the decision-specific endpoint", async () => {
    mockJsonResponse(memoryDecisionRevertFixture);

    const result = await revertMemoryDecision("dec_edit_fixture", {
      reason: "operator reverted from Knowledge",
    });

    expect(result).toEqual(memoryDecisionRevertFixture);
    await expectFetchRequest({
      body: { reason: "operator reverted from Knowledge" },
      method: "POST",
      path: "/api/memory/decisions/dec_edit_fixture/revert",
    });
  });

  it("Should surface 404 for unknown decisions", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 404 }));

    await expect(revertMemoryDecision("missing_decision")).rejects.toThrow(
      "Memory decision not found: missing_decision"
    );
  });
});

describe("triggerMemoryDream", () => {
  it("Should POST /api/memory/dreams/trigger with the workspace id", async () => {
    mockJsonResponse(memoryDreamTriggerFixture);

    const result = await triggerMemoryDream("ws_launch");

    expect(result).toEqual(memoryDreamTriggerFixture);
    await expectFetchRequest({
      body: { workspace_id: "ws_launch" },
      method: "POST",
      path: "/api/memory/dreams/trigger",
    });
  });

  it("Should surface daemon errors as KnowledgeApiError", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 500 }));

    await expect(triggerMemoryDream()).rejects.toThrow(KnowledgeApiError);
    await expect(triggerMemoryDream()).rejects.toThrow("Failed to trigger memory dreaming: 500");
  });
});

describe("KnowledgeApiError", () => {
  it("Should expose name, message, and status", () => {
    const error = new KnowledgeApiError("test error", 404);

    expect(error.name).toBe("KnowledgeApiError");
    expect(error.status).toBe(404);
    expect(error.message).toBe("test error");
    expect(error).toBeInstanceOf(Error);
  });
});
