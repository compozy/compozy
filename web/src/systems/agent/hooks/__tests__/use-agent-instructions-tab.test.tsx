import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { primaryAgentFixture } from "@/systems/agent/testing";
import { primarySessionFixture } from "@/systems/session/testing";

const mocks = vi.hoisted(() => ({
  useSoul: vi.fn(),
  useSoulHistory: vi.fn(),
  putSoul: vi.fn(),
  validateSoul: vi.fn(),
  rollbackSoul: vi.fn(),
  useHeartbeat: vi.fn(),
  useHeartbeatHistory: vi.fn(),
  useHeartbeatStatus: vi.fn(),
  putHeartbeat: vi.fn(),
  validateHeartbeat: vi.fn(),
  rollbackHeartbeat: vi.fn(),
  wakeHeartbeat: vi.fn(),
  soulRefetch: vi.fn(),
  soulHistoryRefetch: vi.fn(),
  heartbeatRefetch: vi.fn(),
  heartbeatHistoryRefetch: vi.fn(),
  statusRefetch: vi.fn(),
}));

vi.mock("../use-agent-soul", () => ({
  useAgentSoul: (...args: unknown[]) => mocks.useSoul(...args),
  useAgentSoulHistory: (...args: unknown[]) => mocks.useSoulHistory(...args),
  usePutAgentSoul: () => ({ mutateAsync: mocks.putSoul }),
  useValidateAgentSoul: () => ({ mutateAsync: mocks.validateSoul }),
  useRollbackAgentSoul: () => ({ mutateAsync: mocks.rollbackSoul }),
}));

vi.mock("../use-agent-heartbeat", () => ({
  useAgentHeartbeat: (...args: unknown[]) => mocks.useHeartbeat(...args),
  useAgentHeartbeatHistory: (...args: unknown[]) => mocks.useHeartbeatHistory(...args),
  useAgentHeartbeatStatus: (...args: unknown[]) => mocks.useHeartbeatStatus(...args),
  usePutAgentHeartbeat: () => ({ mutateAsync: mocks.putHeartbeat }),
  useValidateAgentHeartbeat: () => ({ mutateAsync: mocks.validateHeartbeat }),
  useRollbackAgentHeartbeat: () => ({ mutateAsync: mocks.rollbackHeartbeat }),
  useWakeAgentHeartbeat: () => ({
    mutate: mocks.wakeHeartbeat,
    isPending: false,
    isError: false,
    error: null,
    data: undefined,
  }),
}));

import { useAgentInstructionsTab } from "../use-agent-instructions-tab";

describe("useAgentInstructionsTab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.useSoul.mockReturnValue({
      data: { present: false, validation_status: "missing" },
      isLoading: false,
      isFetched: true,
      isSuccess: true,
      isError: false,
      refetch: mocks.soulRefetch,
    });
    mocks.useSoulHistory.mockReturnValue({
      data: { revisions: [] },
      refetch: mocks.soulHistoryRefetch,
    });
    mocks.useHeartbeat.mockReturnValue({
      data: { present: false, validation_status: "missing" },
      isLoading: false,
      isFetched: true,
      isSuccess: true,
      isError: false,
      refetch: mocks.heartbeatRefetch,
    });
    mocks.useHeartbeatHistory.mockReturnValue({
      data: { revisions: [] },
      refetch: mocks.heartbeatHistoryRefetch,
    });
    mocks.useHeartbeatStatus.mockReturnValue({
      data: { enabled: true },
      isLoading: false,
      isError: false,
      refetch: mocks.statusRefetch,
    });
    mocks.putSoul.mockResolvedValue({ soul: { present: true, digest: "soul-new" } });
    mocks.validateSoul.mockResolvedValue({ diagnostics: [] });
    mocks.rollbackSoul.mockResolvedValue({ soul: { present: true, digest: "soul-restored" } });
    mocks.putHeartbeat.mockResolvedValue({
      heartbeat: { present: true, digest: "heartbeat-new" },
    });
    mocks.validateHeartbeat.mockResolvedValue({ diagnostics: [] });
    mocks.rollbackHeartbeat.mockResolvedValue({
      heartbeat: { present: true, digest: "heartbeat-restored" },
    });
  });

  it("Should fetch both authored files for truthful missing badges before either file tab is opened", () => {
    const { result } = renderHook(() =>
      useAgentInstructionsTab({
        agent: primaryAgentFixture,
        file: "agent",
        workspaceId: "ws-test",
        sessions: [],
      })
    );

    expect(mocks.useSoul).toHaveBeenCalledWith(primaryAgentFixture.name, "ws-test");
    expect(mocks.useHeartbeat).toHaveBeenCalledWith(primaryAgentFixture.name, "ws-test");
    expect(result.current.soulMissing).toBe(true);
    expect(result.current.heartbeatMissing).toBe(true);
    expect(result.current.soul.resourceKey).toBe(
      JSON.stringify(["ws-test", primaryAgentFixture.name, "soul"])
    );
    expect(result.current.heartbeat.resourceKey).toBe(
      JSON.stringify(["ws-test", primaryAgentFixture.name, "heartbeat"])
    );
  });

  it("Should not treat query errors or non-success states as missing badges", () => {
    mocks.useSoul.mockReturnValue({
      data: undefined,
      isLoading: false,
      isFetched: true,
      isSuccess: false,
      isError: true,
      refetch: mocks.soulRefetch,
    });
    mocks.useHeartbeat.mockReturnValue({
      data: undefined,
      isLoading: true,
      isFetched: false,
      isSuccess: false,
      isError: false,
      refetch: mocks.heartbeatRefetch,
    });

    const { result } = renderHook(() =>
      useAgentInstructionsTab({
        agent: primaryAgentFixture,
        file: "agent",
        workspaceId: "ws-test",
        sessions: [],
      })
    );

    expect(result.current.soulMissing).toBe(false);
    expect(result.current.heartbeatMissing).toBe(false);
  });

  it("Should round-trip workspace and CAS inputs for validate, save, restore, retry, and wake", async () => {
    const session = { ...primarySessionFixture, id: "sess-active", state: "active" as const };
    const { result } = renderHook(() =>
      useAgentInstructionsTab({
        agent: primaryAgentFixture,
        file: "heartbeat",
        workspaceId: "ws-test",
        sessions: [session],
      })
    );

    await act(async () => {
      await result.current.soul.onValidate("soul-body");
      await result.current.soul.onSave("soul-body", "soul-digest");
      await result.current.soul.onRestore("soul-rev", "soul-digest");
      await result.current.heartbeat.onValidate("heartbeat-body");
      await result.current.heartbeat.onSave("heartbeat-body", "heartbeat-digest");
      await result.current.heartbeat.onRestore("heartbeat-rev", "heartbeat-digest");
      result.current.soul.onRetry();
      result.current.heartbeat.onRetry();
      result.current.heartbeat.onWake("sess-active");
    });

    expect(mocks.validateSoul).toHaveBeenCalledWith({ body: "soul-body", workspace_id: "ws-test" });
    expect(mocks.putSoul).toHaveBeenCalledWith({
      body: "soul-body",
      expected_digest: "soul-digest",
      workspace_id: "ws-test",
    });
    expect(mocks.rollbackSoul).toHaveBeenCalledWith({
      revision_id: "soul-rev",
      expected_digest: "soul-digest",
      workspace_id: "ws-test",
    });
    expect(mocks.validateHeartbeat).toHaveBeenCalledWith({
      body: "heartbeat-body",
      workspace_id: "ws-test",
    });
    expect(mocks.putHeartbeat).toHaveBeenCalledWith({
      body: "heartbeat-body",
      expected_digest: "heartbeat-digest",
      workspace_id: "ws-test",
    });
    expect(mocks.rollbackHeartbeat).toHaveBeenCalledWith({
      revision_id: "heartbeat-rev",
      expected_digest: "heartbeat-digest",
      workspace_id: "ws-test",
    });
    expect(mocks.wakeHeartbeat).toHaveBeenCalledWith({
      session_id: "sess-active",
      source: "manual",
    });
    expect(mocks.soulRefetch).toHaveBeenCalled();
    expect(mocks.soulHistoryRefetch).toHaveBeenCalled();
    expect(mocks.heartbeatRefetch).toHaveBeenCalled();
    expect(mocks.heartbeatHistoryRefetch).toHaveBeenCalled();
    expect(mocks.statusRefetch).toHaveBeenCalled();
  });

  it("Should derive the sole active wake target and discard a selection that leaves the active set", () => {
    const first = { ...primarySessionFixture, id: "sess-first", state: "active" as const };
    const second = { ...primarySessionFixture, id: "sess-second", state: "active" as const };
    const third = { ...primarySessionFixture, id: "sess-third", state: "active" as const };
    const { result, rerender } = renderHook(
      ({ sessions }) =>
        useAgentInstructionsTab({
          agent: primaryAgentFixture,
          file: "heartbeat",
          workspaceId: "ws-test",
          sessions,
        }),
      { initialProps: { sessions: [first] } }
    );

    expect(result.current.heartbeat.wakeSessionId).toBe("sess-first");

    act(() => result.current.heartbeat.setWakeSessionId("sess-first"));
    rerender({ sessions: [second, third] });

    expect(result.current.heartbeat.wakeSessionId).toBeNull();

    rerender({ sessions: [first, second] });
    expect(result.current.heartbeat.wakeSessionId).toBeNull();
  });
});
