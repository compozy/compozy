import { onlineManager, QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { resetProfileViews, setProfileView, useProfileReadScope } from "@/systems/profiles";

import { agentKeys } from "../../lib/query-keys";

const {
  mockFetchSoul,
  mockPutSoul,
  mockDeleteSoul,
  mockValidateSoul,
  mockRollbackSoul,
  mockFetchSoulHistory,
} = vi.hoisted(() => ({
  mockFetchSoul: vi.fn(),
  mockPutSoul: vi.fn(),
  mockDeleteSoul: vi.fn(),
  mockValidateSoul: vi.fn(),
  mockRollbackSoul: vi.fn(),
  mockFetchSoulHistory: vi.fn(),
}));

vi.mock("../../adapters/agent-soul-api", () => ({
  fetchAgentSoul: mockFetchSoul,
  putAgentSoul: mockPutSoul,
  deleteAgentSoul: mockDeleteSoul,
  validateAgentSoul: mockValidateSoul,
  rollbackAgentSoul: mockRollbackSoul,
  fetchAgentSoulHistory: mockFetchSoulHistory,
}));

import {
  useAgentSoul,
  useAgentSoulHistory,
  useDeleteAgentSoul,
  usePutAgentSoul,
  useRollbackAgentSoul,
  useValidateAgentSoul,
} from "../use-agent-soul";

const soul = {
  active: true,
  present: true,
  enabled: true,
  body: "Be helpful.",
  digest: "a".repeat(64),
  valid: true,
  validation_status: "valid" as const,
  frontmatter: {},
  limits: { max_body_bytes: 65536 },
  config_provenance: {
    digest: "a".repeat(64),
    enabled: true,
    context_projection_bytes: 0,
    max_body_bytes: 65536,
  },
};

function createWrapper(queryClient: QueryClient) {
  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

describe("use-agent-soul", () => {
  beforeEach(() => {
    resetProfileViews();
    mockFetchSoul.mockReset();
    mockPutSoul.mockReset();
    mockDeleteSoul.mockReset();
    mockValidateSoul.mockReset();
    mockRollbackSoul.mockReset();
    mockFetchSoulHistory.mockReset();
    mockFetchSoul.mockResolvedValue(soul);
    mockPutSoul.mockResolvedValue({ soul, revision: { id: "r1" } });
    mockDeleteSoul.mockResolvedValue({ soul: { ...soul, active: false }, revision: { id: "r2" } });
    mockValidateSoul.mockResolvedValue(soul);
    mockRollbackSoul.mockResolvedValue({ soul, revision: { id: "r3" } });
    mockFetchSoulHistory.mockResolvedValue({ revisions: [] });
  });

  afterEach(() =>
    act(() => {
      onlineManager.setOnline(true);
      resetProfileViews();
    })
  );

  it("Should read and cache same-name soul resources independently for the selected profile", async () => {
    setProfileView({ scope: "global" }, { kind: "profile", profile: "open-design" });
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const selected = { ...soul, digest: "selected-profile" };
    const original = { ...soul, digest: "default-profile" };
    const selectedHistory = { revisions: [{ id: "selected-revision" }] };
    const defaultHistory = { revisions: [] };
    mockFetchSoul.mockImplementation(async (_name, _workspace, _signal, profile) =>
      profile === "open-design" ? selected : original
    );
    mockFetchSoulHistory.mockImplementation(async (_name, _workspace, _signal, profile) =>
      profile === "open-design" ? selectedHistory : defaultHistory
    );
    const { result, unmount } = renderHook(
      () => ({
        profile: useProfileReadScope().destination,
        file: useAgentSoul("coder", "ws_alpha"),
        history: useAgentSoulHistory("coder", "ws_alpha"),
      }),
      { wrapper: createWrapper(queryClient) }
    );
    await waitFor(() => expect(result.current.file.data).toEqual(selected));
    await waitFor(() => expect(result.current.history.data).toEqual(selectedHistory));
    expect(mockFetchSoul).toHaveBeenCalledWith(
      "coder",
      "ws_alpha",
      expect.any(AbortSignal),
      "open-design"
    );
    expect(mockFetchSoulHistory).toHaveBeenCalledWith(
      "coder",
      "ws_alpha",
      expect.any(AbortSignal),
      "open-design"
    );
    act(() => setProfileView({ scope: "global" }, { kind: "profile", profile: "default" }));
    await waitFor(() => expect(result.current.file.data).toEqual(original));
    await waitFor(() => expect(result.current.history.data).toEqual(defaultHistory));
    expect(queryClient.getQueryData(agentKeys.soul("coder", "ws_alpha", "open-design"))).toEqual(
      selected
    );
    expect(
      queryClient.getQueryData(agentKeys.soulHistory("coder", "ws_alpha", "open-design"))
    ).toEqual(selectedHistory);
    unmount();
    queryClient.clear();
  });

  it.each(
    (["put", "delete", "rollback", "validate"] as const).flatMap(operation =>
      (["in flight", "offline"] as const).map(phase => ({ operation, phase }))
    )
  )(
    "Should retain the original soul resource for $operation while $phase",
    async ({ operation, phase }) => {
      setProfileView({ scope: "global" }, { kind: "profile", profile: "open-design" });
      const queryClient = new QueryClient({
        defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
      });
      const originalKey = agentKeys.soul("coder", "ws_alpha", "open-design");
      const otherProfileKey = agentKeys.soul("coder", "ws_alpha", "default");
      const nextResourceKey = agentKeys.soul("reviewer", "ws_beta", "default");
      const originalHistoryKey = agentKeys.soulHistory("coder", "ws_alpha", "open-design");
      const otherHistoryKey = agentKeys.soulHistory("coder", "ws_alpha", "default");
      const original = { ...soul, digest: "original" };
      const other = { ...soul, digest: "other-profile" };
      const next = { ...soul, digest: "next-resource" };
      const updated = { ...soul, digest: "changed" };
      queryClient.setQueryData(originalKey, original);
      queryClient.setQueryData(otherProfileKey, other);
      queryClient.setQueryData(nextResourceKey, next);
      queryClient.setQueryData(originalHistoryKey, { revisions: [] });
      queryClient.setQueryData(otherHistoryKey, { revisions: [] });
      let finish!: () => void;
      const pending = new Promise(resolve => {
        finish = () =>
          resolve(
            operation === "validate"
              ? updated
              : { soul: updated, revision: { id: "r2" }, decision: { result: "sent" } }
          );
      });
      const adapter = {
        put: mockPutSoul,
        delete: mockDeleteSoul,
        rollback: mockRollbackSoul,
        validate: mockValidateSoul,
      }[operation];
      adapter.mockReturnValue(pending);
      const { result, rerender, unmount } = renderHook(
        ({ name, workspace }) => ({
          name,
          workspace,
          profile: useProfileReadScope().destination,
          put: usePutAgentSoul(),
          delete: useDeleteAgentSoul(),
          rollback: useRollbackAgentSoul(),
          validate: useValidateAgentSoul(),
        }),
        {
          initialProps: { name: "coder", workspace: "ws_alpha" },
          wrapper: createWrapper(queryClient),
        }
      );
      const run = () => {
        const scope = {
          name: result.current.name,
          cacheWorkspace: result.current.workspace,
          profile: result.current.profile,
        };
        switch (operation) {
          case "put":
            return result.current.put.mutateAsync({
              ...scope,
              params: {
                workspace_id: "ws_alpha",
                body: "changed",
                expected_digest: "a".repeat(64),
              },
            });
          case "delete":
            return result.current.delete.mutateAsync({
              ...scope,
              params: { workspace_id: "ws_alpha", expected_digest: "a".repeat(64) },
            });
          case "rollback":
            return result.current.rollback.mutateAsync({
              ...scope,
              params: {
                workspace_id: "ws_alpha",
                revision_id: "r1",
                expected_digest: "a".repeat(64),
              },
            });
          case "validate":
            return result.current.validate.mutateAsync({
              ...scope,
              params: { workspace_id: "ws_alpha", body: "changed" },
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
      rerender({ name: "reviewer", workspace: "ws_beta" });
      await waitFor(() => expect(result.current.profile).toBe("default"));
      act(() => onlineManager.setOnline(true));
      await waitFor(() => expect(adapter).toHaveBeenCalledOnce());
      await act(async () => {
        finish();
        await completion;
      });
      expect(adapter.mock.calls[0]?.[0]).toBe("coder");
      expect(adapter.mock.calls[0]?.[3]).toBe("open-design");
      expect(adapter.mock.calls[0]?.[1]).toMatchObject({ workspace_id: "ws_alpha" });
      expect(queryClient.getQueryData(originalKey)).toEqual(
        operation === "put" || operation === "rollback" ? updated : original
      );
      expect(queryClient.getQueryData(otherProfileKey)).toEqual(other);
      expect(queryClient.getQueryData(nextResourceKey)).toEqual(next);
      expect(queryClient.getQueryState(originalKey)?.isInvalidated).toBe(operation !== "validate");
      expect(queryClient.getQueryState(otherProfileKey)?.isInvalidated).toBe(false);
      expect(queryClient.getQueryState(nextResourceKey)?.isInvalidated).toBe(false);
      expect(queryClient.getQueryState(originalHistoryKey)?.isInvalidated).toBe(
        operation !== "validate"
      );
      expect(queryClient.getQueryState(otherHistoryKey)?.isInvalidated).toBe(false);
      unmount();
      queryClient.clear();
    }
  );

  it("Should load soul and cache put results", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const { result } = renderHook(() => useAgentSoul("coder", "ws_alpha"), {
      wrapper: createWrapper(queryClient),
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockFetchSoul).toHaveBeenCalledWith(
      "coder",
      "ws_alpha",
      expect.any(AbortSignal),
      "default"
    );

    const history = renderHook(() => useAgentSoulHistory("coder", "ws_alpha"), {
      wrapper: createWrapper(queryClient),
    });
    await waitFor(() => expect(history.result.current.isSuccess).toBe(true));
    expect(mockFetchSoulHistory).toHaveBeenCalled();

    const putSoul = { ...soul, body: "Be sharper.", digest: "c".repeat(64) };
    mockPutSoul.mockImplementation(async () => {
      mockFetchSoul.mockResolvedValue(putSoul);
      return { soul: putSoul, revision: { id: "r1" } };
    });

    const put = renderHook(() => usePutAgentSoul(), {
      wrapper: createWrapper(queryClient),
    });
    await act(async () => {
      await put.result.current.mutateAsync({
        name: "coder",
        cacheWorkspace: "ws_alpha",
        profile: "default",
        params: {
          body: "Be sharper.",
          expected_digest: "a".repeat(64),
        },
      });
    });
    expect(mockPutSoul).toHaveBeenCalledWith(
      "coder",
      {
        body: "Be sharper.",
        expected_digest: "a".repeat(64),
      },
      undefined,
      "default"
    );
    await waitFor(() => {
      expect(queryClient.getQueryData(agentKeys.soul("coder", "ws_alpha"))).toEqual(putSoul);
    });
  });

  it("Should validate, delete, and rollback through mutation hooks", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const rolledBackSoul = { ...soul, body: "Restored.", digest: "d".repeat(64) };
    mockRollbackSoul.mockImplementation(async () => {
      mockFetchSoul.mockResolvedValue(rolledBackSoul);
      return { soul: rolledBackSoul, revision: { id: "r3" } };
    });

    const validate = renderHook(() => useValidateAgentSoul(), {
      wrapper: createWrapper(queryClient),
    });
    await act(async () => {
      await validate.result.current.mutateAsync({
        name: "coder",
        cacheWorkspace: "ws_alpha",
        profile: "default",
        params: { body: "Be helpful." },
      });
    });
    expect(mockValidateSoul).toHaveBeenCalledWith(
      "coder",
      { body: "Be helpful." },
      undefined,
      "default"
    );

    const del = renderHook(() => useDeleteAgentSoul(), {
      wrapper: createWrapper(queryClient),
    });
    await act(async () => {
      await del.result.current.mutateAsync({
        name: "coder",
        cacheWorkspace: "ws_alpha",
        profile: "default",
        params: { expected_digest: "a".repeat(64) },
      });
    });
    expect(mockDeleteSoul).toHaveBeenCalledWith(
      "coder",
      { expected_digest: "a".repeat(64) },
      undefined,
      "default"
    );

    const rollback = renderHook(() => useRollbackAgentSoul(), {
      wrapper: createWrapper(queryClient),
    });
    await act(async () => {
      await rollback.result.current.mutateAsync({
        name: "coder",
        cacheWorkspace: "ws_alpha",
        profile: "default",
        params: {
          revision_id: "r1",
          expected_digest: "a".repeat(64),
        },
      });
    });
    expect(mockRollbackSoul).toHaveBeenCalledWith(
      "coder",
      {
        revision_id: "r1",
        expected_digest: "a".repeat(64),
      },
      undefined,
      "default"
    );
    await waitFor(() => {
      expect(queryClient.getQueryData(agentKeys.soul("coder", "ws_alpha"))).toEqual(rolledBackSoul);
    });
  });
});
