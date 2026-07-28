import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

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

    const put = renderHook(() => usePutAgentHeartbeat("coder", "ws_alpha"), {
      wrapper: createWrapper(queryClient),
    });
    await act(async () => {
      await put.result.current.mutateAsync({
        body: "---\nenabled: true\n---\n",
        expected_digest: "b".repeat(64),
      });
    });
    expect(mockPutHeartbeat).toHaveBeenCalledWith("coder", {
      body: "---\nenabled: true\n---\n",
      expected_digest: "b".repeat(64),
    });
    await waitFor(() => {
      expect(queryClient.getQueryData(agentKeys.heartbeat("coder", "ws_alpha"))).toEqual(
        putHeartbeat
      );
    });
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

    const validate = renderHook(() => useValidateAgentHeartbeat("coder"), {
      wrapper: createWrapper(queryClient),
    });
    await act(async () => {
      await validate.result.current.mutateAsync({ body: "---\nenabled: true\n---\n" });
    });
    expect(mockValidateHeartbeat).toHaveBeenCalledWith("coder", {
      body: "---\nenabled: true\n---\n",
    });

    const del = renderHook(() => useDeleteAgentHeartbeat("coder", "ws_alpha"), {
      wrapper: createWrapper(queryClient),
    });
    await act(async () => {
      await del.result.current.mutateAsync({ expected_digest: "b".repeat(64) });
    });
    expect(mockDeleteHeartbeat).toHaveBeenCalledWith("coder", {
      expected_digest: "b".repeat(64),
    });

    const rollback = renderHook(() => useRollbackAgentHeartbeat("coder", "ws_alpha"), {
      wrapper: createWrapper(queryClient),
    });
    await act(async () => {
      await rollback.result.current.mutateAsync({ expected_digest: "b".repeat(64) });
    });
    expect(mockRollbackHeartbeat).toHaveBeenCalledWith("coder", {
      expected_digest: "b".repeat(64),
    });
    await waitFor(() => {
      expect(queryClient.getQueryData(agentKeys.heartbeat("coder", "ws_alpha"))).toEqual(
        rolledBackHeartbeat
      );
    });

    const wake = renderHook(() => useWakeAgentHeartbeat("coder", "ws_alpha"), {
      wrapper: createWrapper(queryClient),
    });
    await act(async () => {
      await wake.result.current.mutateAsync({ session_id: "sess-1", source: "manual" });
    });
    expect(mockWake).toHaveBeenCalledWith("coder", { session_id: "sess-1", source: "manual" });
  });
});
