import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { taskExecutionProfileFixture } from "@/systems/tasks/mocks/fixtures";

const hooks = vi.hoisted(() => ({
  createSubscription: { isPending: false, mutateAsync: vi.fn() },
  deleteProfile: { isPending: false, mutateAsync: vi.fn() },
  deleteSubscription: { isPending: false, mutateAsync: vi.fn() },
  setProfile: { isPending: false, mutateAsync: vi.fn() },
  subscriptions: vi.fn(() => ({ data: [], error: null, isLoading: false })),
  stream: vi.fn(),
}));

vi.mock("sonner", () => ({
  toast: {
    error: vi.fn(),
    success: vi.fn(),
  },
}));

vi.mock("../use-task-notifications", () => ({
  useCreateTaskBridgeNotificationSubscription: () => hooks.createSubscription,
  useDeleteTaskBridgeNotificationSubscription: () => hooks.deleteSubscription,
  useTaskBridgeNotificationSubscriptions: hooks.subscriptions,
}));

vi.mock("../use-task-profile", () => ({
  useDeleteTaskExecutionProfile: () => hooks.deleteProfile,
  useSetTaskExecutionProfile: () => hooks.setProfile,
}));

vi.mock("../use-task-stream", () => ({
  useTaskStream: hooks.stream,
}));

import { toast } from "sonner";

import { useTaskOperatorLayer } from "../use-task-operator-layer";

beforeEach(() => {
  vi.clearAllMocks();
  hooks.createSubscription.mutateAsync.mockResolvedValue(undefined);
  hooks.deleteProfile.mutateAsync.mockResolvedValue(undefined);
  hooks.deleteSubscription.mutateAsync.mockResolvedValue(undefined);
  hooks.setProfile.mutateAsync.mockResolvedValue(undefined);
});

function latestStreamOptions() {
  return hooks.stream.mock.calls.at(-1)?.[1] as {
    afterSequence: number;
    enabled: boolean;
    onError: (error: unknown) => void;
    onEvent: () => void;
  };
}

describe("useTaskOperatorLayer", () => {
  it("Should gate subscriptions and stream activation on drawer state and a real replay seed", () => {
    const initialProps: { enabled: boolean; latestEventSeq: number | null } = {
      enabled: false,
      latestEventSeq: 12,
    };
    const { result, rerender } = renderHook(
      ({ enabled, latestEventSeq }: { enabled: boolean; latestEventSeq: number | null }) =>
        useTaskOperatorLayer("task_001", { enabled, latestEventSeq }),
      { initialProps }
    );

    expect(hooks.subscriptions).toHaveBeenLastCalledWith("task_001", {}, { enabled: false });
    expect(latestStreamOptions()).toMatchObject({ afterSequence: 12, enabled: false });
    expect(result.current.streamState).toBe("disabled");

    rerender({ enabled: true, latestEventSeq: null });
    expect(hooks.subscriptions).toHaveBeenLastCalledWith("task_001", {}, { enabled: true });
    expect(latestStreamOptions()).toMatchObject({ afterSequence: 0, enabled: false });
    expect(result.current.streamState).toBe("idle");

    rerender({ enabled: true, latestEventSeq: 14 });
    expect(latestStreamOptions()).toMatchObject({ afterSequence: 14, enabled: true });
    expect(result.current.streamSeedSequence).toBe(14);
  });

  it("Should derive receiving and error states from stream events and reset them for a new key", () => {
    const { result, rerender } = renderHook(
      ({ taskId, enabled }: { taskId: string; enabled: boolean }) =>
        useTaskOperatorLayer(taskId, { enabled, latestEventSeq: 7 }),
      { initialProps: { taskId: "task_001", enabled: true } }
    );

    act(() => latestStreamOptions().onEvent());
    expect(result.current.streamState).toBe("receiving");

    act(() => latestStreamOptions().onError(new Error("Stream unavailable")));
    expect(result.current.streamState).toBe("error");
    expect(result.current.streamErrorMessage).toBe("Stream unavailable");

    rerender({ taskId: "task_002", enabled: true });
    expect(result.current.streamState).toBe("idle");
    expect(result.current.streamErrorMessage).toBeNull();

    rerender({ taskId: "task_002", enabled: false });
    expect(result.current.streamState).toBe("disabled");
  });

  it("Should notify and reject when setting the execution profile fails", async () => {
    const runtimeError = new Error("Profile rejected");
    hooks.setProfile.mutateAsync.mockRejectedValue(runtimeError);
    const { result } = renderHook(() =>
      useTaskOperatorLayer("task_001", { enabled: true, latestEventSeq: 0 })
    );

    await act(async () => {
      await expect(result.current.handleSetProfile(taskExecutionProfileFixture)).rejects.toBe(
        runtimeError
      );
    });

    expect(hooks.setProfile.mutateAsync).toHaveBeenCalledWith({
      id: "task_001",
      data: taskExecutionProfileFixture,
    });
    expect(toast.error).toHaveBeenCalledWith("Profile rejected");
  });

  it("Should notify and reject when deleting the execution profile fails", async () => {
    const runtimeError = new Error("Profile delete rejected");
    hooks.deleteProfile.mutateAsync.mockRejectedValue(runtimeError);
    const { result } = renderHook(() =>
      useTaskOperatorLayer("task_001", { enabled: true, latestEventSeq: 0 })
    );

    await act(async () => {
      await expect(result.current.handleDeleteProfile()).rejects.toBe(runtimeError);
    });

    expect(hooks.deleteProfile.mutateAsync).toHaveBeenCalledWith({ id: "task_001" });
    expect(toast.error).toHaveBeenCalledWith("Profile delete rejected");
  });

  it("Should notify and reject when creating a bridge subscription fails", async () => {
    const runtimeError = new Error("Subscription rejected");
    hooks.createSubscription.mutateAsync.mockRejectedValue(runtimeError);
    const request = {
      bridge_instance_id: "bridge_alpha",
      delivery_mode: "direct-send" as const,
      scope: "workspace" as const,
    };
    const { result } = renderHook(() =>
      useTaskOperatorLayer("task_001", { enabled: true, latestEventSeq: 0 })
    );

    await act(async () => {
      await expect(result.current.handleCreateSubscription(request)).rejects.toBe(runtimeError);
    });

    expect(hooks.createSubscription.mutateAsync).toHaveBeenCalledWith({
      taskId: "task_001",
      data: request,
    });
    expect(toast.error).toHaveBeenCalledWith("Subscription rejected");
  });

  it("Should notify and reject when deleting a bridge subscription fails", async () => {
    const runtimeError = new Error("Subscription delete rejected");
    hooks.deleteSubscription.mutateAsync.mockRejectedValue(runtimeError);
    const { result } = renderHook(() =>
      useTaskOperatorLayer("task_001", { enabled: true, latestEventSeq: 0 })
    );

    await act(async () => {
      await expect(result.current.handleDeleteSubscription("subscription_007")).rejects.toBe(
        runtimeError
      );
    });

    expect(hooks.deleteSubscription.mutateAsync).toHaveBeenCalledWith({
      taskId: "task_001",
      subscriptionId: "subscription_007",
    });
    expect(toast.error).toHaveBeenCalledWith("Subscription delete rejected");
  });
});
