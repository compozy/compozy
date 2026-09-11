import { onlineManager, QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { resetProfileViews, setProfileView, useProfileReadScope } from "@/systems/profiles";

import type { AgentPayload } from "../../types";
import { agentKeys } from "../../lib/query-keys";
import { AgentDigestConflictError } from "../../adapters/agent-api";

const {
  mockUpdateAgent,
  mockDeleteAgent,
  mockDuplicateAgent,
  mockCreateAgent,
  mockFetchAgents,
  mockFetchAgent,
} = vi.hoisted(() => {
  return {
    mockUpdateAgent: vi.fn(),
    mockDeleteAgent: vi.fn(),
    mockDuplicateAgent: vi.fn(),
    mockCreateAgent: vi.fn(),
    mockFetchAgents: vi.fn(),
    mockFetchAgent: vi.fn(),
  };
});

vi.mock("../../adapters/agent-api", async () => {
  const actual = await vi.importActual<typeof import("../../adapters/agent-api")>(
    "../../adapters/agent-api"
  );
  return {
    ...actual,
    updateAgent: mockUpdateAgent,
    deleteAgent: mockDeleteAgent,
    duplicateAgent: mockDuplicateAgent,
    createAgent: mockCreateAgent,
    fetchAgents: mockFetchAgents,
    fetchAgent: mockFetchAgent,
  };
});

import {
  useAgent,
  useAgents,
  useCreateAgent,
  useDeleteAgent,
  useDuplicateAgent,
  useUpdateAgent,
} from "../use-agents";

function createWrapper(queryClient: QueryClient) {
  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

function makeAgent(overrides: Partial<AgentPayload> = {}): AgentPayload {
  return {
    name: "coder",
    provider: "claude",
    prompt: "Ship carefully.",
    definition_digest: "d1",
    origin: "workspace",
    ...overrides,
  };
}

describe("use-agent-mutations", () => {
  beforeEach(() => {
    resetProfileViews();
    mockUpdateAgent.mockReset();
    mockDeleteAgent.mockReset();
    mockDuplicateAgent.mockReset();
    mockCreateAgent.mockReset();
    mockFetchAgents.mockReset();
    mockFetchAgent.mockReset();
  });

  afterEach(() =>
    act(() => {
      onlineManager.setOnline(true);
      resetProfileViews();
    })
  );

  it.each(
    (["create", "update", "delete", "duplicate"] as const).flatMap(operation =>
      (["in flight", "offline"] as const).map(phase => ({ operation, phase }))
    )
  )(
    "Should bind $operation requests and completion caches to the starting profile while $phase",
    async ({ operation, phase }) => {
      setProfileView({ scope: "global" }, { kind: "profile", profile: "open-design" });
      const queryClient = new QueryClient({
        defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
      });
      const defaultKey = agentKeys.detail("coder", "ws_alpha", "default");
      const selectedKey = agentKeys.detail("coder", "ws_alpha", "open-design");
      const defaultAgent = makeAgent({ prompt: "Default profile" });
      const selectedAgent = makeAgent({ prompt: "Selected profile" });
      const updated = makeAgent({ prompt: "Changed", definition_digest: "d2" });
      queryClient.setQueryData(defaultKey, defaultAgent);
      queryClient.setQueryData(selectedKey, selectedAgent);
      let finish!: (value: AgentPayload) => void;
      const pending = new Promise<AgentPayload>(resolve => {
        finish = resolve;
      });
      const adapter = {
        create: mockCreateAgent,
        update: mockUpdateAgent,
        delete: mockDeleteAgent,
        duplicate: mockDuplicateAgent,
      }[operation];
      adapter.mockReturnValue(pending);
      const { result, unmount } = renderHook(
        () => ({
          profile: useProfileReadScope().destination,
          create: useCreateAgent(),
          update: useUpdateAgent(),
          delete: useDeleteAgent(),
          duplicate: useDuplicateAgent(),
        }),
        { wrapper: createWrapper(queryClient) }
      );
      const run = () => {
        switch (operation) {
          case "create":
            return result.current.create.mutateAsync({
              profile: result.current.profile,
              params: {
                scope: "workspace",
                workspace: "ws_alpha",
                agent: { name: updated.name, prompt: updated.prompt },
              },
            });
          case "update":
            return result.current.update.mutateAsync({
              profile: result.current.profile,
              name: "coder",
              cacheWorkspace: "ws_alpha",
              params: {
                workspace: "ws_alpha",
                expected_digest: "d1",
                agent: { name: updated.name, prompt: updated.prompt },
              },
            });
          case "delete":
            return result.current.delete.mutateAsync({
              profile: result.current.profile,
              name: "coder",
              workspace: "ws_alpha",
            });
          case "duplicate":
            return result.current.duplicate.mutateAsync({
              profile: result.current.profile,
              sourceName: "source",
              params: { name: "coder", scope: "workspace", workspace: "ws_alpha" },
            });
        }
      };
      let completion!: ReturnType<typeof run>;
      act(() => {
        onlineManager.setOnline(phase !== "offline");
        completion = run();
      });
      if (phase === "offline") {
        await waitFor(() => expect(result.current[operation].isPaused).toBe(true));
        expect(adapter).not.toHaveBeenCalled();
      } else {
        await waitFor(() => expect(adapter).toHaveBeenCalledOnce());
      }
      act(() => setProfileView({ scope: "global" }, { kind: "profile", profile: "default" }));
      await waitFor(() => expect(result.current.profile).toBe("default"));
      act(() => onlineManager.setOnline(true));
      await waitFor(() => expect(adapter).toHaveBeenCalledOnce());
      await act(async () => {
        finish(updated);
        await completion;
      });
      expect(adapter.mock.calls[0]?.at(-1)).toBe("open-design");
      expect(queryClient.getQueryData(defaultKey)).toEqual(defaultAgent);
      expect(queryClient.getQueryData(selectedKey)).toEqual(
        operation === "delete" ? undefined : updated
      );
      unmount();
      queryClient.clear();
    }
  );

  it("Should load agents via useAgents", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    mockFetchAgents.mockResolvedValue([makeAgent()]);
    const { result } = renderHook(() => useAgents("ws_alpha"), {
      wrapper: createWrapper(queryClient),
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockFetchAgents).toHaveBeenCalledWith("ws_alpha", expect.any(AbortSignal), "default");

    mockFetchAgent.mockResolvedValue(makeAgent());
    const detail = renderHook(() => useAgent("coder", "ws_alpha"), {
      wrapper: createWrapper(queryClient),
    });
    await waitFor(() => expect(detail.result.current.isSuccess).toBe(true));
    expect(mockFetchAgent).toHaveBeenCalledWith(
      "coder",
      "ws_alpha",
      expect.any(AbortSignal),
      "default"
    );
  });

  it("Should cache created agents on create success", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const created = makeAgent({ name: "new-agent" });
    mockCreateAgent.mockResolvedValue(created);
    const { result } = renderHook(() => useCreateAgent(), {
      wrapper: createWrapper(queryClient),
    });
    await act(async () => {
      await result.current.mutateAsync({
        profile: "default",
        params: {
          scope: "workspace",
          workspace: "ws_alpha",
          agent: { name: "new-agent", provider: "claude", prompt: "Ship carefully." },
        },
      });
    });
    expect(queryClient.getQueryData(agentKeys.detail("new-agent", "ws_alpha"))).toEqual(created);
  });

  it("Should set detail with fresh digest and invalidate lists plus catalogs on update", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const updated = makeAgent({ prompt: "Updated", definition_digest: "d2" });
    mockUpdateAgent.mockResolvedValue(updated);
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const { result } = renderHook(() => useUpdateAgent(), {
      wrapper: createWrapper(queryClient),
    });

    await act(async () => {
      await result.current.mutateAsync({
        profile: "default",
        name: "coder",
        cacheWorkspace: "ws_alpha",
        params: {
          expected_digest: "d1",
          agent: { name: "coder", provider: "claude", prompt: "Updated" },
          workspace: "ws_alpha",
        },
      });
    });

    expect(queryClient.getQueryData(agentKeys.detail("coder", "ws_alpha"))).toEqual(updated);
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: agentKeys.lists() });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: agentKeys.catalogs() });
  });

  it("Should write global winners to the active queryWorkspace key, not params.workspace null", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const otherWorkspaceKey = agentKeys.detail("fraud-ops-agent", "ws_other");
    const activeViewKey = agentKeys.detail("fraud-ops-agent", "ws_alpha");
    const globalNullKey = agentKeys.detail("fraud-ops-agent", null);
    const staleView = makeAgent({
      name: "fraud-ops-agent",
      origin: "global",
      prompt: "stale",
      definition_digest: "d1",
    });
    const updated = makeAgent({
      name: "fraud-ops-agent",
      origin: "global",
      prompt: "Updated global",
      definition_digest: "d2",
    });
    queryClient.setQueryData(activeViewKey, staleView);
    queryClient.setQueryData(otherWorkspaceKey, staleView);
    queryClient.setQueryData(globalNullKey, staleView);
    mockUpdateAgent.mockResolvedValue(updated);

    const { result } = renderHook(() => useUpdateAgent(), {
      wrapper: createWrapper(queryClient),
    });

    await act(async () => {
      await result.current.mutateAsync({
        profile: "default",
        name: "fraud-ops-agent",
        // Matches useAgent(name, activeWorkspaceId) even when origin is global.
        cacheWorkspace: "ws_alpha",
        params: {
          expected_digest: "d1",
          // Global update omits workspace — must not drive the cache key.
          agent: {
            name: "fraud-ops-agent",
            provider: "claude",
            prompt: "Updated global",
          },
        },
      });
    });

    expect(queryClient.getQueryData(activeViewKey)).toEqual(updated);
    expect(queryClient.getQueryData(otherWorkspaceKey)).toEqual(staleView);
    expect(queryClient.getQueryData(globalNullKey)).toEqual(staleView);
  });

  it("Should not poison detail cache on digest conflict", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const stale = makeAgent();
    queryClient.setQueryData(agentKeys.detail("coder", "ws_alpha"), stale);
    mockUpdateAgent.mockRejectedValue(new AgentDigestConflictError("definition digest conflict"));

    const { result } = renderHook(() => useUpdateAgent(), {
      wrapper: createWrapper(queryClient),
    });

    await act(async () => {
      await expect(
        result.current.mutateAsync({
          profile: "default",
          name: "coder",
          cacheWorkspace: "ws_alpha",
          params: {
            expected_digest: "stale",
            agent: { name: "coder", provider: "claude", prompt: "Nope" },
            workspace: "ws_alpha",
          },
        })
      ).rejects.toBeInstanceOf(AgentDigestConflictError);
    });

    expect(queryClient.getQueryData(agentKeys.detail("coder", "ws_alpha"))).toEqual(stale);
  });

  it("Should leave a global-view cache untouched when a rejected runtime choice conflicts", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const viewKey = agentKeys.detail("fraud-ops-agent", "ws_alpha");
    const serverTruth = makeAgent({
      name: "fraud-ops-agent",
      origin: "global",
      provider: "claude",
      model: "sonnet",
      definition_digest: "d1",
    });
    queryClient.setQueryData(viewKey, serverTruth);
    mockUpdateAgent.mockRejectedValue(new AgentDigestConflictError("definition digest conflict"));

    const { result } = renderHook(() => useUpdateAgent(), {
      wrapper: createWrapper(queryClient),
    });

    await act(async () => {
      await expect(
        result.current.mutateAsync({
          profile: "default",
          name: "fraud-ops-agent",
          cacheWorkspace: "ws_alpha",
          params: {
            expected_digest: "stale",
            agent: {
              name: "fraud-ops-agent",
              provider: "codex",
              model: "gpt-5.4",
              prompt: serverTruth.prompt,
            },
          },
        })
      ).rejects.toBeInstanceOf(AgentDigestConflictError);
    });

    expect(queryClient.getQueryData(viewKey)).toEqual(serverTruth);
    expect(queryClient.getQueryData(agentKeys.detail("fraud-ops-agent", null))).toBeUndefined();
  });

  it("Should remove detail, refresh nav counts, and return unshadowed_origin on delete", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    queryClient.setQueryData(agentKeys.detail("coder", "ws_alpha"), makeAgent());
    mockDeleteAgent.mockResolvedValue({
      name: "coder",
      origin: "workspace",
      unshadowed_origin: "global",
    });
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const { result } = renderHook(() => useDeleteAgent(), {
      wrapper: createWrapper(queryClient),
    });

    let response: unknown;
    await act(async () => {
      response = await result.current.mutateAsync({
        profile: "default",
        name: "coder",
        workspace: "ws_alpha",
      });
    });

    expect(response).toEqual({
      name: "coder",
      origin: "workspace",
      unshadowed_origin: "global",
    });
    expect(queryClient.getQueryData(agentKeys.detail("coder", "ws_alpha"))).toBeUndefined();
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: agentKeys.lists() });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: agentKeys.catalogs() });
  });

  it("Should cache the duplicated agent detail", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const copy = makeAgent({ name: "coder-copy", definition_digest: "d3" });
    mockDuplicateAgent.mockResolvedValue(copy);

    const { result } = renderHook(() => useDuplicateAgent(), {
      wrapper: createWrapper(queryClient),
    });

    await act(async () => {
      await result.current.mutateAsync({
        profile: "default",
        sourceName: "coder",
        params: {
          name: "coder-copy",
          scope: "workspace",
          workspace: "ws_alpha",
        },
      });
    });

    await waitFor(() => {
      expect(queryClient.getQueryData(agentKeys.detail("coder-copy", "ws_alpha"))).toEqual(copy);
    });
  });
});
