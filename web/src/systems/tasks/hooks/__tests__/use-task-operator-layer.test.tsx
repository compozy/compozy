import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { taskExecutionProfileFixture } from "@/systems/tasks/mocks/fixtures";

const hooks = vi.hoisted(() => ({
  deleteProfile: { isPending: false, mutateAsync: vi.fn() },
  setProfile: { isPending: false, mutateAsync: vi.fn() },
}));

vi.mock("sonner", () => ({
  toast: {
    error: vi.fn(),
    success: vi.fn(),
  },
}));

vi.mock("../use-task-profile", () => ({
  useDeleteTaskExecutionProfile: () => hooks.deleteProfile,
  useSetTaskExecutionProfile: () => hooks.setProfile,
}));

import { toast } from "sonner";

import { useTaskOperatorLayer } from "../use-task-operator-layer";

beforeEach(() => {
  vi.clearAllMocks();
  hooks.deleteProfile.mutateAsync.mockResolvedValue(undefined);
  hooks.setProfile.mutateAsync.mockResolvedValue(undefined);
});

describe("useTaskOperatorLayer", () => {
  it("Should gate and expose the stream state owned by the task page", () => {
    const initialProps = {
      enabled: false,
      streamErrorMessage: null as string | null,
      streamSeedSequence: 12,
      streamState: "receiving" as const,
    };
    const { result, rerender } = renderHook(
      (props: typeof initialProps) => useTaskOperatorLayer("task_001", props),
      { initialProps }
    );
    expect(result.current.streamState).toBe("disabled");
    expect(result.current.streamSeedSequence).toBe(12);

    rerender({ ...initialProps, enabled: true });
    expect(result.current.streamState).toBe("receiving");
  });

  it("Should expose a supplied stream error only while the drawer is enabled", () => {
    const { result, rerender } = renderHook(
      ({ enabled }: { enabled: boolean }) =>
        useTaskOperatorLayer("task_001", {
          enabled,
          streamErrorMessage: "Stream unavailable",
          streamSeedSequence: 7,
          streamState: "error",
        }),
      { initialProps: { enabled: true } }
    );

    expect(result.current.streamState).toBe("error");
    expect(result.current.streamErrorMessage).toBe("Stream unavailable");

    rerender({ enabled: false });
    expect(result.current.streamState).toBe("disabled");
    expect(result.current.streamErrorMessage).toBeNull();
  });

  it("Should notify and reject when setting the execution profile fails", async () => {
    const runtimeError = new Error("Profile rejected");
    hooks.setProfile.mutateAsync.mockRejectedValue(runtimeError);
    const { result } = renderHook(() => useTaskOperatorLayer("task_001", { enabled: true }));

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
    const { result } = renderHook(() => useTaskOperatorLayer("task_001", { enabled: true }));

    await act(async () => {
      await expect(result.current.handleDeleteProfile()).rejects.toBe(runtimeError);
    });

    expect(hooks.deleteProfile.mutateAsync).toHaveBeenCalledWith({ id: "task_001" });
    expect(toast.error).toHaveBeenCalledWith("Profile delete rejected");
  });
});
