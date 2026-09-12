import { onlineManager, QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { resetProfileViews, setProfileView, useProfileReadScope } from "@/systems/profiles";

import { agentKeys } from "../../lib/query-keys";

const {
  mockFetchHeartbeat,
  mockPutHeartbeat,
  mockDeleteHeartbeat,
  mockValidateHeartbeat,
  mockRollbackHeartbeat,
  mockFetchHistory,
  mockFetchStatus,
  mockWake,
} = vi.hoisted(() => ({
  mockFetchHeartbeat: vi.fn(),
  mockPutHeartbeat: vi.fn(),
  mockDeleteHeartbeat: vi.fn(),
  mockValidateHeartbeat: vi.fn(),
  mockRollbackHeartbeat: vi.fn(),
  mockFetchHistory: vi.fn(),
  mockFetchStatus: vi.fn(),
  mockWake: vi.fn(),
}));

vi.mock("../../adapters/agent-heartbeat-api", () => ({
  fetchAgentHeartbeat: mockFetchHeartbeat,
  putAgentHeartbeat: mockPutHeartbeat,
  deleteAgentHeartbeat: mockDeleteHeartbeat,
  validateAgentHeartbeat: mockValidateHeartbeat,
  rollbackAgentHeartbeat: mockRollbackHeartbeat,
  fetchAgentHeartbeatHistory: mockFetchHistory,
  fetchAgentHeartbeatStatus: mockFetchStatus,
  wakeAgentHeartbeat: mockWake,
}));

import {
  useAgentHeartbeat,
  useAgentHeartbeatHistory,
  useAgentHeartbeatStatus,
  useDeleteAgentHeartbeat,
  usePutAgentHeartbeat,
  useRollbackAgentHeartbeat,
  useValidateAgentHeartbeat,
  useWakeAgentHeartbeat,
} from "../use-agent-heartbeat";

const heartbeat = {
  active: true,
  present: true,
  enabled: true,
  valid: true,
  validation_status: "valid" as const,
  digest: "b".repeat(64),
  schema_version: 1,
  frontmatter: { enabled: true, version: 1, context: {}, preferences: {} },
  limits: { max_body_bytes: 65536 },
  preferences: { min_interval: "30m", context: {} },
  prompt: {
    active: true,
    context: {},
    max_body_bytes: 65536,
    max_bytes: 65536,
    preferences: { context: {}, min_interval: "30m" },
    truncated: false,
  },
  config_provenance: {
    digest: "b".repeat(64),
    subset: {
      active_session_only: false,
      allow_active_hours_preferences: true,
      context_projection_bytes: 0,
      default_interval: "30m",
      enabled: true,
      max_body_bytes: 65536,
      max_wakes_per_cycle: 1,
      min_interval: "5m",
      session_health_hook_min_interval: "1m",
      session_health_stale_after: "10m",
      wake_cooldown: "1m",
      wake_event_retention: "24h",
    },
  },
};

function createWrapper(queryClient: QueryClient) {
  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

describe("use-agent-heartbeat", () => {
  beforeEach(() => {
    resetProfileViews();
    for (const mock of [
      mockFetchHeartbeat,
      mockPutHeartbeat,
      mockDeleteHeartbeat,
      mockValidateHeartbeat,
      mockRollbackHeartbeat,
      mockFetchHistory,
      mockFetchStatus,
      mockWake,
    ]) {
      mock.mockReset();
    }
    mockFetchHeartbeat.mockResolvedValue(heartbeat);
    mockPutHeartbeat.mockResolvedValue({ heartbeat, revision: { id: "r1" } });
    mockDeleteHeartbeat.mockResolvedValue({
      heartbeat: { ...heartbeat, active: false },
      revision: { id: "r2" },
    });
    mockValidateHeartbeat.mockResolvedValue(heartbeat);
    mockRollbackHeartbeat.mockResolvedValue({ heartbeat, revision: { id: "r3" } });
    mockFetchHistory.mockResolvedValue({ revisions: [] });
    mockFetchStatus.mockResolvedValue({
      agent_name: "coder",
      active: true,
      present: true,
      enabled: true,
      valid: true,
      validation_status: "valid",
      preferences: { min_interval: "30m", context: {} },
    });
    mockWake.mockResolvedValue({
      decision: { result: "sent", reason: "wake_sent", wake_event_id: "wake-1" },
    });
  });

  afterEach(() =>
    act(() => {
      onlineManager.setOnline(true);
      resetProfileViews();
    })
  );

  it("Should read and cache same-name heartbeat resources independently for the selected profile", async () => {
    setProfileView({ scope: "global" }, { kind: "profile", profile: "open-design" });
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const selected = { ...heartbeat, digest: "selected-profile" };
    const original = { ...heartbeat, digest: "default-profile" };
    const selectedHistory = { revisions: [{ id: "selected-revision" }] };
    const defaultHistory = { revisions: [] };
    mockFetchHeartbeat.mockImplementation(async (_name, _workspace, _signal, profile) =>
      profile === "open-design" ? selected : original
    );
    mockFetchHistory.mockImplementation(async (_name, _workspace, _signal, profile) =>
      profile === "open-design" ? selectedHistory : defaultHistory
    );
    const selectedStatus = { agent_name: "coder", enabled: true };
    const defaultStatus = { agent_name: "coder", enabled: false };
    mockFetchStatus.mockImplementation(async (_name, options) =>
      options.profile === "open-design" ? selectedStatus : defaultStatus
    );
    const { result, unmount } = renderHook(
      () => ({
        profile: useProfileReadScope().destination,
        file: useAgentHeartbeat("coder", "ws_alpha"),
        history: useAgentHeartbeatHistory("coder", "ws_alpha"),
        status: useAgentHeartbeatStatus("coder", { workspaceId: "ws_alpha" }),
      }),
      { wrapper: createWrapper(queryClient) }
    );
    await waitFor(() => expect(result.current.file.data).toEqual(selected));
    await waitFor(() => expect(result.current.history.data).toEqual(selectedHistory));
    expect(mockFetchHeartbeat).toHaveBeenCalledWith(
      "coder",
      "ws_alpha",
      expect.any(AbortSignal),
      "open-design"
    );
    expect(mockFetchHistory).toHaveBeenCalledWith(
      "coder",
      "ws_alpha",
      expect.any(AbortSignal),
      "open-design"
    );
    await waitFor(() => expect(result.current.status.data).toEqual(selectedStatus));
    expect(mockFetchStatus).toHaveBeenCalledWith(
      "coder",
      { workspaceId: "ws_alpha", profile: "open-design" },
      expect.any(AbortSignal)
    );
    act(() => setProfileView({ scope: "global" }, { kind: "profile", profile: "default" }));
    await waitFor(() => expect(result.current.file.data).toEqual(original));
    await waitFor(() => expect(result.current.history.data).toEqual(defaultHistory));
    expect(
      queryClient.getQueryData(agentKeys.heartbeat("coder", "ws_alpha", "open-design"))
    ).toEqual(selected);
    expect(
      queryClient.getQueryData(agentKeys.heartbeatHistory("coder", "ws_alpha", "open-design"))
    ).toEqual(selectedHistory);
    await waitFor(() => expect(result.current.status.data).toEqual(defaultStatus));
    expect(
      queryClient.getQueryData(
        agentKeys.heartbeatStatus("coder", { workspaceId: "ws_alpha", profile: "open-design" })
      )
    ).toEqual(selectedStatus);
    unmount();
    queryClient.clear();
  });

  it.each(
    (["put", "delete", "rollback", "validate", "wake"] as const).flatMap(operation =>
      (["in flight", "offline"] as const).map(phase => ({ operation, phase }))
    )
  )(
    "Should retain the original heartbeat resource for $operation while $phase",
    async ({ operation, phase }) => {
      setProfileView({ scope: "global" }, { kind: "profile", profile: "open-design" });
      const queryClient = new QueryClient({
        defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
      });
      const originalKey = agentKeys.heartbeat("coder", "ws_alpha", "open-design");
      const otherProfileKey = agentKeys.heartbeat("coder", "ws_alpha", "default");
      const nextResourceKey = agentKeys.heartbeat("reviewer", "ws_beta", "default");
      const originalHistoryKey = agentKeys.heartbeatHistory("coder", "ws_alpha", "open-design");
      const otherHistoryKey = agentKeys.heartbeatHistory("coder", "ws_alpha", "default");
      const original = { ...heartbeat, digest: "original" };
      const other = { ...heartbeat, digest: "other-profile" };
      const next = { ...heartbeat, digest: "next-resource" };
      const updated = { ...heartbeat, digest: "changed" };
      queryClient.setQueryData(originalKey, original);
      queryClient.setQueryData(otherProfileKey, other);
      queryClient.setQueryData(nextResourceKey, next);
      queryClient.setQueryData(originalHistoryKey, { revisions: [] });
      queryClient.setQueryData(otherHistoryKey, { revisions: [] });
      const originalStatusKey = agentKeys.heartbeatStatus("coder", {
        workspaceId: "ws_alpha",
        profile: "open-design",
      });
      const otherStatusKey = agentKeys.heartbeatStatus("coder", {
        workspaceId: "ws_alpha",
        profile: "default",
      });
      queryClient.setQueryData(originalStatusKey, { enabled: true });
      queryClient.setQueryData(otherStatusKey, { enabled: false });
      let finish!: () => void;
      const pending = new Promise(resolve => {
        finish = () =>
          resolve(
            operation === "validate"
              ? updated
              : { heartbeat: updated, revision: { id: "r2" }, decision: { result: "sent" } }
          );
      });
      const adapter = {
        put: mockPutHeartbeat,
        delete: mockDeleteHeartbeat,
        rollback: mockRollbackHeartbeat,
        validate: mockValidateHeartbeat,
        wake: mockWake,
      }[operation];
      adapter.mockReturnValue(pending);
      const { result, rerender, unmount } = renderHook(
        ({ name, workspace }) => ({
          name,
          workspace,
          profile: useProfileReadScope().destination,
          put: usePutAgentHeartbeat(),
          delete: useDeleteAgentHeartbeat(),
          rollback: useRollbackAgentHeartbeat(),
          validate: useValidateAgentHeartbeat(),
          wake: useWakeAgentHeartbeat(),
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
                expected_digest: "b".repeat(64),
              },
            });
          case "delete":
            return result.current.delete.mutateAsync({
              ...scope,
              params: { workspace_id: "ws_alpha", expected_digest: "b".repeat(64) },
            });
          case "rollback":
            return result.current.rollback.mutateAsync({
              ...scope,
              params: {
                workspace_id: "ws_alpha",
                revision_id: "r1",
                expected_digest: "b".repeat(64),
              },
            });
          case "validate":
            return result.current.validate.mutateAsync({
              ...scope,
              params: { workspace_id: "ws_alpha", body: "changed" },
            });
          case "wake":
            return result.current.wake.mutateAsync({
              ...scope,
              params: { session_id: "sess-original", source: "manual" },
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
      if (operation === "wake") {
        expect(adapter.mock.calls[0]?.[1]).toEqual({
          session_id: "sess-original",
          source: "manual",
        });
      } else {
        expect(adapter.mock.calls[0]?.[1]).toMatchObject({ workspace_id: "ws_alpha" });
      }
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
      expect(queryClient.getQueryState(originalStatusKey)?.isInvalidated).toBe(
        operation !== "validate"
      );
      expect(queryClient.getQueryState(otherStatusKey)?.isInvalidated).toBe(false);
      unmount();
      queryClient.clear();
    }
  );

  it("Should load heartbeat/status and cache put results", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const { result } = renderHook(() => useAgentHeartbeat("coder", "ws_alpha"), {
      wrapper: createWrapper(queryClient),
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    const history = renderHook(() => useAgentHeartbeatHistory("coder", "ws_alpha"), {
      wrapper: createWrapper(queryClient),
    });
    await waitFor(() => expect(history.result.current.isSuccess).toBe(true));

    const status = renderHook(() => useAgentHeartbeatStatus("coder", { workspaceId: "ws_alpha" }), {
      wrapper: createWrapper(queryClient),
    });
    await waitFor(() => expect(status.result.current.isSuccess).toBe(true));

    const putHeartbeat = {
      ...heartbeat,
      digest: "c".repeat(64),
      guidance_markdown: "updated guidance",
    };
    mockPutHeartbeat.mockImplementation(async () => {
      mockFetchHeartbeat.mockResolvedValue(putHeartbeat);
      return { heartbeat: putHeartbeat, revision: { id: "r1" } };
    });

    const put = renderHook(() => usePutAgentHeartbeat(), {
      wrapper: createWrapper(queryClient),
    });
    await act(async () => {
      await put.result.current.mutateAsync({
        name: "coder",
        cacheWorkspace: "ws_alpha",
        profile: "default",
        params: {
          body: "---\nenabled: true\n---\n",
          expected_digest: "b".repeat(64),
        },
      });
    });
    expect(mockPutHeartbeat).toHaveBeenCalledWith(
      "coder",
      {
        body: "---\nenabled: true\n---\n",
        expected_digest: "b".repeat(64),
      },
      undefined,
      "default"
    );
    await waitFor(() => {
      expect(queryClient.getQueryData(agentKeys.heartbeat("coder", "ws_alpha"))).toEqual(
        putHeartbeat
      );
    });
  });

  it("Should refresh selected-session eligibility while heartbeat operations stay mounted", async () => {
    vi.useFakeTimers();
    try {
      const queryClient = new QueryClient({
        defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
      });
      const ineligibleStatus = {
        agent_name: "coder",
        active: true,
        present: true,
        enabled: true,
        valid: true,
        validation_status: "valid" as const,
        preferences: { min_interval: "30m", context: {} },
        session_health: {
          session_id: "sess-1",
          workspace_id: "ws_alpha",
          agent_name: "coder",
          state: "prompting" as const,
          health: "healthy" as const,
          active_prompt: true,
          attachable: true,
          eligible_for_wake: false,
          ineligibility_reason: "session_prompt_active" as const,
          updated_at: "2026-07-29T13:11:38Z",
        },
      };
      mockFetchStatus.mockResolvedValueOnce(ineligibleStatus).mockResolvedValueOnce({
        ...ineligibleStatus,
        session_health: {
          ...ineligibleStatus.session_health,
          state: "idle",
          active_prompt: false,
          eligible_for_wake: true,
          ineligibility_reason: undefined,
          updated_at: "2026-07-29T13:11:39Z",
        },
      });

      const status = renderHook(
        () =>
          useAgentHeartbeatStatus("coder", {
            workspaceId: "ws_alpha",
            sessionId: "sess-1",
            includeSessionHealth: true,
          }),
        { wrapper: createWrapper(queryClient) }
      );

      await act(async () => {
        await vi.advanceTimersByTimeAsync(0);
      });
      expect(status.result.current.data?.session_health?.eligible_for_wake).toBe(false);

      await act(async () => {
        await vi.advanceTimersByTimeAsync(5_000);
        await Promise.resolve();
        await vi.advanceTimersByTimeAsync(0);
      });

      expect(mockFetchStatus).toHaveBeenCalledTimes(2);
      await act(async () => {
        await vi.waitFor(() => {
          expect(status.result.current.data?.session_health?.eligible_for_wake).toBe(true);
        });
      });
    } finally {
      vi.useRealTimers();
    }
  });

  it("Should validate, delete, rollback, and wake", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const rolledBackHeartbeat = { ...heartbeat, digest: "d".repeat(64) };
    mockDeleteHeartbeat.mockResolvedValue({
      heartbeat: { ...heartbeat, active: false },
      revision: { id: "r2" },
    });
    mockRollbackHeartbeat.mockImplementation(async () => {
      mockFetchHeartbeat.mockResolvedValue(rolledBackHeartbeat);
      return { heartbeat: rolledBackHeartbeat, revision: { id: "r3" } };
    });

    const validate = renderHook(() => useValidateAgentHeartbeat(), {
      wrapper: createWrapper(queryClient),
    });
    await act(async () => {
      await validate.result.current.mutateAsync({
        name: "coder",
        cacheWorkspace: "ws_alpha",
        profile: "default",
        params: { body: "---\nenabled: true\n---\n" },
      });
    });
    expect(mockValidateHeartbeat).toHaveBeenCalledWith(
      "coder",
      {
        body: "---\nenabled: true\n---\n",
      },
      undefined,
      "default"
    );

    const del = renderHook(() => useDeleteAgentHeartbeat(), {
      wrapper: createWrapper(queryClient),
    });
    await act(async () => {
      await del.result.current.mutateAsync({
        name: "coder",
        cacheWorkspace: "ws_alpha",
        profile: "default",
        params: { expected_digest: "b".repeat(64) },
      });
    });
    expect(mockDeleteHeartbeat).toHaveBeenCalledWith(
      "coder",
      {
        expected_digest: "b".repeat(64),
      },
      undefined,
      "default"
    );

    const rollback = renderHook(() => useRollbackAgentHeartbeat(), {
      wrapper: createWrapper(queryClient),
    });
    await act(async () => {
      await rollback.result.current.mutateAsync({
        name: "coder",
        cacheWorkspace: "ws_alpha",
        profile: "default",
        params: { expected_digest: "b".repeat(64) },
      });
    });
    expect(mockRollbackHeartbeat).toHaveBeenCalledWith(
      "coder",
      {
        expected_digest: "b".repeat(64),
      },
      undefined,
      "default"
    );
    await waitFor(() => {
      expect(queryClient.getQueryData(agentKeys.heartbeat("coder", "ws_alpha"))).toEqual(
        rolledBackHeartbeat
      );
    });

    const wake = renderHook(() => useWakeAgentHeartbeat(), {
      wrapper: createWrapper(queryClient),
    });
    await act(async () => {
      await wake.result.current.mutateAsync({
        name: "coder",
        cacheWorkspace: "ws_alpha",
        profile: "default",
        params: { session_id: "sess-1", source: "manual" },
      });
    });
    expect(mockWake).toHaveBeenCalledWith(
      "coder",
      { session_id: "sess-1", source: "manual" },
      undefined,
      "default"
    );
  });
});
